package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/trisfproject/gatherhub/internal/models"
	"github.com/trisfproject/gatherhub/internal/services"
	templ "github.com/trisfproject/gatherhub/internal/templates"
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

type ParticipantPage struct {
	Num       int
	URL       string
	IsCurrent bool
}

type AdminNotificationsData struct {
	AdminUser string
	AdminRole string
	Logs      []models.NotificationLog
	Stats     *services.ParticipantStats
	Event     *models.Event
}

type AdminParticipantsData struct {
	AdminUser      string
	AdminRole      string
	Participants   []models.Participant
	Stats          *services.ParticipantStats
	Filter         string
	Search         string
	Event          *models.Event
	Page           int
	TotalPages     int
	TotalItems     int64
	HasPrev        bool
	HasNext        bool
	PrevPage       int
	NextPage       int
	Pages          []int
	PageItems      []ParticipantPage
	ExportURL      string
	TabAllURL      string
	TabPendingURL  string
	TabVerifiedURL string
	TabRejectedURL string
	PagePrevURL    string
	PageNextURL    string
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
	AdminUser           string
	AdminRole           string
	SiteName            string
	SiteDescription     string
	FooterText          string
	RegistrationEnabled bool
	MaintenanceMode     bool
	SupportName         string
	SupportWhatsApp     string
	SupportEmail        string
	WhatsAppEnabled     bool
	NotificationEnabled bool
	StoragePath         string
	SuccessMessage      string
	Errors              []string
	Stats               *services.ParticipantStats
}

type AdminBackupsData struct {
	AdminUser    string
	AdminRole    string
	Backups      []services.BackupInfo
	Stats        *services.ParticipantStats
	FlashSuccess string
	FlashError   string
}

type AdminCheckinData struct {
	AdminUser    string
	AdminRole    string
	Participants []models.Participant
	Stats        *services.ParticipantStats
	Search       string
	FlashSuccess string
	FlashError   string
}

type AdminSystemHealthData struct {
	AdminUser string
	AdminRole string
	Health    *services.HealthReport
	Stats     *services.ParticipantStats
}

// ─────────────────────── Handler ───────────────────────

// AdminHandler handles all admin panel routes
type AdminHandler struct {
	participantService  *services.ParticipantService
	eventService        *services.EventService
	store               *session.Store
	adminService        *services.AdminService
	storageService      *services.StorageService
	notificationService *services.NotificationService
	auditLogService     *services.AuditLogService
	checkinService      *services.CheckinService
	settingsService     *services.SettingsService
	backupService       *services.BackupService
	broadcastService    *services.BroadcastService
	healthService       *services.HealthService
	tmpl                *template.Template
}

