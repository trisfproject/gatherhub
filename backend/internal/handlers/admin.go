package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gatherhub/backend/internal/models"
	"github.com/gatherhub/backend/internal/services"
	templ "github.com/gatherhub/backend/internal/templates"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
)

// ─────────────────────── Data structs ───────────────────────

type AdminLoginData struct {
	Error string
}

type AdminDashboardData struct {
	AdminUser           string
	AdminRole           string
	Stats               *services.ParticipantStats
	RecentRegistrations []models.Participant
	RecentVerifications []models.Participant
	Events              []models.Event
	SelectedEventID     uint
	SelectedEvent       *models.Event
	StartDate           string
	EndDate             string
	AnalyticsJSON       string
}

type AdminParticipantsData struct {
	AdminUser    string
	AdminRole    string
	Participants []models.Participant
	Stats        *services.ParticipantStats
	Filter       string
	Search       string
	Event        *models.Event
	Page         int
	TotalPages   int
	TotalItems   int64
	HasPrev      bool
	HasNext      bool
	PrevPage     int
	NextPage     int
	Pages        []int
}

type AdminParticipantDetailData struct {
	AdminUser   string
	AdminRole   string
	Participant *models.Participant
	Event       *models.Event
	PaymentURL  string
	Stats       *services.ParticipantStats
}

type AdminEventsData struct {
	AdminUser    string
	AdminRole    string
	Events       []models.Event
	Stats        *services.ParticipantStats
	FlashSuccess string
	FlashError   string
}

type AdminEventCreateData struct {
	AdminUser string
	AdminRole string
	Errors    []string
	Form      map[string]string
	Stats     *services.ParticipantStats
}

type AdminEventEditData struct {
	AdminUser string
	AdminRole string
	Event     *models.Event
	Errors    []string
	Form      map[string]string
	Stats     *services.ParticipantStats
}

type AdminEventDetailData struct {
	AdminUser    string
	AdminRole    string
	Event        *models.Event
	Stats        *services.ParticipantStats
	FlashSuccess string
	FlashError   string
}

type AdminAdminsData struct {
	AdminUser string
	AdminRole string
	Admins    []models.Admin
	Stats     *services.ParticipantStats
}

type AdminAdminCreateData struct {
	AdminUser string
	AdminRole string
	Errors    []string
	Form      map[string]string
	Stats     *services.ParticipantStats
}

type AdminAdminEditData struct {
	AdminUser string
	AdminRole string
	Admin     *models.Admin
	Errors    []string
	Form      map[string]string
	Stats     *services.ParticipantStats
}

type AdminSettingsData struct {
	AdminUser      string
	AdminRole      string
	PlatformName   string
	Maintenance    bool
	MaxUploadSize  string
	SuccessMessage string
	Errors         []string
	Stats          *services.ParticipantStats
}

// ─────────────────────── Handler ───────────────────────

// AdminHandler handles all admin panel routes
type AdminHandler struct {
	participantService *services.ParticipantService
	eventService       *services.EventService
	store              *session.Store
	adminService       *services.AdminService
	storageService     *services.StorageService
	tmpl               *template.Template
}

// NewAdminHandler creates and initialises an AdminHandler
func NewAdminHandler(
	participantService *services.ParticipantService,
	eventService *services.EventService,
	store *session.Store,
	adminService *services.AdminService,
	storageService *services.StorageService,
) (*AdminHandler, error) {
	funcMap := buildAdminFuncMap()
	t, err := template.New("").Funcs(funcMap).ParseFS(
		templ.Files,
		"admin_login.html",
		"admin_dashboard.html",
		"admin_participants.html",
		"admin_participant.html",
		"admin_events.html",
		"admin_event.html",
		"admin_event_create.html",
		"admin_event_edit.html",
		"admin_admins.html",
		"admin_admin_create.html",
		"admin_admin_edit.html",
		"admin_settings.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse admin templates: %w", err)
	}
	return &AdminHandler{
		participantService: participantService,
		eventService:       eventService,
		store:              store,
		adminService:       adminService,
		storageService:     storageService,
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

	admin, err := h.adminService.Authenticate(username, password)
	if err != nil {
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
	sess.Set("admin_username", admin.Username)
	sess.Set("admin_role", admin.Role)
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
	eventIDStr := c.Query("event_id")
	var selectedEventID uint
	if eventIDStr != "" {
		if id, err := strconv.Atoi(eventIDStr); err == nil {
			selectedEventID = uint(id)
		}
	} else {
		// Default to first published event
		if firstEvent, err := h.eventService.GetFirst(); err == nil && firstEvent != nil {
			selectedEventID = firstEvent.ID
		}
	}

	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))

	stats, err := h.participantService.GetFilteredStats(selectedEventID, startDate, endDate)
	if err != nil {
		stats = &services.ParticipantStats{}
	}

	recentRegs, _ := h.participantService.GetLatestRegistrations(selectedEventID, 5)
	recentVerifs, _ := h.participantService.GetLatestVerifications(selectedEventID, 5)

	events, _ := h.eventService.GetAll()

	var selectedEvent *models.Event
	for i := range events {
		if events[i].ID == selectedEventID {
			selectedEvent = &events[i]
			break
		}
	}

	analytics, err := h.participantService.GetAnalytics(selectedEventID, startDate, endDate)
	var analyticsJSON string
	if err == nil && analytics != nil {
		if bytes, err := json.Marshal(analytics); err == nil {
			analyticsJSON = string(bytes)
		}
	}
	if analyticsJSON == "" {
		analyticsJSON = "{}"
	}

	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	return h.render(c, "admin_dashboard.html", AdminDashboardData{
		AdminUser:           adminUser,
		AdminRole:           adminRole,
		Stats:               stats,
		RecentRegistrations: recentRegs,
		RecentVerifications: recentVerifs,
		Events:              events,
		SelectedEventID:     selectedEventID,
		SelectedEvent:       selectedEvent,
		StartDate:           startDate,
		EndDate:             endDate,
		AnalyticsJSON:       analyticsJSON,
	})
}

