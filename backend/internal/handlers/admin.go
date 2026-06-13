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

// AdminBase is embedded in every admin page data struct so that the
// shared admin_sidebar.html partial always receives the fields it needs.
type AdminBase struct {
	AdminUser  string
	AdminRole  string
	ActiveMenu string // e.g. "dashboard", "participants", "broadcasts", …
	Stats      *services.ParticipantStats
}

type AdminLoginData struct {
	Error string
}

type AdminDashboardData struct {
	AdminBase
	RecentRegistrations []models.Participant
	RecentVerifications []models.Participant
	Events              []models.Event
	SelectedEventID     uint
	SelectedEvent       *models.Event
	StartDate           string
	EndDate             string
	AnalyticsJSON       string
	SponsorCount        int
	OpenTasksCount      int
	OverdueTasksCount   int
	CompletedTasksCount int
	TotalTasksCount     int
	InProgressTasksCount int
	ProgressPercentage  int
	TasksJSON           string
}

type AdminTasksData struct {
	AdminBase
	Tasks           []models.Task
	Events          []models.Event
	SelectedEventID uint
	SelectedEvent   *models.Event
	CategoryFilter  string
	PriorityFilter  string
	StatusFilter    string
	TotalTasksCount      int
	CompletedTasksCount  int
	InProgressTasksCount int
	OverdueTasksCount    int
	FlashSuccess    string
	FlashError      string
}

type AdminTaskCreateData struct {
	AdminBase
	Events       []models.Event
	Errors       []string
	Form         map[string]string
	FlashSuccess string
	FlashError   string
}

type AdminTaskEditData struct {
	AdminBase
	Task         *models.Task
	Events       []models.Event
	Errors       []string
	Form         map[string]string
	FlashSuccess string
	FlashError   string
}

type AdminSponsorsData struct {
	AdminBase
	Sponsors     []models.Sponsor
	FlashSuccess string
	FlashError   string
}

type AdminSponsorCreateData struct {
	AdminBase
	Events       []models.Event
	Errors       []string
	Form         map[string]string
	FlashSuccess string
	FlashError   string
}

type AdminSponsorEditData struct {
	AdminBase
	Sponsor      *models.Sponsor
	Events       []models.Event
	Errors       []string
	Form         map[string]string
	FlashSuccess string
	FlashError   string
}

type ParticipantPage struct {
	Num       int
	URL       string
	IsCurrent bool
}

type AdminNotificationsData struct {
	AdminBase
	Logs  []models.NotificationLog
	Event *models.Event
}

type AdminParticipantsData struct {
	AdminBase
	Participants   []models.Participant
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
	TabRegisteredURL string
	TabWaitlistURL   string
	TabVerifiedURL string
	TabRejectedURL string
	PagePrevURL    string
	PageNextURL    string
}

type AdminParticipantDetailData struct {
	AdminBase
	Participant *models.Participant
	Event       *models.Event
	PaymentURL  string
}

type AdminEventsData struct {
	AdminBase
	Events       []models.Event
	FlashSuccess string
	FlashError   string
}

type AdminEventCreateData struct {
	AdminBase
	Errors []string
	Form   map[string]string
}

type AdminEventEditData struct {
	AdminBase
	Event  *models.Event
	Errors []string
	Form   map[string]string
}

type AdminEventDetailData struct {
	AdminBase
	Event        *models.Event
	FlashSuccess string
	FlashError   string
}

type AdminAdminsData struct {
	AdminBase
	Admins []models.Admin
}

type AdminAdminCreateData struct {
	AdminBase
	Errors []string
	Form   map[string]string
}

type AdminAdminEditData struct {
	AdminBase
	Admin  *models.Admin
	Errors []string
	Form   map[string]string
}

type AdminSettingsData struct {
	AdminBase
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
}

type AdminBackupsData struct {
	AdminBase
	Backups      []services.BackupInfo
	FlashSuccess string
	FlashError   string
}

type AdminCheckinData struct {
	AdminBase
	Participants []models.Participant
	Search       string
	FlashSuccess string
	FlashError   string
}