// NewAdminHandler creates and initialises an AdminHandler
func NewAdminHandler(
	participantService *services.ParticipantService,
	eventService *services.EventService,
	store *session.Store,
	adminService *services.AdminService,
	storageService *services.StorageService,
	notificationService *services.NotificationService,
	auditLogService *services.AuditLogService,
	checkinService *services.CheckinService,
	settingsService *services.SettingsService,
	backupService *services.BackupService,
	broadcastService *services.BroadcastService,
	healthService *services.HealthService,
) (*AdminHandler, error) {
	funcMap := buildAdminFuncMap()
	funcMap["setting"] = func(key string) string {
		return settingsService.Get(key)
	}
	funcMap["settingBool"] = func(key string) bool {
		return settingsService.GetBool(key)
	}

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
		"admin_notifications.html",
		"admin_audit_logs.html",
		"admin_participant_qr.html",
		"admin_backups.html",
		"admin_checkin.html",
		"admin_attendance.html",
		"admin_broadcasts.html",
		"admin_broadcast_create.html",
		"admin_broadcast_detail.html",
		"admin_system.html",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse admin templates: %w", err)
	}
	return &AdminHandler{
		participantService:  participantService,
		eventService:        eventService,
		store:               store,
		adminService:        adminService,
		storageService:      storageService,
		notificationService: notificationService,
		auditLogService:     auditLogService,
		checkinService:      checkinService,
		settingsService:     settingsService,
		backupService:       backupService,
		broadcastService:    broadcastService,
		healthService:       healthService,
		tmpl:                t,
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

// Helper to build a query string for participants list URL
func buildParticipantsURL(page int, status, search string) string {
	var params []string
	if page > 1 {
		params = append(params, fmt.Sprintf("page=%d", page))
	}
	if status != "" {
		params = append(params, "status="+url.QueryEscape(status))
	}
	if search != "" {
		params = append(params, "q="+url.QueryEscape(search))
	}

	if len(params) > 0 {
		return "/admin/participants?" + strings.Join(params, "&")
	}
	return "/admin/participants"
}

// Helper to build a query string for export URL
func buildExportURL(status, search string) string {
	var params []string
	if status != "" {
		params = append(params, "status="+url.QueryEscape(status))
	}
	if search != "" {
		params = append(params, "q="+url.QueryEscape(search))
	}

	if len(params) > 0 {
		return "/admin/participants/export?" + strings.Join(params, "&")
	}
	return "/admin/participants/export"
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
	var pageItems []ParticipantPage
	for i := 1; i <= totalPages; i++ {
		pages = append(pages, i)
		pageItems = append(pageItems, ParticipantPage{
			Num:       i,
			URL:       buildParticipantsURL(i, filter, search),
			IsCurrent: i == page,
		})
	}

	stats, _ := h.participantService.GetStats()
	event, _ := h.eventService.GetFirst()
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	return h.render(c, "admin_participants.html", AdminParticipantsData{
		AdminUser:      adminUser,
		AdminRole:      adminRole,
		Participants:   participants,
		Stats:          stats,
		Filter:         filter,
		Search:         search,
		Event:          event,
		Page:           page,
		TotalPages:     totalPages,
		TotalItems:     totalItems,
		HasPrev:        hasPrev,
		HasNext:        hasNext,
		PrevPage:       prevPage,
		NextPage:       nextPage,
		Pages:          pages,
		PageItems:      pageItems,
		ExportURL:      buildExportURL(filter, search),
		TabAllURL:      buildParticipantsURL(1, "", search),
		TabPendingURL:  buildParticipantsURL(1, "PENDING", search),
		TabVerifiedURL: buildParticipantsURL(1, "VERIFIED", search),
		TabRejectedURL: buildParticipantsURL(1, "REJECTED", search),
		PagePrevURL:    buildParticipantsURL(prevPage, filter, search),
		PageNextURL:    buildParticipantsURL(nextPage, filter, search),
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

	oldP, getErr := h.participantService.GetByID(uint(id))
	var oldPJSON string
	if getErr == nil && oldP != nil {
		if bytes, err := json.Marshal(oldP); err == nil {
			oldPJSON = string(bytes)
		}
	}

	p, err := h.participantService.UpdateStatus(uint(id), status)
	if err != nil {
		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusBadRequest).SendString(
				`<div class="text-red-400 text-sm font-semibold">Gagal memperbarui status.</div>`)
		}
		return c.Redirect(fmt.Sprintf("/admin/participants/%d", id), fiber.StatusSeeOther)
	}

	action := "UPDATE"
	if status == "VERIFIED" {
		action = "VERIFY"
	} else if status == "REJECTED" {
		action = "REJECT"
	}
	adminUser, _ := c.Locals("admin_username").(string)
	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, action, "PARTICIPANT", p.ID, oldPJSON, p, c.IP(), c.Get("User-Agent"))
	}

	// Trigger verified/rejected notification
	if err := h.notificationService.SendNotification(p, string(status)); err != nil {
		log.Printf("Warning: failed to send status update notification: %v", err)
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
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"4F46E5"}, Pattern: 1},
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
		{"A", "Registration Number", 20},
		{"B", "Full Name", 28},
		{"C", "WhatsApp", 18},
		{"D", "Email", 30},
		{"E", "City", 16},
		{"F", "Company Name", 28},
		{"G", "Industrial Estate", 24},
		{"H", "Telegram Username", 20},
		{"I", "Job Title", 20},
		{"J", "Status", 14},
		{"K", "Registration Date", 22},
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
			statusLabel = "VERIFIED"
		case models.StatusRejected:
			statusLabel = "REJECTED"
		case models.StatusPending:
			statusLabel = "PENDING"
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

	date := time.Now().Format("20060102")
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
	if title == "" {
		errs = append(errs, "Judul Acara wajib diisi")
	}
	if slug == "" {
		errs = append(errs, "Slug Acara wajib diisi")
	}
	if location == "" {
		errs = append(errs, "Lokasi wajib diisi")
	}
	if adminName == "" {
		errs = append(errs, "Nama Admin wajib diisi")
	}
	if adminWhatsapp == "" {
		errs = append(errs, "WhatsApp Admin wajib diisi")
	}

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

	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "CREATE", "EVENT", newEvent.ID, nil, newEvent, c.IP(), c.Get("User-Agent"))
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

	oldEventJSON := ""
	if bytes, err := json.Marshal(event); err == nil {
		oldEventJSON = string(bytes)
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
	if title == "" {
		errs = append(errs, "Judul Acara wajib diisi")
	}
	if slug == "" {
		errs = append(errs, "Slug Acara wajib diisi")
	}
	if location == "" {
		errs = append(errs, "Lokasi wajib diisi")
	}
	if adminName == "" {
		errs = append(errs, "Nama Admin wajib diisi")
	}
	if adminWhatsapp == "" {
		errs = append(errs, "WhatsApp Admin wajib diisi")
	}

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

	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "UPDATE", "EVENT", event.ID, oldEventJSON, event, c.IP(), c.Get("User-Agent"))
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

	event, getErr := h.eventService.GetByID(uint(id))
	var oldEventJSON string
	if getErr == nil && event != nil {
		if bytes, err := json.Marshal(event); err == nil {
			oldEventJSON = string(bytes)
		}
	}

	if err := h.eventService.Delete(uint(id)); err != nil {
		setFlash(c, "error", "Gagal menghapus acara: "+err.Error())
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/admin/events")
			return c.SendStatus(fiber.StatusOK)
		}
		return c.Redirect("/admin/events", fiber.StatusSeeOther)
	}

	adminUser, _ := c.Locals("admin_username").(string)
	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "DELETE", "EVENT", uint(id), oldEventJSON, nil, c.IP(), c.Get("User-Agent"))
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

	oldEventJSON := ""
	if bytes, err := json.Marshal(event); err == nil {
		oldEventJSON = string(bytes)
	}

	event.Status = status
	if err := h.eventService.Update(event); err != nil {
		setFlash(c, "error", "Gagal memperbarui status acara.")
		return c.Redirect(fmt.Sprintf("/admin/events/%d", id), fiber.StatusSeeOther)
	}

	adminUser, _ := c.Locals("admin_username").(string)
	action := "UPDATE"
	if status == "PUBLISHED" {
		action = "PUBLISH"
	} else if status == "CLOSED" {
		action = "CLOSE"
	}
	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, action, "EVENT", event.ID, oldEventJSON, event, c.IP(), c.Get("User-Agent"))
	}

	statusLabels := map[string]string{
		"PUBLISHED": "diterbitkan",
		"CLOSED":    "ditutup",
		"DRAFT":     "dikembalikan ke Draft",
	}
	setFlash(c, "success", "Acara \""+event.Title+"\" berhasil "+statusLabels[status]+".")
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
		eventTitle := "acara ini"
		if p.Event.Title != "" {
			eventTitle = p.Event.Title
		}
		actions = fmt.Sprintf(`
<button onclick="confirmAction('/admin/participants/%d/status', 'VERIFIED', 'Apakah Anda yakin ingin memverifikasi pendaftaran %s untuk %s?', 'VERIFIED')"
  class="flex-1 flex items-center justify-center gap-2 bg-emerald-600 hover:bg-emerald-500 text-white font-bold py-2.5 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7"/></svg>
  Verifikasi
</button>
<button onclick="confirmAction('/admin/participants/%d/status', 'REJECTED', 'Apakah Anda yakin ingin menolak pendaftaran %s untuk %s?', 'REJECTED')"
  class="flex-1 flex items-center justify-center gap-2 bg-red-700 hover:bg-red-600 text-white font-bold py-2.5 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
  Tolak
</button>`, p.ID, p.FullName, eventTitle, p.ID, p.FullName, eventTitle)
	case models.StatusVerified:
		actions = fmt.Sprintf(`
<a href="/admin/participants/%d/qr"
  class="flex-1 flex items-center justify-center gap-2 bg-indigo-600 hover:bg-indigo-500 text-white font-bold py-2.5 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h.01M16 12h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
  Tampilkan QR Code
</a>`, p.ID)
	case models.StatusRejected:
		actions = ""
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
	if name == "" {
		errs = append(errs, "Nama Lengkap wajib diisi")
	}
	if username == "" {
		errs = append(errs, "Username wajib diisi")
	}
	if email == "" {
		errs = append(errs, "Email wajib diisi")
	}
	if password == "" {
		errs = append(errs, "Password wajib diisi")
	}
	if role != "SUPER_ADMIN" && role != "ADMIN" {
		errs = append(errs, "Role tidak valid")
	}

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
	if name == "" {
		errs = append(errs, "Nama Lengkap wajib diisi")
	}
	if email == "" {
		errs = append(errs, "Email wajib diisi")
	}
	if role != "SUPER_ADMIN" && role != "ADMIN" {
		errs = append(errs, "Role tidak valid")
	}

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

	return h.render(c, "admin_settings.html", AdminSettingsData{
		AdminUser:           adminUser,
		AdminRole:           adminRole,
		SiteName:            h.settingsService.Get("site_name"),
		SiteDescription:     h.settingsService.Get("site_description"),
		FooterText:          h.settingsService.Get("footer_text"),
		RegistrationEnabled: h.settingsService.GetBool("registration_enabled"),
		MaintenanceMode:     h.settingsService.GetBool("maintenance_mode"),
		SupportName:         h.settingsService.Get("support_name"),
		SupportWhatsApp:     h.settingsService.Get("support_whatsapp"),
		SupportEmail:        h.settingsService.Get("support_email"),
		WhatsAppEnabled:     h.settingsService.GetBool("whatsapp_enabled"),
		NotificationEnabled: h.settingsService.GetBool("notification_enabled"),
		StoragePath:         h.settingsService.Get("storage_path"),
		Stats:               stats,
	})
}