// ParticipantList handles GET /admin/participants
func (h *AdminHandler) ParticipantList(c *fiber.Ctx) error {
	filter := c.Query("status")
	search := strings.TrimSpace(c.Query("q"))
	page := c.QueryInt("page", 1)
	if page <= 0 {
		page = 1
	}
	limit := 10

	participants, totalItems, err := h.participantService.GetPaginatedForAdmin(filter, search, page, limit)
	if err != nil {
		participants = []models.Participant{}
		totalItems = 0
	}

	totalPages := int((totalItems + int64(limit) - 1) / int64(limit))
	if totalPages <= 0 {
		totalPages = 1
	}

	hasPrev := page > 1
	hasNext := page < totalPages
	prevPage := page - 1
	nextPage := page + 1

	var pages []int
	for i := 1; i <= totalPages; i++ {
		pages = append(pages, i)
	}

	stats, _ := h.participantService.GetStats()
	event, _ := h.eventService.GetFirst()
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	return h.render(c, "admin_participants.html", AdminParticipantsData{
		AdminUser:    adminUser,
		AdminRole:    adminRole,
		Participants: participants,
		Stats:        stats,
		Filter:       filter,
		Search:       search,
		Event:        event,
		Page:         page,
		TotalPages:   totalPages,
		TotalItems:   totalItems,
		HasPrev:      hasPrev,
		HasNext:      hasNext,
		PrevPage:     prevPage,
		NextPage:     nextPage,
		Pages:        pages,
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
	adminRole, _ := c.Locals("admin_role").(string)

	paymentURL := ""
	if participant.PaymentProof != "" {
		paymentURL = "/payments/" + participant.PaymentProof
	}

	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_participant.html", AdminParticipantDetailData{
		AdminUser:   adminUser,
		AdminRole:   adminRole,
		Participant: participant,
		Event:       event,
		PaymentURL:  paymentURL,
		Stats:       stats,
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

	p, err := h.participantService.UpdateStatus(uint(id), status)
	if err != nil {
		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusBadRequest).SendString(
				`<div class="text-red-400 text-sm font-semibold">Gagal memperbarui status.</div>`)
		}
		return c.Redirect(fmt.Sprintf("/admin/participants/%d", id), fiber.StatusSeeOther)
	}

	// HTMX: return updated status badge + action buttons fragment
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Trigger", "statusUpdated")
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(buildStatusFragment(p))
	}

	return c.Redirect(fmt.Sprintf("/admin/participants/%d", id), fiber.StatusSeeOther)
}
// ExportParticipants handles GET /admin/participants/export
// Generates and streams a .xlsx file of all participants (respects status/search filters).
func (h *AdminHandler) ExportParticipants(c *fiber.Ctx) error {
	statusFilter := strings.ToUpper(strings.TrimSpace(c.Query("status")))
	search := strings.TrimSpace(c.Query("q"))

	// Fetch all matching participants (no pagination)
	participants, err := h.participantService.GetAllForAdmin(statusFilter, search)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal mengambil data peserta")
	}

	// Determine event slug for filename
	eventSlug := "event"
	if event, err := h.eventService.GetFirst(); err == nil && event != nil {
		eventSlug = event.Slug
	}

	// Build XLSX
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Peserta"
	f.SetSheetName("Sheet1", sheet)

	// ── Header style ──────────────────────────────────────────
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"4F46E5"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "CCCCCC", Style: 1},
			{Type: "right", Color: "CCCCCC", Style: 1},
			{Type: "top", Color: "CCCCCC", Style: 1},
			{Type: "bottom", Color: "CCCCCC", Style: 1},
		},
	})

	// ── Data style ────────────────────────────────────────────
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: false},
		Border: []excelize.Border{
			{Type: "left", Color: "E5E7EB", Style: 1},
			{Type: "right", Color: "E5E7EB", Style: 1},
			{Type: "bottom", Color: "E5E7EB", Style: 1},
		},
	})

	// ── Headers ───────────────────────────────────────────────
	headers := []struct {
		col   string
		label string
		width float64
	}{
		{"A", "No. Registrasi", 18},
		{"B", "Nama Lengkap", 28},
		{"C", "WhatsApp", 18},
		{"D", "Email", 30},
		{"E", "Kota", 16},
		{"F", "Nama Perusahaan", 28},
		{"G", "Kawasan Industri", 24},
		{"H", "Username Telegram", 20},
		{"I", "Jabatan", 20},
		{"J", "Status", 14},
		{"K", "Tanggal Daftar", 20},
	}

	for _, h := range headers {
		cell := h.col + "1"
		f.SetCellValue(sheet, cell, h.label)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
		f.SetColWidth(sheet, h.col, h.col, h.width)
	}
	f.SetRowHeight(sheet, 1, 22)

	// ── Rows ──────────────────────────────────────────────────
	for i, p := range participants {
		row := i + 2
		rowStr := strconv.Itoa(row)

		jobTitle := ""
		if p.JobTitle != nil {
			jobTitle = *p.JobTitle
		}

		statusLabel := string(p.Status)
		switch p.Status {
		case models.StatusVerified:
			statusLabel = "TERVERIFIKASI"
		case models.StatusRejected:
			statusLabel = "DITOLAK"
		case models.StatusPending:
			statusLabel = "MENUNGGU"
		}

		values := []interface{}{
			p.RegistrationNumber,
			p.FullName,
			p.Phone,
			p.Email,
			p.City,
			p.CompanyName,
			p.IndustrialEstate,
			p.TelegramUsername,
			jobTitle,
			statusLabel,
			p.CreatedAt.Format("02/01/2006 15:04"),
		}

		cols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K"}
		for ci, col := range cols {
			cell := col + rowStr
			f.SetCellValue(sheet, cell, values[ci])
			f.SetCellStyle(sheet, cell, cell, dataStyle)
		}
		f.SetRowHeight(sheet, row, 18)
	}

	// ── Freeze header row ─────────────────────────────────────
	f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	// ── Auto-filter on header row ─────────────────────────────
	f.AutoFilter(sheet, "A1:K1", []excelize.AutoFilterOptions{})

	// ── Stream to response ────────────────────────────────────
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal membuat file Excel")
	}

	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("participants-%s-%s.xlsx", eventSlug, date)

	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Send(buf.Bytes())
}

