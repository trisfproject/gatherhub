package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/trisfproject/gatherhub/internal/models"
	"github.com/trisfproject/gatherhub/internal/services"
	templ "github.com/trisfproject/gatherhub/internal/templates"
)

// ─────────────────────── Data Structs ───────────────────────

// LandingData is the template data for the landing page
type LandingData struct {
	Event *models.Event
}

// EventPublicData is the template data for the public event detail page by slug
type EventPublicData struct {
	Event *models.Event
}

// RegisterData is the template data for the registration form page
type RegisterData struct {
	Event  *models.Event
	Errors []string
	Form   map[string]string // preserved form values on validation failure
}

// SuccessData is the template data for the success page
type SuccessData struct {
	Participant *models.Participant
	Event       *models.Event
	WALink      string // formatted https://wa.me/{number} link
}

// ─────────────────────── Handler ───────────────────────

// PageHandler renders server-side HTML pages for the public registration flow
type PageHandler struct {
	eventService        *services.EventService
	participantService  *services.ParticipantService
	notificationService *services.NotificationService
	checkinService      *services.CheckinService
	settingsService     *services.SettingsService
	tmpl                *template.Template
}

// NewPageHandler creates and initialises a PageHandler, parsing all embedded templates.
func NewPageHandler(
	eventService *services.EventService,
	participantService *services.ParticipantService,
	notificationService *services.NotificationService,
	checkinService *services.CheckinService,
	settingsService *services.SettingsService,
) (*PageHandler, error) {
	funcMap := buildFuncMap()
	funcMap["setting"] = func(key string) string {
		return settingsService.Get(key)
	}
	funcMap["settingBool"] = func(key string) bool {
		return settingsService.GetBool(key)
	}

	t, err := template.New("").Funcs(funcMap).ParseFS(templ.Files, "landing.html", "register.html", "success.html", "event_public.html", "checkin.html", "maintenance.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	return &PageHandler{
		eventService:        eventService,
		participantService:  participantService,
		notificationService: notificationService,
		checkinService:      checkinService,
		settingsService:     settingsService,
		tmpl:                t,
	}, nil
}

// ─────────────────────── Route Handlers ───────────────────────

// Landing handles GET / — event landing page
func (h *PageHandler) Landing(c *fiber.Ctx) error {
	event, err := h.eventService.GetFirst()
	if err != nil {
		return h.renderError(c, "Tidak ada acara yang tersedia saat ini.", "/")
	}
	return h.render(c, "landing.html", LandingData{Event: event})
}

// RegisterPage handles GET /register — registration form
func (h *PageHandler) RegisterPage(c *fiber.Ctx) error {
	// Guard: check if registration is globally enabled in settings
	if !h.settingsService.GetBool("registration_enabled") {
		return h.renderError(c, "Pendaftaran peserta sedang ditutup sementara oleh administrator.", "/")
	}

	event, err := h.eventService.GetFirst()
	if err != nil {
		return h.renderError(c, "Tidak ada acara yang membuka pendaftaran saat ini.", "/")
	}
	// Guard: only PUBLISHED events with an open registration window accept new registrations
	if guardMsg := registrationGuard(event); guardMsg != "" {
		return h.renderError(c, guardMsg, "/")
	}
	return h.render(c, "register.html", RegisterData{
		Event: event,
		Form:  map[string]string{},
	})
}

// EventBySlug handles GET /event/:slug — public event detail page
func (h *PageHandler) EventBySlug(c *fiber.Ctx) error {
	slug := c.Params("slug")
	event, err := h.eventService.GetBySlug(slug)
	if err != nil {
		return h.renderError(c, "Acara tidak ditemukan.", "/")
	}
	return h.render(c, "event_public.html", EventPublicData{Event: event})
}