type AdminSystemHealthData struct {
	AdminBase
	Health *services.HealthReport
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
	sponsorService      *services.SponsorService
	taskService         *services.TaskService
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
	sponsorService *services.SponsorService,
	taskService *services.TaskService,
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
		"admin_transportation.html",
		"admin_sponsors.html",
		"admin_sponsor_create.html",
		"admin_sponsor_edit.html",
		"admin_tasks.html",
		"admin_task_create.html",
		"admin_task_edit.html",
		"admin_sidebar.html",
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
		sponsorService:      sponsorService,
		taskService:         taskService,
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

	sponsorCount := 0
	openTasksCount := 0
	overdueTasksCount := 0
	completedTasksCount := 0
	totalTasksCount := 0
	inProgressTasksCount := 0
	progressPercentage := 0
	tasksJSON := "[]"

	if selectedEventID > 0 {
		if count, err := h.sponsorService.GetCountByEvent(selectedEventID); err == nil {
			sponsorCount = int(count)
		}
		if total, completed, inProg, overdue, err := h.taskService.GetTaskStatsDetailed(selectedEventID); err == nil {
			totalTasksCount = total
			completedTasksCount = completed
			inProgressTasksCount = inProg
			overdueTasksCount = overdue
			openTasksCount = total - completed
			if total > 0 {
				progressPercentage = (completed * 100) / total
			}
		}
		if tasks, err := h.taskService.GetTasksByEvent(selectedEventID, "", "", ""); err == nil && len(tasks) > 0 {
			type calendarTask struct {
				ID         uint   `json:"id"`
				Title      string `json:"title"`
				DueDate    string `json:"due_date"`
				Status     string `json:"status"`
				Priority   string `json:"priority"`
				Category   string `json:"category"`
				AssignedTo string `json:"assigned_to"`
			}
			calTasks := make([]calendarTask, len(tasks))
			for i, t := range tasks {
				calTasks[i] = calendarTask{
					ID:         t.ID,
					Title:      t.Title,
					DueDate:    t.DueDate.Format("2006-01-02"),
					Status:     t.Status,
					Priority:   t.Priority,
					Category:   t.Category,
					AssignedTo: t.AssignedTo,
				}
			}
			if bytes, err := json.Marshal(calTasks); err == nil {
				tasksJSON = string(bytes)
			}
		}
	}

	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	return h.render(c, "admin_dashboard.html", AdminDashboardData{
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "dashboard",
			Stats:      stats,
		},
		RecentRegistrations: recentRegs,
		RecentVerifications: recentVerifs,
		Events:              events,
		SelectedEventID:     selectedEventID,
		SelectedEvent:       selectedEvent,
		StartDate:           startDate,
		EndDate:             endDate,
		AnalyticsJSON:       analyticsJSON,
		SponsorCount:        sponsorCount,
		OpenTasksCount:      openTasksCount,
		OverdueTasksCount:   overdueTasksCount,
		CompletedTasksCount: completedTasksCount,
		TotalTasksCount:     totalTasksCount,
		InProgressTasksCount: inProgressTasksCount,
		ProgressPercentage:  progressPercentage,
		TasksJSON:           tasksJSON,
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "participants",
			Stats:          stats,
		},
		Participants:   participants,
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
		TabPendingURL:  buildParticipantsURL(1, "REGISTERED", search),
		TabRegisteredURL: buildParticipantsURL(1, "REGISTERED", search),
		TabWaitlistURL:   buildParticipantsURL(1, "WAITLIST", search),
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "participants",
			Stats:       stats,
		},
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

	// Determine event config and slug for filename
	event, err := h.eventService.GetFirst()
	if err != nil || event == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Acara tidak ditemukan")
	}
	eventSlug := event.Slug

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

	// Helper to convert column index (1-based) to Excel letter (e.g. 1 -> A, 27 -> AA)
	colNumToName := func(col int) string {
		name := ""
		for col > 0 {
			col--
			name = string(rune('A'+col%26)) + name
			col /= 26
		}
		return name
	}

	type colDef struct {
		label    string
		width    float64
		getValue func(p *models.Participant) interface{}
	}

	var cols []colDef

	// Base fields that are always exported
	cols = append(cols, colDef{"Registration Number", 20, func(p *models.Participant) interface{} { return p.RegistrationNumber }})
	cols = append(cols, colDef{"Full Name", 28, func(p *models.Participant) interface{} { return p.FullName }})
	cols = append(cols, colDef{"WhatsApp", 18, func(p *models.Participant) interface{} { return p.Phone }})
	cols = append(cols, colDef{"Email", 30, func(p *models.Participant) interface{} { return p.Email }})
	cols = append(cols, colDef{"City", 16, func(p *models.Participant) interface{} { return p.City }})
	cols = append(cols, colDef{"Company Name", 28, func(p *models.Participant) interface{} { return p.CompanyName }})

	// Configurable fields
	if event.EnableIndustrialEstate {
		cols = append(cols, colDef{"Industrial Estate", 24, func(p *models.Participant) interface{} { return p.IndustrialEstate }})
		cols = append(cols, colDef{"Industrial Estate Name", 24, func(p *models.Participant) interface{} { return p.IndustrialEstateName }})
	}

	if event.EnableTelegram {
		cols = append(cols, colDef{"Telegram Username", 20, func(p *models.Participant) interface{} { return p.TelegramUsername }})
	}

	if event.EnableJobTitle {
		cols = append(cols, colDef{"Job Title", 20, func(p *models.Participant) interface{} {
			if p.JobTitle != nil {
				return *p.JobTitle
			}
			return ""
		}})
	}

	if event.EnableEmergencyContact {
		cols = append(cols, colDef{"Emergency Contact Name", 24, func(p *models.Participant) interface{} { return p.EmergencyName }})
		cols = append(cols, colDef{"Emergency Relationship", 20, func(p *models.Participant) interface{} { return p.EmergencyRelationship }})
		cols = append(cols, colDef{"Emergency Phone", 18, func(p *models.Participant) interface{} { return p.EmergencyPhone }})
	}

	if event.EnableVehicleInfo {
		cols = append(cols, colDef{"Own Vehicle", 14, func(p *models.Participant) interface{} {
			if p.OwnVehicle {
				return "Ya"
			}
			return "Tidak"
		}})
		cols = append(cols, colDef{"Vehicle Type", 16, func(p *models.Participant) interface{} { return p.VehicleType }})
		cols = append(cols, colDef{"License Plate", 16, func(p *models.Participant) interface{} { return p.LicensePlate }})
	}

	if event.EnableCarpool {
		cols = append(cols, colDef{"Can Bring Others", 18, func(p *models.Participant) interface{} {
			if p.CarpoolCanBring {
				return "Ya"
			}
			return "Tidak"
		}})
		cols = append(cols, colDef{"Available Seats", 16, func(p *models.Participant) interface{} { return p.CarpoolSeats }})
	}

	if event.EnableTShirtSize {
		cols = append(cols, colDef{"T-Shirt Size", 14, func(p *models.Participant) interface{} { return p.TShirtSize }})
	}

	if event.EnableTransportationCoordination {
		cols = append(cols, colDef{"Transportation Status", 24, func(p *models.Participant) interface{} { return p.GetTransportationStatus() }})
		cols = append(cols, colDef{"Official Driver", 18, func(p *models.Participant) interface{} {
			if p.OfficialDriver {
				return "Yes"
			}
			return "No"
		}})
		cols = append(cols, colDef{"Meeting Point", 24, func(p *models.Participant) interface{} {
			if p.OwnVehicle && p.VehicleType == "Car" && p.CarpoolCanBring {
				return p.TransportMeetingPoint
			}
			if p.DriverID != nil && p.Driver != nil {
				return p.Driver.TransportMeetingPoint
			}
			return "–"
		}})
		cols = append(cols, colDef{"Departure Notes", 30, func(p *models.Participant) interface{} {
			if p.OwnVehicle && p.VehicleType == "Car" && p.CarpoolCanBring {
				return p.TransportNotes
			}
			if p.DriverID != nil && p.Driver != nil {
				return p.Driver.TransportNotes
			}
			return "–"
		}})
	}

	// Status and Registration Date
	cols = append(cols, colDef{"Status", 14, func(p *models.Participant) interface{} {
		statusLabel := string(p.Status)
		switch p.Status {
		case models.StatusVerified:
			statusLabel = "VERIFIED"
		case models.StatusRejected:
			statusLabel = "REJECTED"
		case models.StatusPending:
			statusLabel = "PENDING"
		}
		return statusLabel
	}})
	cols = append(cols, colDef{"Registration Status", 20, func(p *models.Participant) interface{} {
		if p.Status == models.StatusWaitlist {
			return "Waitlist"
		}
		return "Registered"
	}})
	cols = append(cols, colDef{"Registration Date", 22, func(p *models.Participant) interface{} {
		return p.CreatedAt.Format("02/01/2006 15:04")
	}})

	// ── Set Headers ───────────────────────────────────────────
	for ci, col := range cols {
		colLetter := colNumToName(ci + 1)
		cell := colLetter + "1"
		f.SetCellValue(sheet, cell, col.label)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
		f.SetColWidth(sheet, colLetter, colLetter, col.width)
	}
	f.SetRowHeight(sheet, 1, 22)

	// ── Set Rows ──────────────────────────────────────────────
	for ri, p := range participants {
		row := ri + 2
		rowStr := strconv.Itoa(row)

		for ci, col := range cols {
			colLetter := colNumToName(ci + 1)
			cell := colLetter + rowStr
			f.SetCellValue(sheet, cell, col.getValue(&p))
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
	lastColLetter := colNumToName(len(cols))
	f.AutoFilter(sheet, "A1:"+lastColLetter+"1", []excelize.AutoFilterOptions{})

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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "events",
			Stats:        stats,
		},
		Events:       events,
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "events",
			Stats:      stats,
		},
		Form:      map[string]string{"enable_waiting_list": "true"},
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
		"title":                    title,
		"slug":                     slug,
		"description":              description,
		"event_date":               eventDateStr,
		"event_time":               eventTime,
		"location":                 location,
		"price":                    priceStr,
		"payment_bank":             paymentBank,
		"payment_account_number":   paymentAccountNumber,
		"payment_account_name":     paymentAccountName,
		"admin_name":               adminName,
		"admin_whatsapp":           adminWhatsapp,
		"max_participants":         maxParticipantsStr,
		"registration_open":        regOpenStr,
		"registration_close":       regCloseStr,
		"status":                   status,
		"enable_telegram":          c.FormValue("enable_telegram"),
		"enable_job_title":         c.FormValue("enable_job_title"),
		"enable_industrial_estate": c.FormValue("enable_industrial_estate"),
		"enable_emergency_contact": c.FormValue("enable_emergency_contact"),
		"enable_vehicle_info":      c.FormValue("enable_vehicle_info"),
		"enable_carpool":           c.FormValue("enable_carpool"),
		"enable_tshirt_size":       c.FormValue("enable_tshirt_size"),
		"enable_transportation_coordination": c.FormValue("enable_transportation_coordination"),
		"enable_waiting_list":      c.FormValue("enable_waiting_list"),
		"enable_sponsors":          c.FormValue("enable_sponsors"),
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
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "events",
				Stats:      stats,
			},
			Errors: errs,
			Form:   formValues,
		})
	}

	newEvent := &models.Event{
		Title:                  title,
		Slug:                   slug,
		Description:            description,
		BannerImage:            bannerFilename,
		EventDate:              eventDate,
		EventTime:              eventTime,
		Location:               location,
		Price:                  price,
		PaymentBank:            paymentBank,
		PaymentAccountNumber:   paymentAccountNumber,
		PaymentAccountName:     paymentAccountName,
		AdminName:              adminName,
		AdminWhatsapp:          adminWhatsapp,
		MaxParticipants:        maxParticipants,
		RegistrationOpen:       regOpen,
		RegistrationClose:      regClose,
		Status:                 status,
		EnableTelegram:         c.FormValue("enable_telegram") == "true",
		EnableJobTitle:         c.FormValue("enable_job_title") == "true",
		EnableIndustrialEstate: c.FormValue("enable_industrial_estate") == "true",
		EnableEmergencyContact: c.FormValue("enable_emergency_contact") == "true",
		EnableVehicleInfo:      c.FormValue("enable_vehicle_info") == "true",
		EnableCarpool:          c.FormValue("enable_carpool") == "true",
		EnableTShirtSize:       c.FormValue("enable_tshirt_size") == "true",
		EnableTransportationCoordination: c.FormValue("enable_transportation_coordination") == "true",
		EnableWaitingList:      c.FormValue("enable_waiting_list") == "true",
		EnableSponsors:         c.FormValue("enable_sponsors") == "true",
	}

	if err := h.eventService.Create(newEvent); err != nil {
		errs = append(errs, "Gagal menyimpan Acara: "+err.Error())
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_event_create.html", AdminEventCreateData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "events",
				Stats:     stats,
			},
			Errors:    errs,
			Form:      formValues,
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
		"title":                    event.Title,
		"slug":                     event.Slug,
		"description":              event.Description,
		"event_date":               event.EventDate.Format("2006-01-02"),
		"event_time":               event.EventTime,
		"location":                 event.Location,
		"price":                    fmt.Sprintf("%.0f", event.Price),
		"payment_bank":             event.PaymentBank,
		"payment_account_number":   event.PaymentAccountNumber,
		"payment_account_name":     event.PaymentAccountName,
		"admin_name":               event.AdminName,
		"admin_whatsapp":           event.AdminWhatsapp,
		"max_participants":         strconv.Itoa(event.MaxParticipants),
		"status":                   event.Status,
		"enable_telegram":          strconv.FormatBool(event.EnableTelegram),
		"enable_job_title":         strconv.FormatBool(event.EnableJobTitle),
		"enable_industrial_estate": strconv.FormatBool(event.EnableIndustrialEstate),
		"enable_emergency_contact": strconv.FormatBool(event.EnableEmergencyContact),
		"enable_vehicle_info":      strconv.FormatBool(event.EnableVehicleInfo),
		"enable_carpool":           strconv.FormatBool(event.EnableCarpool),
		"enable_tshirt_size":       strconv.FormatBool(event.EnableTShirtSize),
		"enable_transportation_coordination": strconv.FormatBool(event.EnableTransportationCoordination),
		"enable_waiting_list":      strconv.FormatBool(event.EnableWaitingList),
	}
	if !event.RegistrationOpen.IsZero() {
		formValues["registration_open"] = event.RegistrationOpen.Format("2006-01-02T15:04")
	}
	if !event.RegistrationClose.IsZero() {
		formValues["registration_close"] = event.RegistrationClose.Format("2006-01-02T15:04")
	}

	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_event_edit.html", AdminEventEditData{
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "events",
			Stats:     stats,
		},
		Event:     event,
		Form:      formValues,
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
		"title":                    title,
		"slug":                     slug,
		"description":              description,
		"event_date":               eventDateStr,
		"event_time":               eventTime,
		"location":                 location,
		"price":                    priceStr,
		"payment_bank":             paymentBank,
		"payment_account_number":   paymentAccountNumber,
		"payment_account_name":     paymentAccountName,
		"admin_name":               adminName,
		"admin_whatsapp":           adminWhatsapp,
		"max_participants":         maxParticipantsStr,
		"registration_open":        regOpenStr,
		"registration_close":       regCloseStr,
		"status":                   status,
		"enable_telegram":          c.FormValue("enable_telegram"),
		"enable_job_title":         c.FormValue("enable_job_title"),
		"enable_industrial_estate": c.FormValue("enable_industrial_estate"),
		"enable_emergency_contact": c.FormValue("enable_emergency_contact"),
		"enable_vehicle_info":      c.FormValue("enable_vehicle_info"),
		"enable_carpool":           c.FormValue("enable_carpool"),
		"enable_tshirt_size":       c.FormValue("enable_tshirt_size"),
		"enable_transportation_coordination": c.FormValue("enable_transportation_coordination"),
		"enable_waiting_list":      c.FormValue("enable_waiting_list"),
		"enable_sponsors":          c.FormValue("enable_sponsors"),
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
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "events",
				Stats:     stats,
			},
			Event:     event,
			Errors:    errs,
			Form:      formValues,
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
	event.EnableTelegram = c.FormValue("enable_telegram") == "true"
	event.EnableJobTitle = c.FormValue("enable_job_title") == "true"
	event.EnableIndustrialEstate = c.FormValue("enable_industrial_estate") == "true"
	event.EnableEmergencyContact = c.FormValue("enable_emergency_contact") == "true"
	event.EnableVehicleInfo = c.FormValue("enable_vehicle_info") == "true"
	event.EnableCarpool = c.FormValue("enable_carpool") == "true"
	event.EnableTShirtSize = c.FormValue("enable_tshirt_size") == "true"
	event.EnableTransportationCoordination = c.FormValue("enable_transportation_coordination") == "true"
	event.EnableWaitingList = c.FormValue("enable_waiting_list") == "true"
	event.EnableSponsors = c.FormValue("enable_sponsors") == "true"

	if err := h.eventService.Update(event); err != nil {
		errs = append(errs, "Gagal memperbarui Acara: "+err.Error())
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_event_edit.html", AdminEventEditData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "events",
				Stats:     stats,
			},
			Event:     event,
			Errors:    errs,
			Form:      formValues,
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "events",
			Stats:        stats,
		},
		Event:        event,
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
	case models.StatusPending, models.StatusRegistered:
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
	case models.StatusWaitlist:
		actions = fmt.Sprintf(`
<button onclick="confirmAction('/admin/participants/%d/status', 'REGISTERED', 'Apakah Anda yakin ingin mempromosikan %s menjadi REGISTERED?', 'REGISTERED')"
  class="flex-1 flex items-center justify-center gap-2 bg-indigo-600 hover:bg-indigo-500 text-white font-bold py-2.5 px-5 rounded-xl text-sm transition-colors">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 11l3-3m0 0l3 3m-3-3v8M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
  Promosikan ke Registered
</button>`, p.ID, p.FullName)
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
VERIFIED</span>`
	case models.StatusRejected:
		return `<span class="inline-flex items-center gap-2 bg-red-500/20 border border-red-500/40 text-red-300 rounded-full px-4 py-1.5 text-sm font-bold">
