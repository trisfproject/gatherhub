package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"
	"time"

	"github.com/gatherhub/backend/internal/models"
	"github.com/gatherhub/backend/internal/services"
	templ "github.com/gatherhub/backend/internal/templates"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

// ─────────────────────── Data structs ───────────────────────

type AdminLoginData struct {
	Error string
}

type AdminDashboardData struct {
	AdminUser    string
	Stats        *services.ParticipantStats
	RecentPending []models.Participant
	Event        *models.Event
}

type AdminParticipantsData struct {
	AdminUser    string
	Participants []models.Participant
	Stats        *services.ParticipantStats
	Filter       string
	Search       string
	Event        *models.Event
}

type AdminParticipantDetailData struct {
	AdminUser   string
	Participant *models.Participant
	Event       *models.Event
	PaymentURL  string
}

// ─────────────────────── Handler ───────────────────────

// AdminHandler handles all admin panel routes
type AdminHandler struct {
	participantService *services.ParticipantService
	eventService       *services.EventService
	store              *session.Store
	adminUsername      string
	adminPassword      string
	paymentDir         string
	tmpl               *template.Template
}

// NewAdminHandler creates and initialises an AdminHandler
func NewAdminHandler(
	participantService *services.ParticipantService,
	eventService *services.EventService,
	store *session.Store,
	adminUsername, adminPassword, paymentDir string,
) (*AdminHandler, error) {
	funcMap := buildAdminFuncMap()
	t, err := template.New("").Funcs(funcMap).ParseFS(templ.Files, "admin_login.html", "admin_dashboard.html", "admin_participants.html", "admin_participant.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse admin templates: %w", err)
	}
	return &AdminHandler{
		participantService: participantService,
		eventService:       eventService,
		store:              store,
		adminUsername:      adminUsername,
		adminPassword:      adminPassword,
		paymentDir:         paymentDir,
		tmpl:               t,
	}, nil
}

// ─────────────────────── Routes ───────────────────────

// LoginPage handles GET /admin/login
func (h *AdminHandler) LoginPage(c *fiber.Ctx) error {
	// If already logged in, redirect to dashboard
	sess, err := h.store.Get(c)
	if err == nil {
		if ok, _ := sess.Get("admin_authenticated").(bool); ok {
			return c.Redirect("/admin/dashboard", fiber.StatusSeeOther)
		}
	}
	return h.render(c, "admin_login.html", AdminLoginData{})
}

// LoginSubmit handles POST /admin/login
func (h *AdminHandler) LoginSubmit(c *fiber.Ctx) error {
	username := strings.TrimSpace(c.FormValue("username"))
	password := c.FormValue("password")

	if username != h.adminUsername || password != h.adminPassword {
		return h.render(c, "admin_login.html", AdminLoginData{
			Error: "Username atau password salah. Coba lagi.",
		})
	}

	// Create authenticated session
	sess, err := h.store.Get(c)
	if err != nil {
		return h.render(c, "admin_login.html", AdminLoginData{Error: "Gagal membuat sesi. Coba lagi."})
	}
	sess.Set("admin_authenticated", true)
	sess.Set("admin_username", username)
	if err := sess.Save(); err != nil {
		return h.render(c, "admin_login.html", AdminLoginData{Error: "Gagal menyimpan sesi. Coba lagi."})
	}

	return c.Redirect("/admin/dashboard", fiber.StatusSeeOther)
}

// Logout handles GET /admin/logout
func (h *AdminHandler) Logout(c *fiber.Ctx) error {
	sess, err := h.store.Get(c)
	if err == nil {
		_ = sess.Destroy()
	}
	return c.Redirect("/admin/login", fiber.StatusSeeOther)
}

// Dashboard handles GET /admin/dashboard
func (h *AdminHandler) Dashboard(c *fiber.Ctx) error {
	stats, err := h.participantService.GetStats()
	if err != nil {
		stats = &services.ParticipantStats{}
	}

	// Most recent pending participants (limit 5)
	pending, _ := h.participantService.GetAllForAdmin("PENDING", "")
	if len(pending) > 5 {
		pending = pending[:5]
	}

	event, _ := h.eventService.GetFirst()
	adminUser, _ := c.Locals("admin_username").(string)

	return h.render(c, "admin_dashboard.html", AdminDashboardData{
		AdminUser:     adminUser,
		Stats:         stats,
		RecentPending: pending,
		Event:         event,
	})
}

// ParticipantList handles GET /admin/participants
func (h *AdminHandler) ParticipantList(c *fiber.Ctx) error {
	filter := c.Query("status")
	search := strings.TrimSpace(c.Query("q"))

	participants, err := h.participantService.GetAllForAdmin(filter, search)
	if err != nil {
		participants = []models.Participant{}
	}

	stats, _ := h.participantService.GetStats()
	event, _ := h.eventService.GetFirst()
	adminUser, _ := c.Locals("admin_username").(string)

	return h.render(c, "admin_participants.html", AdminParticipantsData{
		AdminUser:    adminUser,
		Participants: participants,
		Stats:        stats,
		Filter:       filter,
		Search:       search,
		Event:        event,
	})
}