// SystemSettingsSubmit handles POST /admin/settings
func (h *AdminHandler) SystemSettingsSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	// Parse inputs
	siteName := strings.TrimSpace(c.FormValue("site_name"))
	siteDescription := strings.TrimSpace(c.FormValue("site_description"))
	footerText := strings.TrimSpace(c.FormValue("footer_text"))
	registrationEnabled := c.FormValue("registration_enabled") == "true"
	maintenanceMode := c.FormValue("maintenance_mode") == "true"
	supportName := strings.TrimSpace(c.FormValue("support_name"))
	supportWhatsApp := strings.TrimSpace(c.FormValue("support_whatsapp"))
	supportEmail := strings.TrimSpace(c.FormValue("support_email"))
	whatsAppEnabled := c.FormValue("whatsapp_enabled") == "true"
	notificationEnabled := c.FormValue("notification_enabled") == "true"
	storagePath := strings.TrimSpace(c.FormValue("storage_path"))

	var errs []string
	if siteName == "" {
		errs = append(errs, "Nama Situs wajib diisi")
	}
	if footerText == "" {
		errs = append(errs, "Teks Footer wajib diisi")
	}
	if supportName == "" {
		errs = append(errs, "Nama Support wajib diisi")
	}
	if supportWhatsApp == "" {
		errs = append(errs, "WhatsApp Support wajib diisi")
	}
	if supportEmail == "" {
		errs = append(errs, "Email Support wajib diisi")
	}
	if storagePath == "" {
		errs = append(errs, "Path Penyimpanan wajib diisi")
	}

	if len(errs) > 0 {
		return h.render(c, "admin_settings.html", AdminSettingsData{
			AdminUser:           adminUser,
			AdminRole:           adminRole,
			SiteName:            siteName,
			SiteDescription:     siteDescription,
			FooterText:          footerText,
			RegistrationEnabled: registrationEnabled,
			MaintenanceMode:     maintenanceMode,
			SupportName:         supportName,
			SupportWhatsApp:     supportWhatsApp,
			SupportEmail:        supportEmail,
			WhatsAppEnabled:     whatsAppEnabled,
			NotificationEnabled: notificationEnabled,
			StoragePath:         storagePath,
			Errors:              errs,
			Stats:               stats,
		})
	}

	// Capture old values for audit logging
	oldSettings := map[string]string{
		"site_name":            h.settingsService.Get("site_name"),
		"site_description":     h.settingsService.Get("site_description"),
		"footer_text":          h.settingsService.Get("footer_text"),
		"registration_enabled": h.settingsService.Get("registration_enabled"),
		"maintenance_mode":     h.settingsService.Get("maintenance_mode"),
		"support_name":         h.settingsService.Get("support_name"),
		"support_whatsapp":     h.settingsService.Get("support_whatsapp"),
		"support_email":        h.settingsService.Get("support_email"),
		"whatsapp_enabled":     h.settingsService.Get("whatsapp_enabled"),
		"notification_enabled": h.settingsService.Get("notification_enabled"),
		"storage_path":         h.settingsService.Get("storage_path"),
	}

	newSettings := map[string]string{
		"site_name":            siteName,
		"site_description":     siteDescription,
		"footer_text":          footerText,
		"registration_enabled": strconv.FormatBool(registrationEnabled),
		"maintenance_mode":     strconv.FormatBool(maintenanceMode),
		"support_name":         supportName,
		"support_whatsapp":     supportWhatsApp,
		"support_email":        supportEmail,
		"whatsapp_enabled":     strconv.FormatBool(whatsAppEnabled),
		"notification_enabled": strconv.FormatBool(notificationEnabled),
		"storage_path":         storagePath,
	}

	if err := h.settingsService.UpdateMany(newSettings); err != nil {
		return h.render(c, "admin_settings.html", AdminSettingsData{
			AdminUser:           adminUser,
			AdminRole:           adminRole,
			SiteName:            siteName,
			SiteDescription:     siteDescription,
			FooterText:          footerText,
			RegistrationEnabled: registrationEnabled,
			MaintenanceMode:     maintenanceMode,
			SupportName:         supportName,
			SupportWhatsApp:     supportWhatsApp,
			SupportEmail:        supportEmail,
			WhatsAppEnabled:     whatsAppEnabled,
			NotificationEnabled: notificationEnabled,
			StoragePath:         storagePath,
			Errors:              []string{"Gagal menyimpan pengaturan: " + err.Error()},
			Stats:               stats,
		})
	}

	// Write audit log entry
	ip := c.IP()
	ua := c.Get("User-Agent")
	if err := h.auditLogService.Log(adminUser, "UPDATE", "Setting", 0, oldSettings, newSettings, ip, ua); err != nil {
		log.Printf("Warning: failed to log settings update: %v", err)
	}

	return h.render(c, "admin_settings.html", AdminSettingsData{
		AdminUser:           adminUser,
		AdminRole:           adminRole,
		SiteName:            siteName,
		SiteDescription:     siteDescription,
		FooterText:          footerText,
		RegistrationEnabled: registrationEnabled,
		MaintenanceMode:     maintenanceMode,
		SupportName:         supportName,
		SupportWhatsApp:     supportWhatsApp,
		SupportEmail:        supportEmail,
		WhatsAppEnabled:     whatsAppEnabled,
		NotificationEnabled: notificationEnabled,
		StoragePath:         storagePath,
		SuccessMessage:      "Pengaturan sistem berhasil disimpan!",
		Stats:               stats,
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
		"add":  func(a, b int) int { return a + b },
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

// NotificationList handles GET /admin/notifications
func (h *AdminHandler) NotificationList(c *fiber.Ctx) error {
	logs, err := h.notificationService.GetAllLogs()
	if err != nil {
		logs = []models.NotificationLog{}
	}

	stats, _ := h.participantService.GetStats()
	event, _ := h.eventService.GetFirst()
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	return h.render(c, "admin_notifications.html", AdminNotificationsData{
		AdminUser: adminUser,
		AdminRole: adminRole,
		Logs:      logs,
		Stats:     stats,
		Event:     event,
	})
}

type AdminAuditLogsData struct {
	AdminUser      string
	AdminRole      string
	Logs           []models.AuditLog
	Stats          *services.ParticipantStats
	Search         string
	FilterResource string
	FilterAction   string
	StartDate      string
	EndDate        string
}

// AuditLogList handles GET /admin/audit-logs
func (h *AdminHandler) AuditLogList(c *fiber.Ctx) error {
	search := strings.TrimSpace(c.Query("q"))
	resourceFilter := strings.TrimSpace(c.Query("resource"))
	actionFilter := strings.TrimSpace(c.Query("action"))
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))

	logs, err := h.auditLogService.QueryLogs(search, resourceFilter, actionFilter, startDate, endDate)
	if err != nil {
		logs = []models.AuditLog{}
	}

	stats, _ := h.participantService.GetStats()
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	return h.render(c, "admin_audit_logs.html", AdminAuditLogsData{
		AdminUser:      adminUser,
		AdminRole:      adminRole,
		Logs:           logs,
		Stats:          stats,
		Search:         search,
		FilterResource: resourceFilter,
		FilterAction:   actionFilter,
		StartDate:      startDate,
		EndDate:        endDate,
	})
}