<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
REJECTED</span>`
	case models.StatusWaitlist:
		return `<span class="inline-flex items-center gap-2 bg-cyan-500/20 border border-cyan-500/40 text-cyan-300 rounded-full px-4 py-1.5 text-sm font-bold">
<span class="w-2 h-2 bg-cyan-400 rounded-full" style="animation:pulse 2s infinite"></span>
WAITLIST</span>`
	case models.StatusRegistered:
		return `<span class="inline-flex items-center gap-2 bg-indigo-500/20 border border-indigo-500/40 text-indigo-300 rounded-full px-4 py-1.5 text-sm font-bold">
<span class="w-2 h-2 bg-indigo-400 rounded-full" style="animation:pulse 2s infinite"></span>
REGISTERED</span>`
	default:
		return `<span class="inline-flex items-center gap-2 bg-amber-500/20 border border-amber-500/40 text-amber-300 rounded-full px-4 py-1.5 text-sm font-bold">
<span class="w-2 h-2 bg-amber-400 rounded-full" style="animation:pulse 2s infinite"></span>
PENDING</span>`
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "admins",
			Stats:     stats,
		},
		Admins:    admins,
	})
}

// AdminCreatePage handles GET /admin/admins/create
func (h *AdminHandler) AdminCreatePage(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)
	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_admin_create.html", AdminAdminCreateData{
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "admins",
			Stats:     stats,
		},
		Form:      map[string]string{},
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
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "admins",
				Stats:     stats,
			},
			Errors:    errs,
			Form:      formValues,
		})
	}

	// Password hash using bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		errs = append(errs, "Gagal memproses password: "+err.Error())
		return h.render(c, "admin_admin_create.html", AdminAdminCreateData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "admins",
				Stats:     stats,
			},
			Errors:    errs,
			Form:      formValues,
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
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "admins",
				Stats:     stats,
			},
			Errors:    errs,
			Form:      formValues,
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "admins",
			Stats:     stats,
		},
		Admin:     admin,
		Form:      formValues,
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
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "admins",
				Stats:     stats,
			},
			Admin:     admin,
			Errors:    errs,
			Form:      formValues,
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
				AdminBase: AdminBase{
					AdminUser:  adminUser,
					AdminRole:  adminRole,
					ActiveMenu: "admins",
					Stats:     stats,
				},
				Admin:     admin,
				Errors:    errs,
				Form:      formValues,
			})
		}
		admin.PasswordHash = string(hash)
	}

	if err := h.adminService.UpdateAdmin(admin); err != nil {
		errs = append(errs, "Gagal memperbarui Admin: "+err.Error())
		return h.render(c, "admin_admin_edit.html", AdminAdminEditData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "admins",
				Stats:     stats,
			},
			Admin:     admin,
			Errors:    errs,
			Form:      formValues,
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "settings",
			Stats:               stats,
		},
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
	} else {
		if err := os.MkdirAll(storagePath, 0755); err != nil {
			errs = append(errs, "Path Penyimpanan tidak dapat dibuat atau diakses: "+err.Error())
		} else {
			tempFile, err := os.CreateTemp(storagePath, ".settings_write_test_*")
			if err != nil {
				errs = append(errs, "Path Penyimpanan tidak dapat ditulis (tidak memiliki izin menulis): "+err.Error())
			} else {
				tempFile.Close()
				os.Remove(tempFile.Name())
			}
		}
	}

	if len(errs) > 0 {
		return h.render(c, "admin_settings.html", AdminSettingsData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "settings",
				Stats:               stats,
			},
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
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "settings",
				Stats:               stats,
			},
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
		})
	}

	// Write audit log entry
	ip := c.IP()
	ua := c.Get("User-Agent")
	if err := h.auditLogService.Log(adminUser, "UPDATE", "Setting", 0, oldSettings, newSettings, ip, ua); err != nil {
		log.Printf("Warning: failed to log settings update: %v", err)
	}

	return h.render(c, "admin_settings.html", AdminSettingsData{
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "settings",
			Stats:               stats,
		},
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
			case models.StatusWaitlist:
				return "bg-cyan-500/15 text-cyan-300 border-cyan-500/30"
			case models.StatusRegistered:
				return "bg-indigo-500/15 text-indigo-300 border-indigo-500/30"
			default:
				return "bg-amber-500/15 text-amber-300 border-amber-500/30"
			}
		},
		"statusLabel": func(status models.ParticipantStatus) string {
			switch status {
			case models.StatusVerified:
				return "Verified"
			case models.StatusRejected:
				return "Rejected"
			case models.StatusWaitlist:
				return "Waitlist"
			case models.StatusRegistered:
				return "Registered"
			default:
				return "Pending"
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
		"sidebarItems": func(role string, active string, stats *services.ParticipantStats) []MenuItem {
			return BuildSidebarItems(role, active, stats)
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "notifications",
			Stats:     stats,
		},
		Logs:      logs,
		Event:     event,
	})
}

type AdminAuditLogsData struct {
	AdminBase
	Logs           []models.AuditLog
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "audit-logs",
			Stats:          stats,
		},
		Logs:           logs,
		Search:         search,
		FilterResource: resourceFilter,
		FilterAction:   actionFilter,
		StartDate:      startDate,
		EndDate:        endDate,
	})
}

type AdminParticipantQRData struct {
	AdminBase
	Participant *models.Participant
	Event       *models.Event
	QRToken     string
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "participants",
			Stats:       stats,
		},
		Participant: participant,
		Event:       &participant.Event,
		QRToken:     token,
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "backups",
			Stats:        stats,
		},
		Backups:      backups,
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "checkin",
			Stats:        stats,
		},
		Participants: participants,
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
	AdminBase
	Events          []models.Event
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "attendance",
			Stats:           stats,
		},
		Events:          events,
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
	stats, _ := h.participantService.GetStats()

	data := fiber.Map{
		"AdminUser":  adminUser,
		"AdminRole":  adminRole,
		"ActiveMenu": "broadcasts",
		"Stats":      stats,
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
	stats, _ := h.participantService.GetStats()

	data := fiber.Map{
		"AdminUser":  adminUser,
		"AdminRole":  adminRole,
		"ActiveMenu": "broadcasts",
		"Stats":      stats,
		"Events":     events,
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
	stats, _ := h.participantService.GetStats()

	data := fiber.Map{
		"AdminUser":  adminUser,
		"AdminRole":  adminRole,
		"ActiveMenu": "broadcasts",
		"Stats":      stats,
		"Broadcast":  broadcast,
		"Logs":       logs,
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
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "system",
			Stats:     stats,
		},
		Health:    &report,
	})
}

type MenuItem struct {
	Title          string
	URL            string
	Icon           template.HTML
	Active         bool
	Badge          int
	SuperAdminOnly bool
	External       bool
}

func BuildSidebarItems(role string, active string, stats *services.ParticipantStats) []MenuItem {
	pendingCount := 0
	if stats != nil {
		pendingCount = int(stats.Pending)
	}

	items := []MenuItem{
		{
			Title:  "Dashboard",
			URL:    "/admin/dashboard",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"/></svg>`,
			Active: active == "dashboard",
		},
		{
			Title:  "Peserta",
			URL:    "/admin/participants",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z"/></svg>`,
			Active: active == "participants",
			Badge:  pendingCount,
		},
		{
			Title:  "Acara",
			URL:    "/admin/events",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/></svg>`,
			Active: active == "events",
		},
		{
			Title:  "Notifikasi",
			URL:    "/admin/notifications",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/></svg>`,
			Active: active == "notifications",
		},
		{
			Title:  "Log Audit",
			URL:    "/admin/audit-logs",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>`,
			Active: active == "audit-logs",
		},
		{
			Title:  "Backup & Restore",
			URL:    "/admin/backups",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"/></svg>`,
			Active: active == "backups",
		},
		{
			Title:  "Check-In Manual",
			URL:    "/admin/checkin",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/></svg>`,
			Active: active == "checkin",
		},
		{
			Title:  "Kehadiran Real-time",
			URL:    "/admin/attendance",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 002 2h2a2 2 0 002-2z"/></svg>`,
			Active: active == "attendance",
		},
		{
			Title:  "Pusat Broadcast",
			URL:    "/admin/broadcasts",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z"/></svg>`,
			Active: active == "broadcasts",
		},
		{
			Title:  "Koordinasi Transportasi",
			URL:    "/admin/transportation",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"/></svg>`,
			Active: active == "transportation",
		},
		{
			Title:  "Sponsor & Partner",
			URL:    "/admin/sponsors",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>`,
			Active: active == "sponsors",
		},
		{
			Title:  "Persiapan Acara",
			URL:    "/admin/tasks",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"/></svg>`,
			Active: active == "tasks",
		},
		{
			Title:  "Kesehatan Sistem",
			URL:    "/admin/system",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3H5a2 2 0 00-2 2v4m6-6h10a2 2 0 012 2v4M9 3v18m0 0h10a2 2 0 002-2V9M9 21H5a2 2 0 01-2-2V9m0 0h18"/></svg>`,
			Active: active == "system",
		},
	}

	if role == "SUPER_ADMIN" {
		items = append(items, MenuItem{
			Title:  "Kelola Admin",
			URL:    "/admin/admins",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"/></svg>`,
			Active: active == "admins",
		})
		items = append(items, MenuItem{
			Title:  "Pengaturan",
			URL:    "/admin/settings",
			Icon:   `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/></svg>`,
			Active: active == "settings",
		})
	}

	items = append(items, MenuItem{
		Title:    "Halaman Publik",
		URL:      "/",
		Icon:     `<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"/></svg>`,
		External: true,
	})

	return items
}