// ParticipantDetail handles GET /admin/participants/:id
func (h *AdminHandler) ParticipantDetail(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Redirect("/admin/participants", fiber.StatusSeeOther)
	}

	participant, err := h.participantService.GetByID(uint(id))
	if err != nil {
		return c.Redirect("/admin/participants", fiber.StatusSeeOther)
	}

	event, _ := h.eventService.GetFirst()
	adminUser, _ := c.Locals("admin_username").(string)

	paymentURL := ""
	if participant.PaymentProof != "" {
		paymentURL = "/payments/" + participant.PaymentProof
	}

	return h.render(c, "admin_participant.html", AdminParticipantDetailData{
		AdminUser:   adminUser,
		Participant: participant,
		Event:       event,
		PaymentURL:  paymentURL,
	})
}

// UpdateStatus handles POST /admin/participants/:id/status
func (h *AdminHandler) UpdateStatus(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	rawStatus := strings.ToUpper(strings.TrimSpace(c.FormValue("status")))
	status := models.ParticipantStatus(rawStatus)

	_, err = h.participantService.UpdateStatus(uint(id), status)
	if err != nil {
		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusBadRequest).SendString(
				`<div class="text-red-400 text-sm font-semibold">Gagal memperbarui status.</div>`)
		}
		return c.Redirect(fmt.Sprintf("/admin/participants/%d", id), fiber.StatusSeeOther)
	}

	// HTMX: return updated status badge + action buttons fragment
	if c.Get("HX-Request") == "true" {
		newStatus := models.ParticipantStatus(rawStatus)
		c.Set("HX-Trigger", "statusUpdated")
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(buildStatusFragment(uint(id), newStatus))
	}

	return c.Redirect(fmt.Sprintf("/admin/participants/%d", id), fiber.StatusSeeOther)
}

// ─────────────────────── Helpers ───────────────────────

func (h *AdminHandler) render(c *fiber.Ctx, name string, data any) error {
	var buf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("admin template render error (%s): %w", name, err)
	}
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Status(fiber.StatusOK).Send(buf.Bytes())
}

// buildStatusFragment returns an HTML snippet for HTMX swap after status update
func buildStatusFragment(id uint, status models.ParticipantStatus) string {
	badge := statusBadgeHTML(status)
	actions := ""
	switch status {
	case models.StatusPending:
		actions = fmt.Sprintf(`
<button hx-post="/admin/participants/%d/status" hx-vals='{"status":"VERIFIED"}' hx-target="#status-section" hx-swap="innerHTML" hx-confirm="Verifikasi pendaftaran ini?"
  class="flex-1 flex items-center justify-center gap-2 bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-3 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7"/></svg>
  Verifikasi
</button>
<button hx-post="/admin/participants/%d/status" hx-vals='{"status":"REJECTED"}' hx-target="#status-section" hx-swap="innerHTML" hx-confirm="Tolak pendaftaran ini?"
  class="flex-1 flex items-center justify-center gap-2 bg-red-700 hover:bg-red-600 text-white font-bold py-3 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
  Tolak
</button>`, id, id)
	case models.StatusVerified:
		actions = fmt.Sprintf(`
<button hx-post="/admin/participants/%d/status" hx-vals='{"status":"REJECTED"}' hx-target="#status-section" hx-swap="innerHTML" hx-confirm="Ubah ke Ditolak?"
  class="flex-1 flex items-center justify-center gap-2 bg-red-700 hover:bg-red-600 text-white font-bold py-3 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
  Ubah ke Ditolak
</button>
<button hx-post="/admin/participants/%d/status" hx-vals='{"status":"PENDING"}' hx-target="#status-section" hx-swap="innerHTML" hx-confirm="Kembalikan ke Pending?"
  class="flex-1 flex items-center justify-center gap-2 bg-amber-600 hover:bg-amber-500 text-white font-bold py-3 px-5 rounded-xl text-sm transition-colors">
  Kembalikan ke Pending
</button>`, id, id)
	case models.StatusRejected:
		actions = fmt.Sprintf(`
<button hx-post="/admin/participants/%d/status" hx-vals='{"status":"VERIFIED"}' hx-target="#status-section" hx-swap="innerHTML" hx-confirm="Verifikasi pendaftaran ini?"
  class="flex-1 flex items-center justify-center gap-2 bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-3 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7"/></svg>
  Verifikasi
</button>
<button hx-post="/admin/participants/%d/status" hx-vals='{"status":"PENDING"}' hx-target="#status-section" hx-swap="innerHTML" hx-confirm="Kembalikan ke Pending?"
  class="flex-1 flex items-center justify-center gap-2 bg-amber-600 hover:bg-amber-500 text-white font-bold py-3 px-5 rounded-xl text-sm transition-colors">
  Kembalikan ke Pending
</button>`, id, id)
	}

	return fmt.Sprintf(`
<div class="flex items-center gap-3 mb-4">%s</div>
<div class="flex gap-3">%s</div>`, badge, actions)
}