type AdminParticipantQRData struct {
	AdminUser   string
	AdminRole   string
	Participant *models.Participant
	Event       *models.Event
	QRToken     string
	Stats       *services.ParticipantStats
}

// ParticipantQRPage handles GET /admin/participants/:id/qr
func (h *AdminHandler) ParticipantQRPage(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Redirect("/admin/participants", fiber.StatusSeeOther)
	}

	participant, err := h.participantService.GetByID(uint(id))
	if err != nil {
		return c.Redirect("/admin/participants", fiber.StatusSeeOther)
	}

	if participant.Status != models.StatusVerified {
		setFlash(c, "error", "Hanya peserta VERIFIED yang dapat memiliki QR Code.")
		return c.Redirect(fmt.Sprintf("/admin/participants/%d", id), fiber.StatusSeeOther)
	}

	token, err := h.checkinService.GenerateQRToken(participant.ID, participant.EventID, participant.RegistrationNumber)
	if err != nil {
		setFlash(c, "error", "Gagal membuat token QR: "+err.Error())
		return c.Redirect(fmt.Sprintf("/admin/participants/%d", id), fiber.StatusSeeOther)
	}

	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_participant_qr.html", AdminParticipantQRData{
		AdminUser:   adminUser,
		AdminRole:   adminRole,
		Participant: participant,
		Event:       &participant.Event,
		QRToken:     token,
		Stats:       stats,
	})
}