type DriverInfo struct {
	models.Participant
	Passengers    []models.Participant
	OccupiedSeats int
}

type AdminTransportationData struct {
	AdminBase
	Event           *models.Event
	Drivers         []DriverInfo
	Passengers      []models.Participant
	AllDrivers      []models.Participant
	StatusFilter    string
	SearchQuery     string
	TotalSeats      int
	OccupiedSeats   int
	TotalDrivers    int
	TotalPassengers int
}

// TransportationCoordination handles GET /admin/transportation
func (h *AdminHandler) TransportationCoordination(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	event, err := h.eventService.GetFirst()
	if err != nil || event == nil {
		return h.render(c, "admin_transportation.html", fiber.Map{
			"Error": "Acara tidak ditemukan",
		})
	}

	// Fetch Stats for Sidebar
	stats, _ := h.participantService.GetStats()

	// Query Drivers (Verified and has vehicle and is a car/mobil and can bring others)
	var driverParticipants []models.Participant
	err = h.participantService.GetDB().
		Where("event_id = ? AND status IN ('VERIFIED', 'CHECKED_IN') AND own_vehicle = ? AND (vehicle_type = ? OR vehicle_type = ?) AND carpool_can_bring = ?",
			event.ID, true, "Car", "Mobil", true).
		Order("full_name ASC").
		Find(&driverParticipants).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal mengambil data pengemudi: " + err.Error())
	}

	var drivers []DriverInfo
	for _, dp := range driverParticipants {
		var passengers []models.Participant
		h.participantService.GetDB().Where("driver_id = ?", dp.ID).Find(&passengers)
		drivers = append(drivers, DriverInfo{
			Participant:   dp,
			Passengers:    passengers,
			OccupiedSeats: len(passengers),
		})
	}

	// Filters for Passengers Pool
	statusFilter := strings.TrimSpace(c.Query("status"))
	searchQuery := strings.TrimSpace(c.Query("q"))

	// Query Passengers (Verified/Checked-In, but not drivers)
	qPassengers := h.participantService.GetDB().Preload("Driver").
		Where("event_id = ? AND status IN ('VERIFIED', 'CHECKED_IN')", event.ID).
		Where("NOT (own_vehicle = ? AND (vehicle_type = ? OR vehicle_type = ?) AND carpool_can_bring = ?)", true, "Car", "Mobil", true)

	if statusFilter == "assigned" {
		qPassengers = qPassengers.Where("driver_id IS NOT NULL")
	} else if statusFilter == "unassigned" {
		qPassengers = qPassengers.Where("driver_id IS NULL")
	}
	if searchQuery != "" {
		like := "%" + searchQuery + "%"
		qPassengers = qPassengers.Where("full_name ILIKE ? OR company_name ILIKE ? OR phone ILIKE ?", like, like, like)
	}

	var passengers []models.Participant
	err = qPassengers.Order("full_name ASC").Find(&passengers).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal mengambil data penumpang: " + err.Error())
	}

	// Calculate counts
	totalSeats := 0
	occupiedSeats := 0
	for _, dr := range drivers {
		totalSeats += dr.CarpoolSeats
		occupiedSeats += dr.OccupiedSeats
	}

	return h.render(c, "admin_transportation.html", AdminTransportationData{
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "transportation",
			Stats:       stats,
		},
		Event:           event,
		Drivers:         drivers,
		Passengers:      passengers,
		AllDrivers:      driverParticipants,
		StatusFilter:    statusFilter,
		SearchQuery:     searchQuery,
		TotalSeats:      totalSeats,
		OccupiedSeats:   occupiedSeats,
		TotalDrivers:    len(drivers),
		TotalPassengers: len(passengers),
	})
}