func (h *AdminHandler) EventList(c *fiber.Ctx) error {
	events, err := h.eventService.GetAll()
	if err != nil {
		events = []models.Event{}
	}
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()
	return h.render(c, "admin_events.html", AdminEventsData{
		AdminUser:    adminUser,
		AdminRole:    adminRole,
		Events:       events,
		Stats:        stats,
		FlashSuccess: getFlash(c, "success"),
		FlashError:   getFlash(c, "error"),
	})
}

// EventCreatePage handles GET /admin/events/create
func (h *AdminHandler) EventCreatePage(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()
	return h.render(c, "admin_event_create.html", AdminEventCreateData{
		AdminUser: adminUser,
		AdminRole: adminRole,
		Form:      map[string]string{},
		Stats:     stats,
	})
}

// EventCreateSubmit handles POST /admin/events/create
func (h *AdminHandler) EventCreateSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	title := strings.TrimSpace(c.FormValue("title"))
	slug := strings.TrimSpace(strings.ToLower(c.FormValue("slug")))
	description := strings.TrimSpace(c.FormValue("description"))
	eventDateStr := c.FormValue("event_date")
	eventTime := strings.TrimSpace(c.FormValue("event_time"))
	location := strings.TrimSpace(c.FormValue("location"))
	priceStr := strings.TrimSpace(c.FormValue("price"))
	paymentBank := strings.TrimSpace(c.FormValue("payment_bank"))
	paymentAccountNumber := strings.TrimSpace(c.FormValue("payment_account_number"))
	paymentAccountName := strings.TrimSpace(c.FormValue("payment_account_name"))
	adminName := strings.TrimSpace(c.FormValue("admin_name"))
	adminWhatsapp := strings.TrimSpace(c.FormValue("admin_whatsapp"))
	maxParticipantsStr := strings.TrimSpace(c.FormValue("max_participants"))
	regOpenStr := c.FormValue("registration_open")
	regCloseStr := c.FormValue("registration_close")
	status := strings.ToUpper(strings.TrimSpace(c.FormValue("status")))

	formValues := map[string]string{
		"title":                  title,
		"slug":                   slug,
		"description":            description,
		"event_date":             eventDateStr,
		"event_time":             eventTime,
		"location":               location,
		"price":                  priceStr,
		"payment_bank":           paymentBank,
		"payment_account_number": paymentAccountNumber,
		"payment_account_name":   paymentAccountName,
		"admin_name":             adminName,
		"admin_whatsapp":         adminWhatsapp,
		"max_participants":       maxParticipantsStr,
		"registration_open":      regOpenStr,
		"registration_close":     regCloseStr,
		"status":                 status,
	}

	var errs []string
	if title == "" { errs = append(errs, "Judul Acara wajib diisi") }
	if slug == "" { errs = append(errs, "Slug Acara wajib diisi") }
	if location == "" { errs = append(errs, "Lokasi wajib diisi") }
	if adminName == "" { errs = append(errs, "Nama Admin wajib diisi") }
	if adminWhatsapp == "" { errs = append(errs, "WhatsApp Admin wajib diisi") }

	slugRegex := regexp.MustCompile(`^[a-z0-9-_]+$`)
	if slug != "" && !slugRegex.MatchString(slug) {
		errs = append(errs, "Slug hanya boleh berisi huruf kecil, angka, tanda hubung (-), dan garis bawah (_)")
	}

	if slug != "" {
		if _, err := h.eventService.GetBySlug(slug); err == nil {
			errs = append(errs, "Slug sudah digunakan oleh acara lain")
		}
	}

	var eventDate time.Time
	var err error
	if eventDateStr != "" {
		eventDate, err = time.Parse("2006-01-02", eventDateStr)
		if err != nil {
			errs = append(errs, "Format Tanggal Acara tidak valid")
		}
	} else {
		errs = append(errs, "Tanggal Acara wajib diisi")
	}

	var regOpen time.Time
	if regOpenStr != "" {
		regOpen, err = time.Parse("2006-01-02T15:04", regOpenStr)
		if err != nil {
			errs = append(errs, "Format Tanggal Pendaftaran Dibuka tidak valid (gunakan format lokal YYYY-MM-DDTHH:MM)")
		}
	}

	var regClose time.Time
	if regCloseStr != "" {
		regClose, err = time.Parse("2006-01-02T15:04", regCloseStr)
		if err != nil {
			errs = append(errs, "Format Tanggal Pendaftaran Ditutup tidak valid (gunakan format lokal YYYY-MM-DDTHH:MM)")
		}
	}

	price := 0.0
	if priceStr != "" {
		price, err = strconv.ParseFloat(priceStr, 64)
		if err != nil || price < 0 {
			errs = append(errs, "Format Biaya Pendaftaran tidak valid")
		}
	}

	maxParticipants := 0
	if maxParticipantsStr != "" {
		maxParticipants, err = strconv.Atoi(maxParticipantsStr)
		if err != nil || maxParticipants < 0 {
			errs = append(errs, "Format Maksimum Peserta tidak valid")
		}
	}

	allowedStatus := map[string]bool{"DRAFT": true, "PUBLISHED": true, "CLOSED": true}
	if status == "" {
		status = "DRAFT"
	} else if !allowedStatus[status] {
		errs = append(errs, "Status tidak valid")
	}

	// Image upload
	bannerFilename := ""
	file, err := c.FormFile("banner_image")
	if err == nil && file != nil {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
		if !allowedExts[ext] {
			errs = append(errs, "Format Banner hanya boleh JPG, JPEG, PNG, atau WEBP")
		} else if file.Size > 10*1024*1024 {
			errs = append(errs, "Ukuran Banner maksimal 10MB")
		} else {
			bannerFilename, err = h.storageService.SaveEventBanner(file)
			if err != nil {
				errs = append(errs, "Gagal mengunggah Banner: "+err.Error())
			}
		}
	}

	if len(errs) > 0 {
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_event_create.html", AdminEventCreateData{
			AdminUser: adminUser,
			AdminRole: adminRole,
			Errors:    errs,
			Form:      formValues,
			Stats:     stats,
		})
	}

	newEvent := &models.Event{
		Title:                title,
		Slug:                 slug,
		Description:          description,
		BannerImage:          bannerFilename,
		EventDate:            eventDate,
		EventTime:            eventTime,
		Location:             location,
		Price:                price,
		PaymentBank:          paymentBank,
		PaymentAccountNumber: paymentAccountNumber,
		PaymentAccountName:   paymentAccountName,
		AdminName:            adminName,
		AdminWhatsapp:        adminWhatsapp,
		MaxParticipants:      maxParticipants,
		RegistrationOpen:     regOpen,
		RegistrationClose:    regClose,
		Status:               status,
	}

	if err := h.eventService.Create(newEvent); err != nil {
		errs = append(errs, "Gagal menyimpan Acara: "+err.Error())
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_event_create.html", AdminEventCreateData{
			AdminUser: adminUser,
			AdminRole: adminRole,
			Errors:    errs,
			Form:      formValues,
			Stats:     stats,
		})
	}

	setFlash(c, "success", "Acara \""+title+"\" berhasil dibuat.")
	return c.Redirect("/admin/events", fiber.StatusSeeOther)
}