// BackupsPage handles GET /admin/backups
func (h *AdminHandler) BackupsPage(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	backups, err := h.backupService.ListBackups()
	if err != nil {
		setFlash(c, "error", "Gagal memuat daftar backup: "+err.Error())
		backups = []services.BackupInfo{}
	}

	return h.render(c, "admin_backups.html", AdminBackupsData{
		AdminUser:    adminUser,
		AdminRole:    adminRole,
		Backups:      backups,
		Stats:        stats,
		FlashSuccess: getFlash(c, "success"),
		FlashError:   getFlash(c, "error"),
	})
}

// CreateBackupSubmit handles POST /admin/backups/create
func (h *AdminHandler) CreateBackupSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	filename, err := h.backupService.CreateBackup()
	if err != nil {
		setFlash(c, "error", "Gagal membuat backup: "+err.Error())
		return c.Redirect("/admin/backups", fiber.StatusSeeOther)
	}

	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "CREATE", "BACKUP", 0, nil, map[string]string{"filename": filename}, c.IP(), c.Get("User-Agent"))
	}

	setFlash(c, "success", "Backup \""+filename+"\" berhasil dibuat.")
	return c.Redirect("/admin/backups", fiber.StatusSeeOther)
}

// DownloadBackup handles GET /admin/backups/download/:filename
func (h *AdminHandler) DownloadBackup(c *fiber.Ctx) error {
	filename := c.Params("filename")
	cleaned := filepath.Base(filename)
	filePath := filepath.Join(h.storageService.GetRootPath(), "backups", cleaned)

	if _, err := os.Stat(filePath); err != nil {
		setFlash(c, "error", "File backup tidak ditemukan.")
		return c.Redirect("/admin/backups", fiber.StatusSeeOther)
	}

	return c.Download(filePath, cleaned)
}

