package handlers

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/gatherhub/backend/internal/models"
	"github.com/gatherhub/backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

// EventHandler handles HTTP requests for events
type EventHandler struct {
	eventService *services.EventService
}

// NewEventHandler creates a new EventHandler
func NewEventHandler(eventService *services.EventService) *EventHandler {
	return &EventHandler{eventService: eventService}
}

// List handles GET /api/v1/events — returns JSON list
func (h *EventHandler) List(c *fiber.Ctx) error {
	events, err := h.eventService.GetAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch events",
		})
	}
	return c.JSON(fiber.Map{
		"data":  events,
		"total": len(events),
	})
}

// GetByID handles GET /api/v1/events/:id — returns JSON event detail
func (h *EventHandler) GetByID(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid event ID",
		})
	}

	event, err := h.eventService.GetByID(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{"data": event})
}

// ---- HTML Fragment Handlers (for HTMX) ----

// ListFragment handles GET /fragments/events — returns HTML cards for HTMX
func (h *EventHandler) ListFragment(c *fiber.Ctx) error {
	events, err := h.eventService.GetAll()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(
			`<div class="text-red-400 text-center py-8">Failed to load events. Please refresh.</div>`,
		)
	}

	if len(events) == 0 {
		return c.SendString(
			`<div class="text-white/50 text-center py-12">No events scheduled yet. Check back soon!</div>`,
		)
	}

	var sb strings.Builder
	for _, ev := range events {
		sb.WriteString(renderEventCard(ev))
	}
	c.Set("Content-Type", "text/html")
	return c.SendString(sb.String())
}

// GetFragment handles GET /fragments/events/:id — returns HTML event detail for HTMX
func (h *EventHandler) GetFragment(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).SendString(
			`<div class="text-red-400 text-center py-8">Invalid event ID.</div>`,
		)
	}

	event, err := h.eventService.GetByID(uint(id))
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString(
			`<div class="text-red-400 text-center py-8">Event not found.</div>`,
		)
	}

	c.Set("Content-Type", "text/html")
	return c.SendString(renderEventDetail(*event))
}

// ---- Template Helpers ----

func formatDate(t time.Time) string {
	return t.Format("Monday, 02 January 2006")
}

func formatTime(t time.Time) string {
	return t.Format("15:04 WIB")
}

func formatPrice(price float64) string {
	if price == 0 {
		return "FREE"
	}
	return fmt.Sprintf("Rp %s", formatNumber(int64(price)))
}

func formatNumber(n int64) string {
	s := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += "."
		}
		result += string(c)
	}
	return result
}

func esc(s string) string {
	return template.HTMLEscapeString(s)
}

func renderEventCard(ev models.Event) string {
	price := formatPrice(ev.Price)
	priceClass := "text-emerald-400"
	if ev.Price > 0 {
		priceClass = "text-brand-300"
	}

	return fmt.Sprintf(`
<article class="event-card glass-card rounded-3xl overflow-hidden group cursor-pointer"
         onclick="window.location.href='/event/%s'">
  <div class="h-3 bg-gradient-to-r from-brand-500 to-purple-600"></div>
  <div class="p-8">
    <div class="flex items-start justify-between gap-4 mb-4">
      <h3 class="text-xl font-bold text-white group-hover:text-brand-300 transition-colors leading-tight">%s</h3>
      <span class="flex-shrink-0 %s font-bold text-sm bg-white/5 rounded-xl px-3 py-1.5">%s</span>
    </div>

    <div class="space-y-2.5 mb-6">
      <div class="flex items-center gap-2.5 text-white/60 text-sm">
        <svg class="w-4 h-4 text-brand-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
        </svg>
        <span>%s · %s</span>
      </div>
      <div class="flex items-center gap-2.5 text-white/60 text-sm">
        <svg class="w-4 h-4 text-purple-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z"/>
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"/>
        </svg>
        <span class="line-clamp-1">%s</span>
      </div>
    </div>

    <a href="/event/%s"
       class="inline-flex items-center gap-2 btn-primary text-sm font-semibold px-5 py-2.5 rounded-xl text-white">
      View Details
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 8l4 4m0 0l-4 4m4-4H3"/>
      </svg>
    </a>
  </div>
</article>`,
		esc(ev.Slug),
		esc(ev.Title),
		priceClass,
		esc(price),
		formatDate(ev.EventDate),
		formatTime(ev.EventDate),
		esc(ev.Location),
		esc(ev.Slug),
	)
}