// AssignPassenger handles POST /admin/transportation/assign
func (h *AdminHandler) AssignPassenger(c *fiber.Ctx) error {
	passengerID, err := strconv.Atoi(c.FormValue("passenger_id"))
	if err != nil || passengerID <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("ID penumpang tidak valid")
	}

	driverID, err := strconv.Atoi(c.FormValue("driver_id"))
	if err != nil || driverID <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("ID pengemudi tidak valid")
	}

	// Fetch passenger
	var passenger models.Participant
	if err := h.participantService.GetDB().First(&passenger, passengerID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Penumpang tidak ditemukan")
	}

	// Fetch driver
	var driver models.Participant
	if err := h.participantService.GetDB().First(&driver, driverID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Pengemudi tidak ditemukan")
	}

	// Check if passenger is the driver
	if passenger.ID == driver.ID {
		return c.Status(fiber.StatusBadRequest).SendString("Tidak bisa menugaskan pengemudi ke dirinya sendiri")
	}

	// Check available seats
	var currentPassengersCount int64
	h.participantService.GetDB().Model(&models.Participant{}).Where("driver_id = ?", driver.ID).Count(&currentPassengersCount)

	if int(currentPassengersCount) >= driver.CarpoolSeats {
		return c.Status(fiber.StatusBadRequest).SendString("Jumlah kursi yang tersedia pada pengemudi ini sudah penuh")
	}

	// Assign
	dID := uint(driverID)
	passenger.DriverID = &dID
	if err := h.participantService.GetDB().Save(&passenger).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal menugaskan penumpang: " + err.Error())
	}

	// Log audit trail
	adminUser, _ := c.Locals("admin_username").(string)
	_ = h.auditLogService.Log(adminUser, "ASSIGN_PASSENGER", "PARTICIPANT", passenger.ID, nil, map[string]string{"driver_id": strconv.Itoa(int(driver.ID))}, c.IP(), c.Get("User-Agent"))

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Refresh", "true")
		return c.SendStatus(fiber.StatusOK)
	}
	return c.Redirect("/admin/transportation", fiber.StatusSeeOther)
}

// UnassignPassenger handles POST /admin/transportation/unassign
func (h *AdminHandler) UnassignPassenger(c *fiber.Ctx) error {
	passengerID, err := strconv.Atoi(c.FormValue("passenger_id"))
	if err != nil || passengerID <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("ID penumpang tidak valid")
	}

	// Fetch passenger
	var passenger models.Participant
	if err := h.participantService.GetDB().First(&passenger, passengerID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Penumpang tidak ditemukan")
	}

	passenger.DriverID = nil
	if err := h.participantService.GetDB().Save(&passenger).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal melepas penumpang: " + err.Error())
	}

	// Log audit trail
	adminUser, _ := c.Locals("admin_username").(string)
	_ = h.auditLogService.Log(adminUser, "UNASSIGN_PASSENGER", "PARTICIPANT", passenger.ID, nil, nil, c.IP(), c.Get("User-Agent"))

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Refresh", "true")
		return c.SendStatus(fiber.StatusOK)
	}
	return c.Redirect("/admin/transportation", fiber.StatusSeeOther)
}

// UpdateDriverDetails handles POST /admin/transportation/driver-details
func (h *AdminHandler) UpdateDriverDetails(c *fiber.Ctx) error {
	driverID, err := strconv.Atoi(c.FormValue("driver_id"))
	if err != nil || driverID <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString("ID pengemudi tidak valid")
	}

	meetingPoint := strings.TrimSpace(c.FormValue("meeting_point"))
	notes := strings.TrimSpace(c.FormValue("notes"))
	officialDriver := c.FormValue("official_driver") == "true"

	// Fetch driver
	var driver models.Participant
	if err := h.participantService.GetDB().First(&driver, driverID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Pengemudi tidak ditemukan")
	}

	driver.TransportMeetingPoint = meetingPoint
	driver.TransportNotes = notes
	driver.OfficialDriver = officialDriver

	if err := h.participantService.GetDB().Save(&driver).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal menyimpan detail keberangkatan: " + err.Error())
	}

	// Log audit trail
	adminUser, _ := c.Locals("admin_username").(string)
	_ = h.auditLogService.Log(adminUser, "UPDATE_DRIVER_DETAILS", "PARTICIPANT", driver.ID, nil, map[string]string{
		"meeting_point":   meetingPoint,
		"notes":           notes,
		"official_driver": strconv.FormatBool(officialDriver),
	}, c.IP(), c.Get("User-Agent"))

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Refresh", "true")
		return c.SendStatus(fiber.StatusOK)
	}
	return c.Redirect("/admin/transportation", fiber.StatusSeeOther)
}

// ExportTransportation handles GET /admin/transportation/export
func (h *AdminHandler) ExportTransportation(c *fiber.Ctx) error {
	event, err := h.eventService.GetFirst()
	if err != nil || event == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Acara tidak ditemukan")
	}

	// Fetch all verified/checked-in participants for this event
	var participants []models.Participant
	err = h.participantService.GetDB().Preload("Driver").
		Where("event_id = ? AND status IN ('VERIFIED', 'CHECKED_IN')", event.ID).
		Order("full_name ASC").
		Find(&participants).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal mengambil data peserta")
	}

	// Build XLSX
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Transportasi"
	f.SetSheetName("Sheet1", sheet)

	// Style header
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

	dataStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: false},
		Border: []excelize.Border{
			{Type: "left", Color: "E5E7EB", Style: 1},
			{Type: "right", Color: "E5E7EB", Style: 1},
			{Type: "bottom", Color: "E5E7EB", Style: 1},
		},
	})

	colNumToName := func(col int) string {
		name := ""
		for col > 0 {
			col--
			name = string(rune('A'+col%26)) + name
			col /= 26
		}
		return name
	}

	type colDef struct {
		label string
		width float64
		value func(p *models.Participant) string
	}

	cols := []colDef{
		{"Participant", 24, func(p *models.Participant) string { return p.FullName }},
		{"WhatsApp", 18, func(p *models.Participant) string { return p.Phone }},
		{"Driver", 24, func(p *models.Participant) string {
			if p.OwnVehicle && p.VehicleType == "Car" && p.CarpoolCanBring {
				return "Self (Driver)"
			}
			if p.DriverID != nil && p.Driver != nil {
				return p.Driver.FullName
			}
			return "–"
		}},
		{"Driver Phone", 18, func(p *models.Participant) string {
			if p.OwnVehicle && p.VehicleType == "Car" && p.CarpoolCanBring {
				return p.Phone
			}
			if p.DriverID != nil && p.Driver != nil {
				return p.Driver.Phone
			}
			return "–"
		}},
		{"Vehicle Plate", 16, func(p *models.Participant) string {
			if p.OwnVehicle && p.VehicleType == "Car" && p.CarpoolCanBring {
				return p.LicensePlate
			}
			if p.DriverID != nil && p.Driver != nil {
				return p.Driver.LicensePlate
			}
			return "–"
		}},
		{"Meeting Point", 24, func(p *models.Participant) string {
			if p.OwnVehicle && p.VehicleType == "Car" && p.CarpoolCanBring {
				return p.TransportMeetingPoint
			}
			if p.DriverID != nil && p.Driver != nil {
				return p.Driver.TransportMeetingPoint
			}
			return "–"
		}},
		{"Departure Notes", 30, func(p *models.Participant) string {
			if p.OwnVehicle && p.VehicleType == "Car" && p.CarpoolCanBring {
				return p.TransportNotes
			}
			if p.DriverID != nil && p.Driver != nil {
				return p.Driver.TransportNotes
			}
			return "–"
		}},
		{"Transportation Status", 24, func(p *models.Participant) string {
			return p.GetTransportationStatus()
		}},
		{"Official Driver", 18, func(p *models.Participant) string {
			if p.OfficialDriver {
				return "Yes"
			}
			return "No"
		}},
	}

	// Write Headers
	for ci, col := range cols {
		colLetter := colNumToName(ci + 1)
		cell := colLetter + "1"
		f.SetCellValue(sheet, cell, col.label)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
		f.SetColWidth(sheet, colLetter, colLetter, col.width)
	}
	f.SetRowHeight(sheet, 1, 22)

	// Write Data
	for ri, p := range participants {
		row := ri + 2
		rowStr := strconv.Itoa(row)
		for ci, col := range cols {
			colLetter := colNumToName(ci + 1)
			cell := colLetter + rowStr
			f.SetCellValue(sheet, cell, col.value(&p))
			f.SetCellStyle(sheet, cell, cell, dataStyle)
		}
		f.SetRowHeight(sheet, row, 20)
	}

	// Stream file to response
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	filename := fmt.Sprintf("transport-coordination-%s.xlsx", event.Slug)
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	if err := f.Write(c.Response().BodyWriter()); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Gagal menulis file spreadsheet")
	}

	return nil
}

// SponsorList handles GET /admin/sponsors
func (h *AdminHandler) SponsorList(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	sponsors, err := h.sponsorService.GetAllForAdmin()
	if err != nil {
		sponsors = []models.Sponsor{}
	}

	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_sponsors.html", AdminSponsorsData{
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "sponsors",
			Stats:      stats,
		},
		Sponsors:     sponsors,
		FlashSuccess: getFlash(c, "success"),
		FlashError:   getFlash(c, "error"),
	})
}