// EventEditPage handles GET /admin/events/:id/edit
func (h *AdminHandler) EventEditPage(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Redirect("/admin/events", fiber.StatusSeeOther)
	}

	event, err := h.eventService.GetByID(uint(id))
	if err != nil {
		return c.Redirect("/admin/events", fiber.StatusSeeOther)
	}

	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	formValues := map[string]string{
		"title":                  event.Title,
		"slug":                   event.Slug,
		"description":            event.Description,
		"event_date":             event.EventDate.Format("2006-01-02"),
		"event_time":             event.EventTime,
		"location":               event.Location,
		"price":                  fmt.Sprintf("%.0f", event.Price),
		"payment_bank":           event.PaymentBank,
		"payment_account_number": event.PaymentAccountNumber,
		"payment_account_name":   event.PaymentAccountName,
		"admin_name":             event.AdminName,
		"admin_whatsapp":         event.AdminWhatsapp,
		"max_participants":       strconv.Itoa(event.MaxParticipants),
		"status":                 event.Status,
	}
	if !event.RegistrationOpen.IsZero() {
		formValues["registration_open"] = event.RegistrationOpen.Format("2006-01-02T15:04")
	}
	if !event.RegistrationClose.IsZero() {
		formValues["registration_close"] = event.RegistrationClose.Format("2006-01-02T15:04")
	}

	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_event_edit.html", AdminEventEditData{
		AdminUser: adminUser,
		AdminRole: adminRole,
		Event:     event,
		Form:      formValues,
		Stats:     stats,
	})
}