func statusBadgeHTML(status models.ParticipantStatus) string {
	switch status {
	case models.StatusVerified:
		return `<span class="inline-flex items-center gap-2 bg-emerald-500/20 border border-emerald-500/40 text-emerald-300 rounded-full px-4 py-1.5 text-sm font-bold">
<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7"/></svg>
TERVERIFIKASI</span>`
	case models.StatusRejected:
		return `<span class="inline-flex items-center gap-2 bg-red-500/20 border border-red-500/40 text-red-300 rounded-full px-4 py-1.5 text-sm font-bold">
<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
DITOLAK</span>`
	default:
		return `<span class="inline-flex items-center gap-2 bg-amber-500/20 border border-amber-500/40 text-amber-300 rounded-full px-4 py-1.5 text-sm font-bold">
<span class="w-2 h-2 bg-amber-400 rounded-full" style="animation:pulse 2s infinite"></span>
MENUNGGU</span>`
	}
}

// ─────────────────────── Template Functions ───────────────────────

var adminIDMonths = map[time.Month]string{
	time.January: "Jan", time.February: "Feb", time.March: "Mar",
	time.April: "Apr", time.May: "Mei", time.June: "Jun",
	time.July: "Jul", time.August: "Agu", time.September: "Sep",
	time.October: "Okt", time.November: "Nov", time.December: "Des",
}

func buildAdminFuncMap() template.FuncMap {
	return template.FuncMap{
		"formatDateShort": func(t time.Time) string {
			return fmt.Sprintf("%d %s %d", t.Day(), adminIDMonths[t.Month()], t.Year())
		},
		"formatDateLong": func(t time.Time) string {
			days := map[time.Weekday]string{
				time.Sunday: "Minggu", time.Monday: "Senin", time.Tuesday: "Selasa",
				time.Wednesday: "Rabu", time.Thursday: "Kamis", time.Friday: "Jumat",
				time.Saturday: "Sabtu",
			}
			months := map[time.Month]string{
				time.January: "Januari", time.February: "Februari", time.March: "Maret",
				time.April: "April", time.May: "Mei", time.June: "Juni",
				time.July: "Juli", time.August: "Agustus", time.September: "September",
				time.October: "Oktober", time.November: "November", time.December: "Desember",
			}
			return fmt.Sprintf("%s, %d %s %d", days[t.Weekday()], t.Day(), months[t.Month()], t.Year())
		},
		"formatTime": func(t time.Time) string {
			return t.Format("15:04")
		},
		"formatDateTime": func(t time.Time) string {
			return t.Format("02/01/2006 15:04")
		},
		"formatPrice": func(price float64) string {
			if price == 0 {
				return "GRATIS"
			}
			s := fmt.Sprintf("%.0f", price)
			result := ""
			for i, ch := range s {
				if i > 0 && (len(s)-i)%3 == 0 {
					result += "."
				}
				result += string(ch)
			}
			return "Rp " + result
		},
		"now":        time.Now,
		"formatYear": func(t time.Time) string { return fmt.Sprintf("%d", t.Year()) },
		"statusBadge": func(status models.ParticipantStatus) template.HTML {
			return template.HTML(statusBadgeHTML(status))
		},
		"statusClass": func(status models.ParticipantStatus) string {
			switch status {
			case models.StatusVerified:
				return "bg-emerald-500/15 text-emerald-300 border-emerald-500/30"
			case models.StatusRejected:
				return "bg-red-500/15 text-red-300 border-red-500/30"
			default:
				return "bg-amber-500/15 text-amber-300 border-amber-500/30"
			}
		},
		"statusLabel": func(status models.ParticipantStatus) string {
			switch status {
			case models.StatusVerified:
				return "Terverifikasi"
			case models.StatusRejected:
				return "Ditolak"
			default:
				return "Menunggu"
			}
		},
		"isPDF": func(filename string) bool {
			return strings.ToLower(filepath.Ext(filename)) == ".pdf"
		},
		"isEq": func(a, b string) bool { return a == b },
		"add": func(a, b int) int { return a + b },
		"truncate": func(n int, s string) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "…"
		},
		"defaultStr": func(fallback string, s interface{}) string {
			if s == nil {
				return fallback
			}
			if strPtr, ok := s.(*string); ok {
				if strPtr == nil || strings.TrimSpace(*strPtr) == "" {
					return fallback
				}
				return *strPtr
			}
			if str, ok := s.(string); ok {
				if strings.TrimSpace(str) == "" {
					return fallback
				}
				return str
			}
			return fallback
		},
	}
}