// RegisterSubmit handles POST /register — process form submission
func (h *PageHandler) RegisterSubmit(c *fiber.Ctx) error {
	// Guard: check if registration is globally enabled in settings
	if !h.settingsService.GetBool("registration_enabled") {
		return h.renderError(c, "Pendaftaran peserta sedang ditutup sementara oleh administrator.", "/")
	}

	event, err := h.eventService.GetFirst()
	if err != nil {
		return c.Redirect("/", fiber.StatusSeeOther)
	}
	// Guard: reject submissions for non-PUBLISHED / closed-window events
	if guardMsg := registrationGuard(event); guardMsg != "" {
		return h.renderError(c, guardMsg, "/")
	}

	// Parse form fields
	form := &services.RegisterForm{
		FullName:         c.FormValue("full_name"),
		Phone:            c.FormValue("phone"),
		Email:            c.FormValue("email"),
		City:             c.FormValue("city"),
		CompanyName:      c.FormValue("company_name"),
		IndustrialEstate: c.FormValue("industrial_estate"),
		TelegramUsername: c.FormValue("telegram_username"),
		JobTitle:         c.FormValue("job_title"),
	}

	// Collect validation errors
	var allErrors []string
	allErrors = append(allErrors, form.Validate()...)

	// Handle payment proof upload
	file, fileErr := c.FormFile("payment_proof")
	if fileErr != nil {
		allErrors = append(allErrors, "Bukti pembayaran wajib diunggah")
	} else if err := services.ValidateFile(file); err != nil {
		allErrors = append(allErrors, err.Error())
	}

	// If validation errors exist, respond appropriately
	if len(allErrors) > 0 {
		return h.handleValidationError(c, event, form, allErrors)
	}

	// Save payment proof file
	paymentFilename, err := h.participantService.SavePaymentProof(file)
	if err != nil {
		return h.handleValidationError(c, event, form, []string{"Gagal mengunggah file: " + err.Error()})
	}

	// Create participant record
	participant, err := h.participantService.Create(event.ID, form, paymentFilename)
	if err != nil {
		return h.handleValidationError(c, event, form, []string{"Gagal menyimpan data: " + err.Error()})
	}

	// Trigger registration submitted notification
	participant.Event = *event
	if err := h.notificationService.SendNotification(participant, "SUBMITTED"); err != nil {
		log.Printf("Warning: failed to send registration submitted notification: %v", err)
	}

	// Success — redirect to success page
	successURL := fmt.Sprintf("/register/success?pid=%d", participant.ID)

	// HTMX request: use HX-Redirect header
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", successURL)
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect(successURL, fiber.StatusSeeOther)
}