// EventEditSubmit handles POST /admin/events/:id/edit
func (h *AdminHandler) EventEditSubmit(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Redirect("/admin/events", fiber.StatusSeeOther)
	}

	event, err := h.eventService.GetByID(uint(id))
	if err != nil {
		return c.Redirect("/admin/events", fiber.StatusSeeOther)
	}

	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	title := strings.TrimSpace(c.FormValue("title"))
	slug := strings.TrimSpace(strings.ToLower(c.FormValue("slug")))
	description := strings.TrimSpace(c.FormValue("description"))
	eventDateStr := c.FormValue("event_date")
	eventTime := strings.TrimSpace(c.FormValue("event_time"))
	location := strings.TrimSpace(c.FormValue("location"))
	priceStr := strings.TrimSpace(c.FormValue("price"))
	paymentBank := strings.TrimSpace(c.FormValue("payment_bank"))
	paymentAccountNumber := strings.TrimSpace(c.FormValue("payment_account_number"))
	paymentAccountName := strings.TrimSpace(c.FormValue("payment_account_name"))
	adminName := strings.TrimSpace(c.FormValue("admin_name"))
	adminWhatsapp := strings.TrimSpace(c.FormValue("admin_whatsapp"))
	maxParticipantsStr := strings.TrimSpace(c.FormValue("max_participants"))
	regOpenStr := c.FormValue("registration_open")
	regCloseStr := c.FormValue("registration_close")
	status := strings.ToUpper(strings.TrimSpace(c.FormValue("status")))

	formValues := map[string]string{
		"title":                  title,
		"slug":                   slug,
		"description":            description,
		"event_date":             eventDateStr,
		"event_time":             eventTime,
		"location":               location,
		"price":                  priceStr,
		"payment_bank":           paymentBank,
		"payment_account_number": paymentAccountNumber,
		"payment_account_name":   paymentAccountName,
		"admin_name":             adminName,
		"admin_whatsapp":         adminWhatsapp,
		"max_participants":       maxParticipantsStr,
		"registration_open":      regOpenStr,
		"registration_close":     regCloseStr,
		"status":                 status,
	}

	var errs []string
	if title == "" { errs = append(errs, "Judul Acara wajib diisi") }
	if slug == "" { errs = append(errs, "Slug Acara wajib diisi") }
	if location == "" { errs = append(errs, "Lokasi wajib diisi") }
	if adminName == "" { errs = append(errs, "Nama Admin wajib diisi") }
	if adminWhatsapp == "" { errs = append(errs, "WhatsApp Admin wajib diisi") }

	slugRegex := regexp.MustCompile(`^[a-z0-9-_]+$`)
	if slug != "" && !slugRegex.MatchString(slug) {
		errs = append(errs, "Slug hanya boleh berisi huruf kecil, angka, tanda hubung (-), dan garis bawah (_)")
	}

	if slug != "" && slug != event.Slug {
		if _, err := h.eventService.GetBySlug(slug); err == nil {
			errs = append(errs, "Slug sudah digunakan oleh acara lain")
		}
	}

	var eventDate time.Time
	if eventDateStr != "" {
		eventDate, err = time.Parse("2006-01-02", eventDateStr)
		if err != nil {
			errs = append(errs, "Format Tanggal Acara tidak valid")
		}
	} else {
		errs = append(errs, "Tanggal Acara wajib diisi")
	}

	var regOpen time.Time
	if regOpenStr != "" {
		regOpen, err = time.Parse("2006-01-02T15:04", regOpenStr)
		if err != nil {
			errs = append(errs, "Format Tanggal Pendaftaran Dibuka tidak valid (gunakan format lokal YYYY-MM-DDTHH:MM)")
		}
	}

	var regClose time.Time
	if regCloseStr != "" {
		regClose, err = time.Parse("2006-01-02T15:04", regCloseStr)
		if err != nil {
			errs = append(errs, "Format Tanggal Pendaftaran Ditutup tidak valid (gunakan format lokal YYYY-MM-DDTHH:MM)")
		}
	}

	price := 0.0
	if priceStr != "" {
		price, err = strconv.ParseFloat(priceStr, 64)
		if err != nil || price < 0 {
			errs = append(errs, "Format Biaya Pendaftaran tidak valid")
		}
	}

	maxParticipants := 0
	if maxParticipantsStr != "" {
		maxParticipants, err = strconv.Atoi(maxParticipantsStr)
		if err != nil || maxParticipants < 0 {
			errs = append(errs, "Format Maksimum Peserta tidak valid")
		}
	}

	allowedStatus := map[string]bool{"DRAFT": true, "PUBLISHED": true, "CLOSED": true}
	if status == "" {
		status = "DRAFT"
	} else if !allowedStatus[status] {
		errs = append(errs, "Status tidak valid")
	}

	// Banner image update
	bannerFilename := event.BannerImage
	file, err := c.FormFile("banner_image")
	if err == nil && file != nil {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
		if !allowedExts[ext] {
			errs = append(errs, "Format Banner hanya boleh JPG, JPEG, PNG, atau WEBP")
		} else if file.Size > 10*1024*1024 {
			errs = append(errs, "Ukuran Banner maksimal 10MB")
		} else {
			bannerFilename, err = h.storageService.SaveEventBanner(file)
			if err != nil {
				errs = append(errs, "Gagal mengunggah Banner: "+err.Error())
			}
		}
	}

	if len(errs) > 0 {
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_event_edit.html", AdminEventEditData{
			AdminUser: adminUser,
			AdminRole: adminRole,
			Event:     event,
			Errors:    errs,
			Form:      formValues,
			Stats:     stats,
		})
	}

	event.Title = title
	event.Slug = slug
	event.Description = description
	event.BannerImage = bannerFilename
	event.EventDate = eventDate
	event.EventTime = eventTime
	event.Location = location
	event.Price = price
	event.PaymentBank = paymentBank
	event.PaymentAccountNumber = paymentAccountNumber
	event.PaymentAccountName = paymentAccountName
	event.AdminName = adminName
	event.AdminWhatsapp = adminWhatsapp
	event.MaxParticipants = maxParticipants
	event.RegistrationOpen = regOpen
	event.RegistrationClose = regClose
	event.Status = status

	if err := h.eventService.Update(event); err != nil {
		errs = append(errs, "Gagal memperbarui Acara: "+err.Error())
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_event_edit.html", AdminEventEditData{
			AdminUser: adminUser,
			AdminRole: adminRole,
			Event:     event,
			Errors:    errs,
			Form:      formValues,
			Stats:     stats,
		})
	}

	setFlash(c, "success", "Acara \""+title+"\" berhasil diperbarui.")
	return c.Redirect("/admin/events", fiber.StatusSeeOther)
}

// EventDelete handles POST /admin/events/:id/delete or DELETE /admin/events/:id
func (h *AdminHandler) EventDelete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	if err := h.eventService.Delete(uint(id)); err != nil {
		setFlash(c, "error", "Gagal menghapus acara: "+err.Error())
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/admin/events")
			return c.SendStatus(fiber.StatusOK)
		}
		return c.Redirect("/admin/events", fiber.StatusSeeOther)
	}

	setFlash(c, "success", "Acara berhasil dihapus.")
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/admin/events")
		return c.SendStatus(fiber.StatusOK)
	}
	return c.Redirect("/admin/events", fiber.StatusSeeOther)
}