// RestoreBackupSubmit handles POST /admin/backups/restore/:filename
func (h *AdminHandler) RestoreBackupSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	filename := c.Params("filename")
	cleaned := filepath.Base(filename)

	err := h.backupService.RestoreBackup(cleaned)
	if err != nil {
		setFlash(c, "error", "Gagal memulihkan backup: "+err.Error())
		return c.Redirect("/admin/backups", fiber.StatusSeeOther)
	}

	// Reload settings cache
	if err := h.settingsService.Load(); err != nil {
		log.Printf("Warning: failed to reload settings cache after restore: %v", err)
	}

	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "RESTORE", "BACKUP", 0, nil, map[string]string{"filename": cleaned}, c.IP(), c.Get("User-Agent"))
	}

	setFlash(c, "success", "Backup \""+cleaned+"\" berhasil dipulihkan.")
	return c.Redirect("/admin/backups", fiber.StatusSeeOther)
}

// UploadRestoreBackup handles POST /admin/backups/upload
func (h *AdminHandler) UploadRestoreBackup(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)

	file, err := c.FormFile("backup_file")
	if err != nil {
		setFlash(c, "error", "Gagal mengunggah file: "+err.Error())
		return c.Redirect("/admin/backups", fiber.StatusSeeOther)
	}

	if !strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		setFlash(c, "error", "Format file tidak valid. Harus berupa file .zip.")
		return c.Redirect("/admin/backups", fiber.StatusSeeOther)
	}

	backupsDir := filepath.Join(h.storageService.GetRootPath(), "backups")
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		setFlash(c, "error", "Gagal menyiapkan folder backup: "+err.Error())
		return c.Redirect("/admin/backups", fiber.StatusSeeOther)
	}

	cleanedName := filepath.Base(file.Filename)
	destPath := filepath.Join(backupsDir, cleanedName)

	if err := c.SaveFile(file, destPath); err != nil {
		setFlash(c, "error", "Gagal menyimpan file backup: "+err.Error())
		return c.Redirect("/admin/backups", fiber.StatusSeeOther)
	}

	err = h.backupService.RestoreBackup(cleanedName)
	if err != nil {
		setFlash(c, "error", "Gagal memulihkan backup yang diunggah: "+err.Error())
		return c.Redirect("/admin/backups", fiber.StatusSeeOther)
	}

	// Reload settings cache
	if err := h.settingsService.Load(); err != nil {
		log.Printf("Warning: failed to reload settings cache after restore: %v", err)
	}

	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "RESTORE", "BACKUP", 0, nil, map[string]string{"filename": cleanedName, "uploaded": "true"}, c.IP(), c.Get("User-Agent"))
	}

	setFlash(c, "success", "Backup \""+cleanedName+"\" berhasil diunggah dan dipulihkan.")
	return c.Redirect("/admin/backups", fiber.StatusSeeOther)
}

// DeleteBackupSubmit handles POST /admin/backups/delete/:filename
func (h *AdminHandler) DeleteBackupSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	filename := c.Params("filename")
	cleaned := filepath.Base(filename)

	err := h.backupService.DeleteBackup(cleaned)
	if err != nil {
		setFlash(c, "error", "Gagal menghapus backup: "+err.Error())
		return c.Redirect("/admin/backups", fiber.StatusSeeOther)
	}

	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "DELETE", "BACKUP", 0, nil, map[string]string{"filename": cleaned}, c.IP(), c.Get("User-Agent"))
	}

	setFlash(c, "success", "Backup \""+cleaned+"\" berhasil dihapus.")
	return c.Redirect("/admin/backups", fiber.StatusSeeOther)
}

// CheckinPage handles GET /admin/checkin
func (h *AdminHandler) CheckinPage(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	search := strings.TrimSpace(c.Query("q"))

	var participants []models.Participant
	var err error

	participants, err = h.participantService.GetAllForAdmin("", search)
	if err != nil {
		participants = []models.Participant{}
	}

	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_checkin.html", AdminCheckinData{
		AdminUser:    adminUser,
		AdminRole:    adminRole,
		Participants: participants,
		Stats:        stats,
		Search:       search,
		FlashSuccess: getFlash(c, "success"),
		FlashError:   getFlash(c, "error"),
	})
}