// SponsorCreatePage handles GET /admin/sponsors/create
func (h *AdminHandler) SponsorCreatePage(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	events, _ := h.eventService.GetAll()
	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_sponsor_create.html", AdminSponsorCreateData{
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "sponsors",
			Stats:      stats,
		},
		Events:       events,
		Form:         map[string]string{"active": "true", "display_order": "0"},
		FlashSuccess: getFlash(c, "success"),
		FlashError:   getFlash(c, "error"),
	})
}

// SponsorCreateSubmit handles POST /admin/sponsors/create
func (h *AdminHandler) SponsorCreateSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	eventIDStr := c.FormValue("event_id")
	name := strings.TrimSpace(c.FormValue("name"))
	category := strings.TrimSpace(c.FormValue("category"))
	websiteURL := strings.TrimSpace(c.FormValue("website_url"))
	displayOrderStr := strings.TrimSpace(c.FormValue("display_order"))
	active := c.FormValue("active") == "true" || c.FormValue("active") == "on"

	formValues := map[string]string{
		"event_id":      eventIDStr,
		"name":          name,
		"category":      category,
		"website_url":   websiteURL,
		"display_order": displayOrderStr,
		"active":        c.FormValue("active"),
	}

	var errs []string
	var eventID int
	var err error
	if eventIDStr != "" {
		eventID, err = strconv.Atoi(eventIDStr)
		if err != nil || eventID <= 0 {
			errs = append(errs, "Pilihan Acara tidak valid")
		}
	} else {
		errs = append(errs, "Acara wajib dipilih")
	}

	if name == "" {
		errs = append(errs, "Nama Sponsor/Partner wajib diisi")
	}

	allowedCategories := map[string]bool{
		"Title Sponsor": true, "Platinum Sponsor": true, "Gold Sponsor": true,
		"Silver Sponsor": true, "Bronze Sponsor": true, "Community Partner": true,
		"Media Partner": true,
	}
	if category == "" {
		errs = append(errs, "Kategori wajib diisi")
	} else if !allowedCategories[category] {
		errs = append(errs, "Kategori tidak valid")
	}

	displayOrder := 0
	if displayOrderStr != "" {
		displayOrder, err = strconv.Atoi(displayOrderStr)
		if err != nil {
			errs = append(errs, "Format urutan tampilan tidak valid")
		}
	}

	logoFilename := ""
	file, fileErr := c.FormFile("logo")
	if fileErr != nil {
		errs = append(errs, "Logo wajib diunggah")
	} else {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
		if !allowedExts[ext] {
			errs = append(errs, "Format Logo hanya boleh JPG, JPEG, PNG, atau WEBP")
		} else if file.Size > 5*1024*1024 {
			errs = append(errs, "Ukuran Logo maksimal 5MB")
		} else {
			logoFilename, err = h.storageService.SaveSponsorLogo(file)
			if err != nil {
				errs = append(errs, "Gagal mengunggah Logo: "+err.Error())
			}
		}
	}

	if len(errs) > 0 {
		events, _ := h.eventService.GetAll()
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_sponsor_create.html", AdminSponsorCreateData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "sponsors",
				Stats:      stats,
			},
			Events:       events,
			Errors:       errs,
			Form:         formValues,
			FlashSuccess: getFlash(c, "success"),
			FlashError:   getFlash(c, "error"),
		})
	}

	newSponsor := &models.Sponsor{
		EventID:      uint(eventID),
		Name:         name,
		Category:     category,
		Logo:         logoFilename,
		WebsiteURL:   websiteURL,
		DisplayOrder: displayOrder,
		Active:       active,
	}

	if err := h.sponsorService.Create(newSponsor); err != nil {
		errs = append(errs, "Gagal menyimpan Sponsor/Mitra: "+err.Error())
		events, _ := h.eventService.GetAll()
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_sponsor_create.html", AdminSponsorCreateData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "sponsors",
				Stats:      stats,
			},
			Events:       events,
			Errors:       errs,
			Form:         formValues,
			FlashSuccess: getFlash(c, "success"),
			FlashError:   getFlash(c, "error"),
		})
	}

	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "CREATE", "SPONSOR", newSponsor.ID, "", newSponsor, c.IP(), c.Get("User-Agent"))
	}

	setFlash(c, "success", "Sponsor/Mitra \""+name+"\" berhasil ditambahkan.")
	return c.Redirect("/admin/sponsors", fiber.StatusSeeOther)
}

// SponsorEditPage handles GET /admin/sponsors/:id/edit
func (h *AdminHandler) SponsorEditPage(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Redirect("/admin/sponsors", fiber.StatusSeeOther)
	}

	sponsor, err := h.sponsorService.GetByID(uint(id))
	if err != nil {
		return c.Redirect("/admin/sponsors", fiber.StatusSeeOther)
	}

	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	events, _ := h.eventService.GetAll()
	stats, _ := h.participantService.GetStats()

	formValues := map[string]string{
		"event_id":      strconv.Itoa(int(sponsor.EventID)),
		"name":          sponsor.Name,
		"category":      sponsor.Category,
		"website_url":   sponsor.WebsiteURL,
		"display_order": strconv.Itoa(sponsor.DisplayOrder),
		"active":        strconv.FormatBool(sponsor.Active),
	}

	return h.render(c, "admin_sponsor_edit.html", AdminSponsorEditData{
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "sponsors",
			Stats:      stats,
		},
		Sponsor:      sponsor,
		Events:       events,
		Form:         formValues,
		FlashSuccess: getFlash(c, "success"),
		FlashError:   getFlash(c, "error"),
	})
}

// SponsorEditSubmit handles POST /admin/sponsors/:id/edit
func (h *AdminHandler) SponsorEditSubmit(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Redirect("/admin/sponsors", fiber.StatusSeeOther)
	}

	sponsor, err := h.sponsorService.GetByID(uint(id))
	if err != nil {
		return c.Redirect("/admin/sponsors", fiber.StatusSeeOther)
	}

	oldSponsorJSON := ""
	if bytes, err := json.Marshal(sponsor); err == nil {
		oldSponsorJSON = string(bytes)
	}

	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	eventIDStr := c.FormValue("event_id")
	name := strings.TrimSpace(c.FormValue("name"))
	category := strings.TrimSpace(c.FormValue("category"))
	websiteURL := strings.TrimSpace(c.FormValue("website_url"))
	displayOrderStr := strings.TrimSpace(c.FormValue("display_order"))
	active := c.FormValue("active") == "true" || c.FormValue("active") == "on"

	formValues := map[string]string{
		"event_id":      eventIDStr,
		"name":          name,
		"category":      category,
		"website_url":   websiteURL,
		"display_order": displayOrderStr,
		"active":        c.FormValue("active"),
	}

	var errs []string
	var eventID int
	if eventIDStr != "" {
		eventID, err = strconv.Atoi(eventIDStr)
		if err != nil || eventID <= 0 {
			errs = append(errs, "Pilihan Acara tidak valid")
		}
	} else {
		errs = append(errs, "Acara wajib dipilih")
	}

	if name == "" {
		errs = append(errs, "Nama Sponsor/Partner wajib diisi")
	}

	allowedCategories := map[string]bool{
		"Title Sponsor": true, "Platinum Sponsor": true, "Gold Sponsor": true,
		"Silver Sponsor": true, "Bronze Sponsor": true, "Community Partner": true,
		"Media Partner": true,
	}
	if category == "" {
		errs = append(errs, "Kategori wajib diisi")
	} else if !allowedCategories[category] {
		errs = append(errs, "Kategori tidak valid")
	}

	displayOrder := 0
	if displayOrderStr != "" {
		displayOrder, err = strconv.Atoi(displayOrderStr)
		if err != nil {
			errs = append(errs, "Format urutan tampilan tidak valid")
		}
	}

	logoFilename := sponsor.Logo
	file, fileErr := c.FormFile("logo")
	if fileErr == nil && file != nil {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
		if !allowedExts[ext] {
			errs = append(errs, "Format Logo hanya boleh JPG, JPEG, PNG, atau WEBP")
		} else if file.Size > 5*1024*1024 {
			errs = append(errs, "Ukuran Logo maksimal 5MB")
		} else {
			logoFilename, err = h.storageService.SaveSponsorLogo(file)
			if err != nil {
				errs = append(errs, "Gagal mengunggah Logo: "+err.Error())
			}
		}
	}

	if len(errs) > 0 {
		events, _ := h.eventService.GetAll()
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_sponsor_edit.html", AdminSponsorEditData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "sponsors",
				Stats:      stats,
			},
			Sponsor:      sponsor,
			Events:       events,
			Errors:       errs,
			Form:         formValues,
			FlashSuccess: getFlash(c, "success"),
			FlashError:   getFlash(c, "error"),
		})
	}

	sponsor.EventID = uint(eventID)
	sponsor.Name = name
	sponsor.Category = category
	sponsor.Logo = logoFilename
	sponsor.WebsiteURL = websiteURL
	sponsor.DisplayOrder = displayOrder
	sponsor.Active = active

	if err := h.sponsorService.Update(sponsor); err != nil {
		errs = append(errs, "Gagal memperbarui Sponsor/Mitra: "+err.Error())
		events, _ := h.eventService.GetAll()
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_sponsor_edit.html", AdminSponsorEditData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "sponsors",
				Stats:      stats,
			},
			Sponsor:      sponsor,
			Events:       events,
			Errors:       errs,
			Form:         formValues,
			FlashSuccess: getFlash(c, "success"),
			FlashError:   getFlash(c, "error"),
		})
	}

	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "UPDATE", "SPONSOR", sponsor.ID, oldSponsorJSON, sponsor, c.IP(), c.Get("User-Agent"))
	}

	setFlash(c, "success", "Sponsor/Mitra \""+name+"\" berhasil diperbarui.")
	return c.Redirect("/admin/sponsors", fiber.StatusSeeOther)
}