// EventDetail handles GET /admin/events/:id
func (h *AdminHandler) EventDetail(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Redirect("/admin/events", fiber.StatusSeeOther)
	}

	event, err := h.eventService.GetByID(uint(id))
	if err != nil {
		return c.Redirect("/admin/events", fiber.StatusSeeOther)
	}

	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_event.html", AdminEventDetailData{
		AdminUser:    adminUser,
		AdminRole:    adminRole,
		Event:        event,
		Stats:        stats,
		FlashSuccess: getFlash(c, "success"),
		FlashError:   getFlash(c, "error"),
	})
}

// EventUpdateStatus handles POST /admin/events/:id/status
func (h *AdminHandler) EventUpdateStatus(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Redirect("/admin/events", fiber.StatusSeeOther)
	}

	status := strings.ToUpper(strings.TrimSpace(c.FormValue("status")))
	allowedStatus := map[string]bool{"DRAFT": true, "PUBLISHED": true, "CLOSED": true}
	if !allowedStatus[status] {
		return c.Redirect(fmt.Sprintf("/admin/events/%d", id), fiber.StatusSeeOther)
	}

	event, err := h.eventService.GetByID(uint(id))
	if err != nil {
		return c.Redirect("/admin/events", fiber.StatusSeeOther)
	}

	event.Status = status
	if err := h.eventService.Update(event); err != nil {
		setFlash(c, "error", "Gagal memperbarui status acara.")
		return c.Redirect(fmt.Sprintf("/admin/events/%d", id), fiber.StatusSeeOther)
	}

	statusLabels := map[string]string{
		"PUBLISHED": "diterbitkan",
		"CLOSED":    "ditutup",
		"DRAFT":     "dikembalikan ke Draft",
	}
	setFlash(c, "success", "Acara \"" + event.Title + "\" berhasil " + statusLabels[status] + ".")
	return c.Redirect(fmt.Sprintf("/admin/events/%d", id), fiber.StatusSeeOther)
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

// setFlash sets a one-time flash cookie ("flash_success" or "flash_error").
func setFlash(c *fiber.Ctx, kind, message string) {
	c.Cookie(&fiber.Cookie{
		Name:     "flash_" + kind,
		Value:    message,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		MaxAge:   30, // 30 seconds — consumed on the very next page load
	})
}

// getFlash reads and immediately clears a flash cookie.
func getFlash(c *fiber.Ctx, kind string) string {
	val := c.Cookies("flash_" + kind)
	if val != "" {
		// Clear the cookie by expiring it
		c.Cookie(&fiber.Cookie{
			Name:   "flash_" + kind,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
	}
	return val
}

// buildStatusFragment returns an HTML snippet for HTMX swap after status update
func buildStatusFragment(p *models.Participant) string {
	badge := statusBadgeHTML(p.Status)
	actions := ""
	switch p.Status {
	case models.StatusPending:
		actions = fmt.Sprintf(`
<button onclick="confirmAction('/admin/participants/%d/status', 'VERIFIED', 'Apakah Anda yakin ingin memverifikasi pendaftaran %s?', 'VERIFIED')"
  class="flex-1 flex items-center justify-center gap-2 bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-2.5 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7"/></svg>
  Verifikasi
</button>
<button onclick="confirmAction('/admin/participants/%d/status', 'REJECTED', 'Apakah Anda yakin ingin menolak pendaftaran %s?', 'REJECTED')"
  class="flex-1 flex items-center justify-center gap-2 bg-red-700 hover:bg-red-600 text-white font-bold py-2.5 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
  Tolak
</button>`, p.ID, p.FullName, p.ID, p.FullName)
	case models.StatusVerified:
		actions = fmt.Sprintf(`
<button onclick="confirmAction('/admin/participants/%d/status', 'REJECTED', 'Apakah Anda yakin ingin menolak pendaftaran %s?', 'REJECTED')"
  class="flex-1 flex items-center justify-center gap-2 bg-red-700 hover:bg-red-600 text-white font-bold py-2.5 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
  Ubah ke Ditolak
</button>
<button onclick="confirmAction('/admin/participants/%d/status', 'PENDING', 'Apakah Anda yakin ingin mengembalikan pendaftaran %s ke status Pending?', 'PENDING')"
  class="flex-1 flex items-center justify-center gap-2 bg-amber-600 hover:bg-amber-500 text-white font-bold py-2.5 px-5 rounded-xl text-sm transition-colors">
  Kembalikan ke Pending
</button>`, p.ID, p.FullName, p.ID, p.FullName)
	case models.StatusRejected:
		actions = fmt.Sprintf(`
<button onclick="confirmAction('/admin/participants/%d/status', 'VERIFIED', 'Apakah Anda yakin ingin memverifikasi pendaftaran %s?', 'VERIFIED')"
  class="flex-1 flex items-center justify-center gap-2 bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-2.5 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7"/></svg>
  Verifikasi
</button>
<button onclick="confirmAction('/admin/participants/%d/status', 'PENDING', 'Apakah Anda yakin ingin mengembalikan pendaftaran %s ke status Pending?', 'PENDING')"
  class="flex-1 flex items-center justify-center gap-2 bg-amber-600 hover:bg-amber-500 text-white font-bold py-2.5 px-5 rounded-xl text-sm transition-colors">
  Kembalikan ke Pending
</button>`, p.ID, p.FullName, p.ID, p.FullName)
	}

	historyHTML := ""
	if p.VerifiedAt != nil {
		historyHTML = fmt.Sprintf(`
<div class="flex items-start gap-3 bg-emerald-500/10 border border-emerald-500/20 rounded-xl p-4">
  <div class="w-8 h-8 rounded-full bg-emerald-500/20 flex items-center justify-center flex-shrink-0 text-emerald-400">
    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7"/></svg>
  </div>
  <div>
    <p class="text-sm font-bold text-emerald-300">Pendaftaran Diverifikasi & Disetujui</p>
    <p class="text-xs text-white/45 mt-1">Diproses pada: %s WIB</p>
  </div>
</div>`, p.VerifiedAt.Format("02/01/2006 15:04"))
	} else if p.RejectedAt != nil {
		historyHTML = fmt.Sprintf(`
<div class="flex items-start gap-3 bg-red-500/10 border border-red-500/20 rounded-xl p-4">
  <div class="w-8 h-8 rounded-full bg-red-500/20 flex items-center justify-center flex-shrink-0 text-red-400">
    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M6 18L18 6M6 6l12 12"/></svg>
  </div>
  <div>
    <p class="text-sm font-bold text-red-300">Pendaftaran Ditolak</p>
    <p class="text-xs text-white/45 mt-1">Diproses pada: %s WIB</p>
  </div>
</div>`, p.RejectedAt.Format("02/01/2006 15:04"))
	} else {
		historyHTML = `
<div class="flex items-start gap-3 bg-white/3 border border-white/8 rounded-xl p-4">
  <div class="w-8 h-8 rounded-full bg-white/5 flex items-center justify-center flex-shrink-0 text-white/30">
    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
  </div>
  <div>
    <p class="text-sm font-bold text-white/60">Menunggu Verifikasi (Pending)</p>
    <p class="text-xs text-white/30 mt-1">Pendaftar baru masuk ke sistem dan belum diproses.</p>
  </div>
</div>`
	}

	return fmt.Sprintf(`
<div class="flex items-center gap-3 mb-4">%s</div>
<div class="flex gap-3">%s</div>

<div id="history-card" hx-swap-oob="true">
  <div class="glass p-6 fade-up" style="animation-delay: .15s">
    <p class="text-xs font-bold uppercase tracking-widest text-indigo-400 mb-4">Riwayat Verifikasi</p>
    <div class="space-y-4">
      %s
    </div>
  </div>
</div>`, badge, actions, historyHTML)
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

// ─────────────────────── Admin Management Handlers (SUPER_ADMIN) ───────────────────────

// AdminList handles GET /admin/admins
func (h *AdminHandler) AdminList(c *fiber.Ctx) error {
	admins, err := h.adminService.GetAllAdmins()
	if err != nil {
		admins = []models.Admin{}
	}
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_admins.html", AdminAdminsData{
		AdminUser: adminUser,
		AdminRole: adminRole,
		Admins:    admins,
		Stats:     stats,
	})
}

// AdminCreatePage handles GET /admin/admins/create
func (h *AdminHandler) AdminCreatePage(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_admin_create.html", AdminAdminCreateData{
		AdminUser: adminUser,
		AdminRole: adminRole,
		Form:      map[string]string{},
		Stats:     stats,
	})
}