// CheckinSubmit handles POST /admin/checkin/:participant_id
func (h *AdminHandler) CheckinSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)

	id, err := c.ParamsInt("participant_id")
	if err != nil || id <= 0 {
		setFlash(c, "error", "ID peserta tidak valid.")
		return c.Redirect("/admin/checkin", fiber.StatusSeeOther)
	}

	p, err := h.participantService.GetByID(uint(id))
	if err != nil {
		setFlash(c, "error", "Peserta tidak ditemukan.")
		return c.Redirect("/admin/checkin", fiber.StatusSeeOther)
	}

	_, err = h.checkinService.Checkin(p.ID, p.EventID, adminUser)
	if err != nil {
		setFlash(c, "error", err.Error())
		return c.Redirect("/admin/checkin", fiber.StatusSeeOther)
	}

	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "CHECKIN", "PARTICIPANT", p.ID, nil, map[string]string{"method": "manual"}, c.IP(), c.Get("User-Agent"))
	}

	setFlash(c, "success", "Peserta \""+p.FullName+"\" berhasil di-check in.")
	return c.Redirect("/admin/checkin", fiber.StatusSeeOther)
}

// AdminAttendanceData contains all the data needed for the attendance dashboard template.
type AdminAttendanceData struct {
	AdminUser       string
	AdminRole       string
	Events          []models.Event
	Stats           *services.ParticipantStats
	Checkins        []models.Attendance
	SelectedEventID uint
	SelectedDate    string
	Search          string
}

// AttendanceDashboard handles GET /admin/attendance
func (h *AdminHandler) AttendanceDashboard(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	var eventID uint
	if eventIDStr := c.Query("event_id"); eventIDStr != "" {
		if val, err := strconv.Atoi(eventIDStr); err == nil && val > 0 {
			eventID = uint(val)
		}
	}

	date := strings.TrimSpace(c.Query("date"))
	search := strings.TrimSpace(c.Query("q"))

	// Fetch active events list for the dropdown filter
	events, err := h.eventService.GetAll()
	if err != nil {
		events = []models.Event{}
	}

	// Fetch attendance stats with filters
	stats, err := h.participantService.GetAttendanceStats(eventID, date)
	if err != nil {
		stats = &services.ParticipantStats{}
	}

	// Fetch latest check-ins
	checkins, err := h.checkinService.GetLatestCheckins(eventID, date, search, 50)
	if err != nil {
		checkins = []models.Attendance{}
	}

	data := AdminAttendanceData{
		AdminUser:       adminUser,
		AdminRole:       adminRole,
		Events:          events,
		Stats:           stats,
		Checkins:        checkins,
		SelectedEventID: eventID,
		SelectedDate:    date,
		Search:          search,
	}

	// If HTMX request, render only the partial fragment template block
	if c.Get("HX-Request") == "true" {
		return h.render(c, "attendance_fragment", data)
	}

	return h.render(c, "admin_attendance.html", data)
}

// BroadcastList handles GET /admin/broadcasts
func (h *AdminHandler) BroadcastList(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	broadcasts, err := h.broadcastService.GetAllBroadcasts()
	if err != nil {
		broadcasts = []models.Broadcast{}
	}

	data := fiber.Map{
		"AdminUser":  adminUser,
		"AdminRole":  adminRole,
		"Broadcasts": broadcasts,
	}
	return h.render(c, "admin_broadcasts.html", data)
}

// BroadcastCreatePage handles GET /admin/broadcasts/create
func (h *AdminHandler) BroadcastCreatePage(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	events, err := h.eventService.GetAll()
	if err != nil {
		events = []models.Event{}
	}

	data := fiber.Map{
		"AdminUser": adminUser,
		"AdminRole": adminRole,
		"Events":    events,
	}
	return h.render(c, "admin_broadcast_create.html", data)
}

// BroadcastCountRecipients handles GET /admin/broadcasts/count-recipients
func (h *AdminHandler) BroadcastCountRecipients(c *fiber.Ctx) error {
	var eventID uint
	if idStr := c.Query("event_id"); idStr != "" {
		if val, err := strconv.Atoi(idStr); err == nil {
			eventID = uint(val)
		}
	}
	group := c.Query("group")
	city := c.Query("city")
	estate := c.Query("industrial_estate")
	company := c.Query("company_name")

	count, err := h.broadcastService.GetRecipientsCount(eventID, group, city, estate, company)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error")
	}

	htmlBadge := fmt.Sprintf(`<span class="bg-indigo-500/20 text-indigo-300 px-3 py-1 rounded-lg border border-indigo-500/30 font-bold" id="recipients-badge">Estimasi: %d Peserta</span>`, count)
	return c.Type("html").SendString(htmlBadge)
}