// SponsorDelete handles POST /admin/sponsors/:id/delete
func (h *AdminHandler) SponsorDelete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	sponsor, err := h.sponsorService.GetByID(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Sponsor tidak ditemukan"})
	}

	sponsorJSON := ""
	if bytes, err := json.Marshal(sponsor); err == nil {
		sponsorJSON = string(bytes)
	}

	if err := h.sponsorService.Delete(uint(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Gagal menghapus sponsor: " + err.Error()})
	}

	adminUser, _ := c.Locals("admin_username").(string)
	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "DELETE", "SPONSOR", uint(id), sponsorJSON, nil, c.IP(), c.Get("User-Agent"))
	}

	setFlash(c, "success", "Sponsor/Mitra \""+sponsor.Name+"\" berhasil dihapus.")
	return c.Redirect("/admin/sponsors", fiber.StatusSeeOther)
}

// TaskList handles GET /admin/tasks
func (h *AdminHandler) TaskList(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	events, _ := h.eventService.GetAll()
	eventIDStr := c.Query("event_id")
	var selectedEventID uint
	if eventIDStr != "" {
		if id, err := strconv.Atoi(eventIDStr); err == nil {
			selectedEventID = uint(id)
		}
	} else {
		// Default to first event
		if len(events) > 0 {
			selectedEventID = events[0].ID
		}
	}

	var selectedEvent *models.Event
	for i := range events {
		if events[i].ID == selectedEventID {
			selectedEvent = &events[i]
			break
		}
	}

	categoryFilter := c.Query("category")
	priorityFilter := c.Query("priority")
	statusFilter := c.Query("status")

	var tasks []models.Task
	var err error
	if selectedEventID > 0 {
		tasks, err = h.taskService.GetTasksByEvent(selectedEventID, categoryFilter, priorityFilter, statusFilter)
		if err != nil {
			tasks = []models.Task{}
		}
	}

	totalCount := 0
	completedCount := 0
	inProgressCount := 0
	overdueCount := 0
	if selectedEventID > 0 {
		totalCount, completedCount, inProgressCount, overdueCount, _ = h.taskService.GetTaskStatsDetailed(selectedEventID)
	}

	stats, _ := h.participantService.GetStats()

	return h.render(c, "admin_tasks.html", AdminTasksData{
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "tasks",
			Stats:      stats,
		},
		Tasks:           tasks,
		Events:          events,
		SelectedEventID: selectedEventID,
		SelectedEvent:   selectedEvent,
		CategoryFilter:  categoryFilter,
		PriorityFilter:  priorityFilter,
		StatusFilter:    statusFilter,
		TotalTasksCount:      totalCount,
		CompletedTasksCount:  completedCount,
		InProgressTasksCount: inProgressCount,
		OverdueTasksCount:    overdueCount,
		FlashSuccess:    getFlash(c, "success"),
		FlashError:      getFlash(c, "error"),
	})
}

// TaskCreatePage handles GET /admin/tasks/create
func (h *AdminHandler) TaskCreatePage(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	events, _ := h.eventService.GetAll()
	stats, _ := h.participantService.GetStats()

	eventIDStr := c.Query("event_id")
	formValues := map[string]string{
		"event_id": eventIDStr,
		"status":   "Todo",
	}

	return h.render(c, "admin_task_create.html", AdminTaskCreateData{
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "tasks",
			Stats:      stats,
		},
		Events:       events,
		Form:         formValues,
		FlashSuccess: getFlash(c, "success"),
		FlashError:   getFlash(c, "error"),
	})
}

// TaskCreateSubmit handles POST /admin/tasks/create
func (h *AdminHandler) TaskCreateSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	eventIDStr := c.FormValue("event_id")
	title := strings.TrimSpace(c.FormValue("title"))
	description := strings.TrimSpace(c.FormValue("description"))
	category := strings.TrimSpace(c.FormValue("category"))
	priority := strings.TrimSpace(c.FormValue("priority"))
	dueDateStr := strings.TrimSpace(c.FormValue("due_date"))
	assignedTo := strings.TrimSpace(c.FormValue("assigned_to"))
	status := strings.TrimSpace(c.FormValue("status"))

	formValues := map[string]string{
		"event_id":    eventIDStr,
		"title":       title,
		"description": description,
		"category":    category,
		"priority":    priority,
		"due_date":    dueDateStr,
		"assigned_to": assignedTo,
		"status":      status,
	}

	var errs []string
	var eventID int
	var err error
	if eventIDStr != "" {
		eventID, err = strconv.Atoi(eventIDStr)
		if err != nil || eventID <= 0 {
			errs = append(errs, "Pilihan Acara tidak valid")
		}
	} else {
		errs = append(errs, "Acara wajib dipilih")
	}

	if title == "" {
		errs = append(errs, "Judul tugas wajib diisi")
	}

	allowedCategories := map[string]bool{
		"Registration": true, "Transportation": true, "Merchandise": true,
		"Sponsor": true, "Venue": true, "Broadcast": true, "Logistics": true, "Other": true,
	}
	if category == "" {
		errs = append(errs, "Kategori wajib diisi")
	} else if !allowedCategories[category] {
		errs = append(errs, "Kategori tidak valid")
	}

	allowedPriorities := map[string]bool{
		"Low": true, "Medium": true, "High": true, "Critical": true,
	}
	if priority == "" {
		errs = append(errs, "Prioritas wajib diisi")
	} else if !allowedPriorities[priority] {
		errs = append(errs, "Prioritas tidak valid")
	}

	allowedStatuses := map[string]bool{
		"Todo": true, "In Progress": true, "Done": true, "Cancelled": true,
	}
	if status == "" {
		status = "Todo"
	} else if !allowedStatuses[status] {
		errs = append(errs, "Status tidak valid")
	}

	var dueDate time.Time
	if dueDateStr == "" {
		errs = append(errs, "Tanggal Jatuh Tempo wajib diisi")
	} else {
		dueDate, err = time.Parse("2006-01-02", dueDateStr)
		if err != nil {
			errs = append(errs, "Format Tanggal Jatuh Tempo harus YYYY-MM-DD")
		}
	}

	if len(errs) > 0 {
		events, _ := h.eventService.GetAll()
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_task_create.html", AdminTaskCreateData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "tasks",
				Stats:      stats,
			},
			Events:       events,
			Errors:       errs,
			Form:         formValues,
			FlashSuccess: getFlash(c, "success"),
			FlashError:   getFlash(c, "error"),
		})
	}

	newTask := &models.Task{
		EventID:     uint(eventID),
		Title:       title,
		Description: description,
		Category:    category,
		Priority:    priority,
		DueDate:     dueDate,
		AssignedTo:  assignedTo,
		Status:      status,
	}

	if err := h.taskService.Create(newTask); err != nil {
		errs = append(errs, "Gagal menyimpan tugas: "+err.Error())
		events, _ := h.eventService.GetAll()
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_task_create.html", AdminTaskCreateData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "tasks",
				Stats:      stats,
			},
			Events:       events,
			Errors:       errs,
			Form:         formValues,
			FlashSuccess: getFlash(c, "success"),
			FlashError:   getFlash(c, "error"),
		})
	}

	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "CREATE", "TASK", newTask.ID, "", newTask, c.IP(), c.Get("User-Agent"))
	}

	setFlash(c, "success", "Tugas \""+title+"\" berhasil dibuat.")
	return c.Redirect("/admin/tasks?event_id="+eventIDStr, fiber.StatusSeeOther)
}