// Success handles GET /register/success
func (h *PageHandler) Success(c *fiber.Ctx) error {
	pidStr := c.Query("pid")
	pid, err := strconv.ParseUint(pidStr, 10, 64)
	if err != nil || pid == 0 {
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	participant, err := h.participantService.GetByID(uint(pid))
	if err != nil {
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	event, err := h.eventService.GetFirst()
	if err != nil {
		return c.Redirect("/", fiber.StatusSeeOther)
	}

	waLink := "https://wa.me/" + event.AdminWhatsapp

	return h.render(c, "success.html", SuccessData{
		Participant: participant,
		Event:       event,
		WALink:      waLink,
	})
}

// CheckinPage handles GET /checkin
func (h *PageHandler) CheckinPage(c *fiber.Ctx) error {
	event, err := h.eventService.GetFirst()
	if err != nil {
		event = &models.Event{Title: "GatherHub Event"}
	}
	return h.render(c, "checkin.html", fiber.Map{
		"Event": event,
	})
}

// CheckinSubmit handles POST /checkin
func (h *PageHandler) CheckinSubmit(c *fiber.Ctx) error {
	token := strings.TrimSpace(c.FormValue("token"))
	if token == "" {
		var buf bytes.Buffer
		_ = h.tmpl.ExecuteTemplate(&buf, "checkin_invalid", fiber.Map{
			"Message": "Token QR kosong atau tidak ditemukan.",
		})
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Status(fiber.StatusOK).Send(buf.Bytes())
	}

	// 1. Verify token signature and authenticity
	payload, err := h.checkinService.VerifyQRToken(token)
	if err != nil {
		var buf bytes.Buffer
		_ = h.tmpl.ExecuteTemplate(&buf, "checkin_invalid", fiber.Map{
			"Message": "QR Code tidak valid atau tanda tangan digital salah.",
		})
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Status(fiber.StatusOK).Send(buf.Bytes())
	}

	// 2. Attempt checking in
	att, err := h.checkinService.Checkin(payload.ParticipantID, payload.EventID, "QR_CODE")
	if err != nil {
		var buf bytes.Buffer
		if strings.Contains(err.Error(), "sudah melakukan check-in") || strings.Contains(err.Error(), "already checked in") {
			// Fetch the existing attendance record to display who and when they checked in
			existingAtt, getErr := h.checkinService.GetAttendance(payload.ParticipantID, payload.EventID)
			if getErr == nil {
				_ = h.tmpl.ExecuteTemplate(&buf, "checkin_duplicate", existingAtt)
			} else {
				// Fallback if we couldn't load detail
				_ = h.tmpl.ExecuteTemplate(&buf, "checkin_invalid", fiber.Map{
					"Message": "Peserta sudah terdaftar check-in, tetapi detail tidak ditemukan.",
				})
			}
			c.Set("Content-Type", "text/html; charset=utf-8")
			return c.Status(fiber.StatusOK).Send(buf.Bytes())
		}

		// Handle other validation messages (e.g. participant not verified, participant not found)
		_ = h.tmpl.ExecuteTemplate(&buf, "checkin_invalid", fiber.Map{
			"Message": err.Error(),
		})
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Status(fiber.StatusOK).Send(buf.Bytes())
	}

	// 3. Success
	var buf bytes.Buffer
	_ = h.tmpl.ExecuteTemplate(&buf, "checkin_success", att)
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(fiber.StatusOK).Send(buf.Bytes())
}

// ─────────────────────── Helpers ───────────────────────

// handleValidationError sends errors back — HTML fragment for HTMX, full page re-render otherwise
func (h *PageHandler) handleValidationError(
	c *fiber.Ctx,
	event *models.Event,
	form *services.RegisterForm,
	errs []string,
) error {
	// HTMX request: return HTML error fragment
	if c.Get("HX-Request") == "true" {
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.Status(fiber.StatusUnprocessableEntity).SendString(buildErrorFragment(errs))
	}

	// Standard request: re-render the full register page with preserved form values
	return h.render(c, "register.html", RegisterData{
		Event:  event,
		Errors: errs,
		Form:   formToMap(form),
	})
}

// render executes a named template and sends the result as HTML
func (h *PageHandler) render(c *fiber.Ctx, name string, data any) error {
	var buf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("template render error (%s): %w", name, err)
	}
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(fiber.StatusOK).Send(buf.Bytes())
}

// renderError shows a simple error page
func (h *PageHandler) renderError(c *fiber.Ctx, message, backURL string) error {
	html := fmt.Sprintf(`<!DOCTYPE html><html lang="id"><head><meta charset="UTF-8"/><title>Error — GatherHub</title>
<script src="https://cdn.tailwindcss.com"></script></head>
<body class="bg-[#07071a] text-white min-h-screen flex items-center justify-center">
<div class="text-center px-6">
  <div class="text-6xl mb-6">⚠️</div>
  <h1 class="text-2xl font-bold mb-3">Oops!</h1>
  <p class="text-white/60 mb-8">%s</p>
  <a href="%s" class="inline-flex items-center gap-2 bg-indigo-600 hover:bg-indigo-500 font-semibold px-6 py-3 rounded-xl text-sm transition-colors">
    Kembali
  </a>
</div></body></html>`, message, backURL)
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(fiber.StatusNotFound).SendString(html)
}

// buildErrorFragment returns an HTML fragment for HTMX error injection
func buildErrorFragment(errs []string) string {
	var items strings.Builder
	for _, e := range errs {
		items.WriteString(fmt.Sprintf(
			`<li class="flex items-center gap-1.5 text-red-400/80 text-sm"><span class="w-1.5 h-1.5 bg-red-400/60 rounded-full flex-shrink-0"></span>%s</li>`,
			template.HTMLEscapeString(e),
		))
	}
	return fmt.Sprintf(`
<div class="alert-error" style="background:rgba(239,68,68,.1);border:1.5px solid rgba(239,68,68,.3);border-radius:14px;padding:16px 20px;">
  <div style="display:flex;align-items:flex-start;gap:12px;">
    <svg style="width:20px;height:20px;color:#f87171;flex-shrink:0;margin-top:2px" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
    </svg>
    <div>
      <p style="color:#fca5a5;font-weight:700;font-size:.875rem;margin-bottom:8px;">Mohon perbaiki kesalahan berikut:</p>
      <ul style="list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:4px;">%s</ul>
    </div>
  </div>
</div>`, items.String())
}

func formToMap(form *services.RegisterForm) map[string]string {
	return map[string]string{
		"full_name":         form.FullName,
		"phone":             form.Phone,
		"email":             form.Email,
		"city":              form.City,
		"company_name":      form.CompanyName,
		"industrial_estate": form.IndustrialEstate,
		"telegram_username": form.TelegramUsername,
		"job_title":         form.JobTitle,
	}
}

// registrationGuard returns a human-readable error message if the event does not
// currently accept registrations, or an empty string if registration is allowed.
func registrationGuard(event *models.Event) string {
	if event.Status != "PUBLISHED" {
		return "Pendaftaran untuk acara ini tidak tersedia saat ini."
	}
	now := time.Now()
	if !event.RegistrationOpen.IsZero() && now.Before(event.RegistrationOpen) {
		return "Pendaftaran belum dibuka. Silakan coba lagi nanti."
	}
	if !event.RegistrationClose.IsZero() && now.After(event.RegistrationClose) {
		return "Pendaftaran sudah ditutup."
	}
	return ""
}

// ─────────────────────── Template Functions ───────────────────────

var idMonths = map[time.Month]string{
	time.January: "Januari", time.February: "Februari", time.March: "Maret",
	time.April: "April", time.May: "Mei", time.June: "Juni",
	time.July: "Juli", time.August: "Agustus", time.September: "September",
	time.October: "Oktober", time.November: "November", time.December: "Desember",
}

var idDays = map[time.Weekday]string{
	time.Sunday: "Minggu", time.Monday: "Senin", time.Tuesday: "Selasa",
	time.Wednesday: "Rabu", time.Thursday: "Kamis", time.Friday: "Jumat",
	time.Saturday: "Sabtu",
}

func buildFuncMap() template.FuncMap {
	return template.FuncMap{
		// formatDateLong → "Jumat, 15 Agustus 2025"
		"formatDateLong": func(t time.Time) string {
			return fmt.Sprintf("%s, %d %s %d", idDays[t.Weekday()], t.Day(), idMonths[t.Month()], t.Year())
		},
		// formatTime → "09:00 WIB"
		"formatTime": func(t time.Time) string {
			return t.Format("15:04") + " WIB"
		},
		// formatPrice → "Rp 350.000" or "GRATIS"
		"formatPrice": func(price float64) string {
			if price == 0 {
				return "GRATIS"
			}
			s := fmt.Sprintf("%.0f", price)
			// Add thousand separators (dots)
			result := ""
			for i, ch := range s {
				if i > 0 && (len(s)-i)%3 == 0 {
					result += "."
				}
				result += string(ch)
			}
			return "Rp " + result
		},
		// shortLocation truncates location to first segment (before comma)
		"shortLocation": func(loc string) string {
			parts := strings.SplitN(loc, ",", 2)
			return strings.TrimSpace(parts[0])
		},
		// truncate string to max length
		"truncate": func(n int, s string) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
		// now returns current time
		"now": time.Now,
		// formatYear → "2025"
		"formatYear": func(t time.Time) string {
			return fmt.Sprintf("%d", t.Year())
		},
	}
}

// RenderMaintenance renders the maintenance.html template.
func (h *PageHandler) RenderMaintenance(c *fiber.Ctx) error {
	c.Status(fiber.StatusServiceUnavailable) // HTTP 503
	return h.render(c, "maintenance.html", nil)
}