// BroadcastPreview handles POST /admin/broadcasts/preview
func (h *AdminHandler) BroadcastPreview(c *fiber.Ctx) error {
	var eventID uint
	if idStr := c.FormValue("event_id"); idStr != "" {
		if val, err := strconv.Atoi(idStr); err == nil {
			eventID = uint(val)
		}
	}
	group := c.FormValue("group")
	city := c.FormValue("city")
	estate := c.FormValue("industrial_estate")
	company := c.FormValue("company_name")
	title := c.FormValue("title")
	body := c.FormValue("message")

	participants, err := h.broadcastService.GetRecipients(eventID, group, city, estate, company)
	var samplePart models.Participant
	hasMatches := false

	if err == nil && len(participants) > 0 {
		samplePart = participants[0]
		hasMatches = true
	} else {
		// Fallback dummy participant for preview purposes
		var dummyEvent models.Event
		if eventID > 0 {
			if e, err := h.eventService.GetByID(eventID); err == nil {
				dummyEvent = *e
			}
		}
		if dummyEvent.ID == 0 {
			dummyEvent = models.Event{Title: "Seminar & Gathering Nasional"}
		}
		samplePart = models.Participant{
			FullName:           "Budi Santoso",
			RegistrationNumber: "GH-2026-9999",
			Event:              dummyEvent,
		}
	}

	renderedTitle := title
	if renderedTitle == "" {
		renderedTitle = "(Judul Kosong)"
	}
	renderedBody := h.broadcastService.RenderTemplate(body, &samplePart)
	if renderedBody == "" {
		renderedBody = "(Pesan Kosong)"
	}

	data := fiber.Map{
		"HasMatches":   hasMatches,
		"TotalMatches": len(participants),
		"PreviewTitle": renderedTitle,
		"PreviewBody":  renderedBody,
		"SampleName":   samplePart.FullName,
		"SampleRegNum": samplePart.RegistrationNumber,
		"SampleEvent":  samplePart.Event.Title,
	}

	return h.render(c, "broadcast_preview_fragment", data)
}

// BroadcastCreateSubmit handles POST /admin/broadcasts/create
func (h *AdminHandler) BroadcastCreateSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)

	var eventID uint
	if idStr := c.FormValue("event_id"); idStr != "" {
		if val, err := strconv.Atoi(idStr); err == nil {
			eventID = uint(val)
		}
	}
	if eventID == 0 {
		setFlash(c, "error", "Pilih acara terlebih dahulu.")
		return c.Redirect("/admin/broadcasts/create")
	}

	title := strings.TrimSpace(c.FormValue("title"))
	message := strings.TrimSpace(c.FormValue("message"))
	group := c.FormValue("group")
	city := strings.TrimSpace(c.FormValue("city"))
	estate := strings.TrimSpace(c.FormValue("industrial_estate"))
	company := strings.TrimSpace(c.FormValue("company_name"))

	if title == "" || message == "" {
		setFlash(c, "error", "Judul dan Isi Pesan harus diisi.")
		return c.Redirect("/admin/broadcasts/create")
	}

	broadcast, err := h.broadcastService.CreateBroadcast(eventID, title, message, group, city, estate, company)
	if err != nil {
		setFlash(c, "error", fmt.Sprintf("Gagal mengirim broadcast: %v", err))
		return c.Redirect("/admin/broadcasts/create")
	}

	// Write audit log entry
	oldVal := map[string]interface{}{}
	newVal := map[string]interface{}{
		"broadcast_id":     broadcast.ID,
		"title":            broadcast.Title,
		"event_id":         eventID,
		"audience":         broadcast.Audience,
		"total_recipients": broadcast.TotalRecipients,
	}
	_ = h.auditLogService.Log(adminUser, "CREATE", "BROADCAST", broadcast.ID, oldVal, newVal, c.IP(), c.Get("User-Agent"))

	setFlash(c, "success", fmt.Sprintf("Broadcast '%s' berhasil dikirim ke %d peserta.", broadcast.Title, broadcast.TotalRecipients))
	return c.Redirect("/admin/broadcasts")
}

// BroadcastDetail handles GET /admin/broadcasts/:id
func (h *AdminHandler) BroadcastDetail(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	idStr := c.Params("id")
	val, err := strconv.Atoi(idStr)
	if err != nil || val <= 0 {
		setFlash(c, "error", "ID Broadcast tidak valid.")
		return c.Redirect("/admin/broadcasts")
	}

	broadcast, err := h.broadcastService.GetBroadcastByID(uint(val))
	if err != nil {
		setFlash(c, "error", "Broadcast tidak ditemukan.")
		return c.Redirect("/admin/broadcasts")
	}

	logs, err := h.broadcastService.GetBroadcastLogs(broadcast.ID)
	if err != nil {
		logs = []models.NotificationLog{}
	}

	data := fiber.Map{
		"AdminUser": adminUser,
		"AdminRole": adminRole,
		"Broadcast": broadcast,
		"Logs":      logs,
	}
	return h.render(c, "admin_broadcast_detail.html", data)
}

// ─────────────────────── System Health ───────────────────────

// SystemHealth handles GET /admin/system
func (h *AdminHandler) SystemHealth(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	// Refresh storage path from live settings before running the check
	storagePath := h.settingsService.Get("storage_path")
	h.healthService.UpdateStoragePath(storagePath)

	report := h.healthService.Report()

	stats, err := h.participantService.GetStats()
	if err != nil {
		stats = &services.ParticipantStats{}
	}

	return h.render(c, "admin_system.html", AdminSystemHealthData{
		AdminUser: adminUser,
		AdminRole: adminRole,
		Health:    &report,
		Stats:     stats,
	})
}