// AdminCreateSubmit handles POST /admin/admins/create
func (h *AdminHandler) AdminCreateSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	name := strings.TrimSpace(c.FormValue("name"))
	username := strings.TrimSpace(c.FormValue("username"))
	email := strings.TrimSpace(c.FormValue("email"))
	password := c.FormValue("password")
	role := strings.TrimSpace(c.FormValue("role"))

	formValues := map[string]string{
		"name":     name,
		"username": username,
		"email":    email,
		"role":     role,
	}

	var errs []string
	if name == "" { errs = append(errs, "Nama Lengkap wajib diisi") }
	if username == "" { errs = append(errs, "Username wajib diisi") }
	if email == "" { errs = append(errs, "Email wajib diisi") }
	if password == "" { errs = append(errs, "Password wajib diisi") }
	if role != "SUPER_ADMIN" && role != "ADMIN" { errs = append(errs, "Role tidak valid") }

	// Check if username/email already exists
	if username != "" {
		if _, err := h.adminService.GetByUsername(username); err == nil {
			errs = append(errs, "Username sudah digunakan")
		}
	}
	if email != "" {
		if _, err := h.adminService.GetByEmail(email); err == nil {
			errs = append(errs, "Email sudah digunakan")
		}
	}

	if len(errs) > 0 {
		return h.render(c, "admin_admin_create.html", AdminAdminCreateData{
			AdminUser: adminUser,
			AdminRole: adminRole,
			Errors:    errs,
			Form:      formValues,
			Stats:     stats,
		})
	}

	// Password hash using bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		errs = append(errs, "Gagal memproses password: "+err.Error())
		return h.render(c, "admin_admin_create.html", AdminAdminCreateData{
			AdminUser: adminUser,
			AdminRole: adminRole,
			Errors:    errs,
			Form:      formValues,
			Stats:     stats,
		})
	}

	newAdmin := &models.Admin{
		Name:         name,
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
	}

	if err := h.adminService.CreateAdmin(newAdmin); err != nil {
		errs = append(errs, "Gagal membuat Admin: "+err.Error())
		return h.render(c, "admin_admin_create.html", AdminAdminCreateData{
			AdminUser: adminUser,
			AdminRole: adminRole,
			Errors:    errs,
			Form:      formValues,
			Stats:     stats,
		})
	}

	return c.Redirect("/admin/admins", fiber.StatusSeeOther)
}