// TaskEditPage handles GET /admin/tasks/:id/edit
func (h *AdminHandler) TaskEditPage(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		setFlash(c, "error", "ID tugas tidak valid")
		return c.Redirect("/admin/tasks", fiber.StatusSeeOther)
	}

	task, err := h.taskService.GetByID(uint(id))
	if err != nil {
		setFlash(c, "error", "Tugas tidak ditemukan")
		return c.Redirect("/admin/tasks", fiber.StatusSeeOther)
	}

	events, _ := h.eventService.GetAll()
	stats, _ := h.participantService.GetStats()

	formValues := map[string]string{
		"event_id":    strconv.Itoa(int(task.EventID)),
		"title":       task.Title,
		"description": task.Description,
		"category":    task.Category,
		"priority":    task.Priority,
		"due_date":    task.DueDate.Format("2006-01-02"),
		"assigned_to": task.AssignedTo,
		"status":      task.Status,
	}

	return h.render(c, "admin_task_edit.html", AdminTaskEditData{
		AdminBase: AdminBase{
			AdminUser:  adminUser,
			AdminRole:  adminRole,
			ActiveMenu: "tasks",
			Stats:      stats,
		},
		Task:         task,
		Events:       events,
		Form:         formValues,
		FlashSuccess: getFlash(c, "success"),
		FlashError:   getFlash(c, "error"),
	})
}

// TaskEditSubmit handles POST /admin/tasks/:id/edit
func (h *AdminHandler) TaskEditSubmit(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)
	adminRole, _ := c.Locals("admin_role").(string)

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		setFlash(c, "error", "ID tugas tidak valid")
		return c.Redirect("/admin/tasks", fiber.StatusSeeOther)
	}

	task, err := h.taskService.GetByID(uint(id))
	if err != nil {
		setFlash(c, "error", "Tugas tidak ditemukan")
		return c.Redirect("/admin/tasks", fiber.StatusSeeOther)
	}

	oldTaskJSON := ""
	if bytes, err := json.Marshal(task); err == nil {
		oldTaskJSON = string(bytes)
	}

	eventIDStr := c.FormValue("event_id")
	title := strings.TrimSpace(c.FormValue("title"))
	description := strings.TrimSpace(c.FormValue("description"))
	category := strings.TrimSpace(c.FormValue("category"))
	priority := strings.TrimSpace(c.FormValue("priority"))
	dueDateStr := strings.TrimSpace(c.FormValue("due_date"))
	assignedTo := strings.TrimSpace(c.FormValue("assigned_to"))
	status := strings.TrimSpace(c.FormValue("status"))

	formValues := map[string]string{
		"event_id":    eventIDStr,
		"title":       title,
		"description": description,
		"category":    category,
		"priority":    priority,
		"due_date":    dueDateStr,
		"assigned_to": assignedTo,
		"status":      status,
	}

	var errs []string
	var eventID int
	if eventIDStr != "" {
		eventID, err = strconv.Atoi(eventIDStr)
		if err != nil || eventID <= 0 {
			errs = append(errs, "Pilihan Acara tidak valid")
		}
	} else {
		errs = append(errs, "Acara wajib dipilih")
	}

	if title == "" {
		errs = append(errs, "Judul tugas wajib diisi")
	}

	allowedCategories := map[string]bool{
		"Registration": true, "Transportation": true, "Merchandise": true,
		"Sponsor": true, "Venue": true, "Broadcast": true, "Logistics": true, "Other": true,
	}
	if category == "" {
		errs = append(errs, "Kategori wajib diisi")
	} else if !allowedCategories[category] {
		errs = append(errs, "Kategori tidak valid")
	}

	allowedPriorities := map[string]bool{
		"Low": true, "Medium": true, "High": true, "Critical": true,
	}
	if priority == "" {
		errs = append(errs, "Prioritas wajib diisi")
	} else if !allowedPriorities[priority] {
		errs = append(errs, "Prioritas tidak valid")
	}

	allowedStatuses := map[string]bool{
		"Todo": true, "In Progress": true, "Done": true, "Cancelled": true,
	}
	if status == "" {
		errs = append(errs, "Status tidak valid")
	} else if !allowedStatuses[status] {
		errs = append(errs, "Status tidak valid")
	}

	var dueDate time.Time
	if dueDateStr == "" {
		errs = append(errs, "Tanggal Jatuh Tempo wajib diisi")
	} else {
		dueDate, err = time.Parse("2006-01-02", dueDateStr)
		if err != nil {
			errs = append(errs, "Format Tanggal Jatuh Tempo harus YYYY-MM-DD")
		}
	}

	if len(errs) > 0 {
		events, _ := h.eventService.GetAll()
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_task_edit.html", AdminTaskEditData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "tasks",
				Stats:      stats,
			},
			Task:         task,
			Events:       events,
			Errors:       errs,
			Form:         formValues,
			FlashSuccess: getFlash(c, "success"),
			FlashError:   getFlash(c, "error"),
		})
	}

	task.EventID = uint(eventID)
	task.Title = title
	task.Description = description
	task.Category = category
	task.Priority = priority
	task.DueDate = dueDate
	task.AssignedTo = assignedTo
	task.Status = status

	if err := h.taskService.Update(task); err != nil {
		errs = append(errs, "Gagal menyimpan tugas: "+err.Error())
		events, _ := h.eventService.GetAll()
		stats, _ := h.participantService.GetStats()
		return h.render(c, "admin_task_edit.html", AdminTaskEditData{
			AdminBase: AdminBase{
				AdminUser:  adminUser,
				AdminRole:  adminRole,
				ActiveMenu: "tasks",
				Stats:      stats,
			},
			Task:         task,
			Events:       events,
			Errors:       errs,
			Form:         formValues,
			FlashSuccess: getFlash(c, "success"),
			FlashError:   getFlash(c, "error"),
		})
	}

	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "UPDATE", "TASK", task.ID, oldTaskJSON, task, c.IP(), c.Get("User-Agent"))
	}

	setFlash(c, "success", "Tugas \""+title+"\" berhasil diperbarui.")
	return c.Redirect("/admin/tasks?event_id="+eventIDStr, fiber.StatusSeeOther)
}

// TaskDelete handles POST /admin/tasks/:id/delete
func (h *AdminHandler) TaskDelete(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	task, err := h.taskService.GetByID(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Tugas tidak ditemukan"})
	}

	taskJSON := ""
	if bytes, err := json.Marshal(task); err == nil {
		taskJSON = string(bytes)
	}

	eventIDStr := strconv.Itoa(int(task.EventID))

	if err := h.taskService.Delete(uint(id)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Gagal menghapus tugas: " + err.Error()})
	}

	adminUser, _ := c.Locals("admin_username").(string)
	if h.auditLogService != nil {
		_ = h.auditLogService.Log(adminUser, "DELETE", "TASK", uint(id), taskJSON, nil, c.IP(), c.Get("User-Agent"))
	}

	setFlash(c, "success", "Tugas \""+task.Title+"\" berhasil dihapus.")
	return c.Redirect("/admin/tasks?event_id="+eventIDStr, fiber.StatusSeeOther)
}

// TaskCreateStarter handles POST /admin/tasks/starter
func (h *AdminHandler) TaskCreateStarter(c *fiber.Ctx) error {
	adminUser, _ := c.Locals("admin_username").(string)

	eventIDStr := c.FormValue("event_id")
	title := strings.TrimSpace(c.FormValue("title"))
	category := strings.TrimSpace(c.FormValue("category"))

	var errs []string
	var eventID int
	var err error
	if eventIDStr != "" {
		eventID, err = strconv.Atoi(eventIDStr)
		if err != nil || eventID <= 0 {
			errs = append(errs, "Pilihan Acara tidak valid")
		}
	} else {
		errs = append(errs, "Acara wajib dipilih")
	}

	if title == "" {
		errs = append(errs, "Judul tugas wajib diisi")
	}
	if category == "" {
		errs = append(errs, "Kategori wajib diisi")
	}

	if len(errs) > 0 {
		setFlash(c, "error", strings.Join(errs, ", "))
		return c.Redirect("/admin/tasks?event_id="+eventIDStr, fiber.StatusSeeOther)
	}

	// Default values
	priority := "Medium"
	status := "Todo"
	dueDate := time.Now().Add(7 * 24 * time.Hour) // 7 days from now

	// If the event exists and has a date, we can set due date to 1 day before the event
	if event, err := h.eventService.GetByID(uint(eventID)); err == nil && event != nil {
		eventPrevDay := event.EventDate.Add(-24 * time.Hour)
		if eventPrevDay.After(time.Now()) {
			dueDate = eventPrevDay
		}
	}

	newTask := &models.Task{
		EventID:     uint(eventID),
		Title:       title,
		Description: "Tugas starter untuk persiapan acara: " + title + ".",
		Category:    category,
		Priority:    priority,
		DueDate:     dueDate,
		AssignedTo:  "Organisator",
		Status:      status,
	}

	if err := h.taskService.Create(newTask); err != nil {
		setFlash(c, "error", "Gagal membuat tugas starter: "+err.Error())
	} else {
		if h.auditLogService != nil {
			_ = h.auditLogService.Log(adminUser, "CREATE", "TASK", newTask.ID, "", newTask, c.IP(), c.Get("User-Agent"))
		}
		setFlash(c, "success", "Tugas starter \""+title+"\" berhasil dibuat.")
	}

	return c.Redirect("/admin/tasks?event_id="+eventIDStr, fiber.StatusSeeOther)
}