func renderEventDetail(ev models.Event) string {
	price := formatPrice(ev.Price)

	// Build payment info section
	paymentHTML := ""
	if ev.Price > 0 {
		paymentHTML = fmt.Sprintf(`
      <div class="glass-card rounded-2xl p-6 mt-6">
        <h3 class="font-semibold text-white mb-4 flex items-center gap-2">
          <svg class="w-5 h-5 text-brand-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z"/>
          </svg>
          Payment Details
        </h3>
        <div class="space-y-2 text-sm">
          <div class="flex justify-between">
            <span class="text-white/50">Bank</span>
            <span class="text-white font-medium">%s</span>
          </div>
          <div class="flex justify-between">
            <span class="text-white/50">Account No.</span>
            <span class="text-white font-medium font-mono">%s</span>
          </div>
          <div class="flex justify-between">
            <span class="text-white/50">Account Name</span>
            <span class="text-white font-medium">%s</span>
          </div>
          <div class="border-t border-white/10 pt-3 mt-3 flex justify-between">
            <span class="text-white/50">Registration Fee</span>
            <span class="text-brand-300 font-bold text-base">%s</span>
          </div>
        </div>
      </div>`,
			esc(ev.PaymentBank),
			esc(ev.PaymentAccountNumber),
			esc(ev.PaymentAccountName),
			esc(price),
		)
	}

	return fmt.Sprintf(`
<div class="space-y-6" id="event-detail-content">
  <!-- Event Header -->
  <div>
    <h1 class="text-3xl md:text-4xl font-extrabold text-white mb-3 leading-tight">%s</h1>
    <div class="flex flex-wrap gap-3">
      <span class="glass-card rounded-full px-4 py-1.5 text-sm font-semibold text-brand-300">%s</span>
      <span class="glass-card rounded-full px-4 py-1.5 text-sm text-white/60 flex items-center gap-1.5">
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
        </svg>
        %s
      </span>
      <span class="glass-card rounded-full px-4 py-1.5 text-sm text-white/60 flex items-center gap-1.5">
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
        </svg>
        %s
      </span>
    </div>
  </div>

  <!-- Location -->
  <div class="glass-card rounded-2xl p-5 flex items-start gap-4">
    <div class="w-10 h-10 rounded-xl bg-purple-500/20 flex items-center justify-center flex-shrink-0">
      <svg class="w-5 h-5 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z"/>
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"/>
      </svg>
    </div>
    <div>
      <div class="text-xs text-white/40 font-medium uppercase tracking-widest mb-1">Location</div>
      <div class="text-white font-semibold">%s</div>
    </div>
  </div>

  <!-- Description -->
  <div class="glass-card rounded-2xl p-6">
    <h3 class="font-semibold text-white mb-3 flex items-center gap-2">
      <svg class="w-5 h-5 text-brand-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
      </svg>
      About This Event
    </h3>
    <p class="text-white/60 leading-relaxed whitespace-pre-line text-sm">%s</p>
  </div>

  %s

  <!-- Admin Contact -->
  <div class="glass-card rounded-2xl p-5 flex items-center gap-4">
    <div class="w-10 h-10 rounded-xl bg-emerald-500/20 flex items-center justify-center flex-shrink-0">
      <svg class="w-5 h-5 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/>
      </svg>
    </div>
    <div>
      <div class="text-xs text-white/40 font-medium uppercase tracking-widest mb-1">Organizer</div>
      <div class="text-white font-semibold">%s</div>
      <a href="https://wa.me/%s" target="_blank" class="text-emerald-400 text-sm hover:text-emerald-300 transition-colors">
        WhatsApp: +%s
      </a>
    </div>
  </div>

  <!-- Register CTA -->
  <div class="pt-2">
    <a id="register-btn" href="/register"
       class="btn-primary w-full flex items-center justify-center gap-3 font-bold px-8 py-4 rounded-2xl text-base text-white">
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/>
      </svg>
      Register for This Event
    </a>
    <p class="text-center text-white/30 text-xs mt-3">No account required · Instant confirmation</p>
  </div>
</div>`,
		esc(ev.Title),
		esc(price),
		formatDate(ev.EventDate),
		formatTime(ev.EventDate),
		esc(ev.Location),
		esc(ev.Description),
		paymentHTML,
		esc(ev.AdminName),
		esc(ev.AdminWhatsapp),
		esc(ev.AdminWhatsapp),
	)
}