// AdminEditPage handles GET /admin/admins/:id/edit
func (h *AdminHandler) AdminEditPage(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Redirect("/admin/admins", fiber.StatusSeeOther)
	}

	admin, err := h.adminService.GetAdminByID(uint(id))
	if err != nil {
		return c.Redirect("/admin/admins", fiber.StatusSeeOther)
	}

	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	formValues := map[string]string{
		"name":     admin.Name,
		"username": admin.Username,
		"email":    admin.Email,
		"role":     admin.Role,
	}

	return h.render(c, "admin_admin_edit.html", AdminAdminEditData{
		AdminUser: adminUser,
		AdminRole: adminRole,
		Admin:     admin,
		Form:      formValues,
		Stats:     stats,
	})
}

// AdminEditSubmit handles POST /admin/admins/:id/edit
func (h *AdminHandler) AdminEditSubmit(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Redirect("/admin/admins", fiber.StatusSeeOther)
	}

	admin, err := h.adminService.GetAdminByID(uint(id))
	if err != nil {
		return c.Redirect("/admin/admins", fiber.StatusSeeOther)
	}

	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	name := strings.TrimSpace(c.FormValue("name"))
	email := strings.TrimSpace(c.FormValue("email"))
	password := c.FormValue("password")
	role := strings.TrimSpace(c.FormValue("role"))

	formValues := map[string]string{
		"name":     name,
		"username": admin.Username,
		"email":    email,
		"role":     role,
	}

	var errs []string
	if name == "" { errs = append(errs, "Nama Lengkap wajib diisi") }
	if email == "" { errs = append(errs, "Email wajib diisi") }
	if role != "SUPER_ADMIN" && role != "ADMIN" { errs = append(errs, "Role tidak valid") }

	// Check email uniqueness if modified
	if email != "" && email != admin.Email {
		if _, err := h.adminService.GetByEmail(email); err == nil {
			errs = append(errs, "Email sudah digunakan oleh admin lain")
		}
	}

	if len(errs) > 0 {
		return h.render(c, "admin_admin_edit.html", AdminAdminEditData{
			AdminUser: adminUser,
			AdminRole: adminRole,
			Admin:     admin,
			Errors:    errs,
			Form:      formValues,
			Stats:     stats,
		})
	}

	admin.Name = name
	admin.Email = email
	admin.Role = role

	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			errs = append(errs, "Gagal memproses password baru: "+err.Error())
			return h.render(c, "admin_admin_edit.html", AdminAdminEditData{
				AdminUser: adminUser,
				AdminRole: adminRole,
				Admin:     admin,
				Errors:    errs,
				Form:      formValues,
				Stats:     stats,
			})
		}
		admin.PasswordHash = string(hash)
	}

	if err := h.adminService.UpdateAdmin(admin); err != nil {
		errs = append(errs, "Gagal memperbarui Admin: "+err.Error())
		return h.render(c, "admin_admin_edit.html", AdminAdminEditData{
			AdminUser: adminUser,
			AdminRole: adminRole,
			Admin:     admin,
			Errors:    errs,
			Form:      formValues,
			Stats:     stats,
		})
	}

	return c.Redirect("/admin/admins", fiber.StatusSeeOther)
}

// AdminDelete handles POST /admin/admins/:id/delete or DELETE /admin/admins/:id
func (h *AdminHandler) AdminDelete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	admin, err := h.adminService.GetAdminByID(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Admin tidak ditemukan"})
	}

	loggedInUser, _ := c.Locals("admin_username").(string)
	if admin.Username == loggedInUser {
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Trigger", "adminDeleteError")
			return c.Status(fiber.StatusBadRequest).SendString("Anda tidak dapat menghapus akun Anda sendiri!")
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Anda tidak dapat menghapus akun Anda sendiri!"})
	}

	if err := h.adminService.DeleteAdmin(uint(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/admin/admins")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/admin/admins", fiber.StatusSeeOther)
}

// ─────────────────────── System Settings Handlers (SUPER_ADMIN) ───────────────────────

// SystemSettingsPage handles GET /admin/settings
func (h *AdminHandler) SystemSettingsPage(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	// Default/Mock settings
	return h.render(c, "admin_settings.html", AdminSettingsData{
		AdminUser:     adminUser,
		AdminRole:     adminRole,
		PlatformName:  "GatherHub",
		Maintenance:   false,
		MaxUploadSize: "10 MB",
		Stats:         stats,
	})
}

// SystemSettingsSubmit handles POST /admin/settings
func (h *AdminHandler) SystemSettingsSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	platformName := strings.TrimSpace(c.FormValue("platform_name"))
	maintenance := c.FormValue("maintenance") == "true"
	maxUploadSize := strings.TrimSpace(c.FormValue("max_upload_size"))

	var errs []string
	if platformName == "" { errs = append(errs, "Nama Platform wajib diisi") }
	if maxUploadSize == "" { errs = append(errs, "Maksimum Upload wajib diisi") }

	if len(errs) > 0 {
		return h.render(c, "admin_settings.html", AdminSettingsData{
			AdminUser:     adminUser,
			AdminRole:     adminRole,
			PlatformName:  platformName,
			Maintenance:   maintenance,
			MaxUploadSize: maxUploadSize,
			Errors:        errs,
			Stats:         stats,
		})
	}

	log.Printf("System settings updated by %s: Platform=%s, Maintenance=%v, MaxUpload=%s", adminUser, platformName, maintenance, maxUploadSize)

	return h.render(c, "admin_settings.html", AdminSettingsData{
		AdminUser:      adminUser,
		AdminRole:      adminRole,
		PlatformName:   platformName,
		Maintenance:    maintenance,
		MaxUploadSize:  maxUploadSize,
		SuccessMessage: "Pengaturan sistem berhasil disimpan!",
		Stats:          stats,
	})
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
		"formatPtrDateTime": func(t *time.Time) string {
			if t == nil {
				return ""
			}
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
