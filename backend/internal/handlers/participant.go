package handlers

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gatherhub/backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

// ParticipantHandler handles participant registration HTTP requests
type ParticipantHandler struct {
	participantService *services.ParticipantService
	eventService       *services.EventService
}

// NewParticipantHandler creates a new ParticipantHandler
func NewParticipantHandler(
	participantService *services.ParticipantService,
	eventService *services.EventService,
) *ParticipantHandler {
	return &ParticipantHandler{
		participantService: participantService,
		eventService:       eventService,
	}
}

// Register handles POST /api/v1/events/:id/register
// Accepts multipart/form-data with participant fields + payment_proof file
func (h *ParticipantHandler) Register(c *fiber.Ctx) error {
	// Parse event ID
	eventID, err := c.ParamsInt("id")
	if err != nil || eventID <= 0 {
		return h.errorResponse(c, "Invalid event ID")
	}

	// Validate event exists
	event, err := h.eventService.GetByID(uint(eventID))
	if err != nil {
		return h.errorResponse(c, fmt.Sprintf("Event not found: %s", err.Error()))
	}

	// Parse multipart form
	form := new(services.RegisterForm)
	if err := c.BodyParser(form); err != nil {
		return h.errorResponse(c, "Failed to parse form data: "+err.Error())
	}

	// Validate form fields
	if errs := form.Validate(); len(errs) > 0 {
		return h.validationErrorResponse(c, errs)
	}

	// Handle payment proof upload
	file, err := c.FormFile("payment_proof")
	if err != nil {
		return h.errorResponse(c, "Payment proof is required. Please upload an image or PDF.")
	}

	paymentFilename, err := h.participantService.SavePaymentProof(file)
	if err != nil {
		return h.errorResponse(c, err.Error())
	}

	// Save participant to database
	participant, err := h.participantService.Create(uint(eventID), form, paymentFilename)
	if err != nil {
		return h.errorResponse(c, "Registration failed: "+err.Error())
	}

	// Build redirect URL with query params
	params := url.Values{}
	params.Set("id", strconv.Itoa(int(participant.ID)))
	params.Set("name", participant.FullName)
	params.Set("event", event.Title)
	params.Set("status", string(participant.Status))
	redirectURL := "/success.html?" + params.Encode()

	// HTMX request: set HX-Redirect header
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", redirectURL)
		return c.SendStatus(fiber.StatusOK)
	}

	// Standard request: HTTP redirect
	return c.Redirect(redirectURL, fiber.StatusSeeOther)
}

// errorResponse returns an HTMX-compatible error HTML fragment
func (h *ParticipantHandler) errorResponse(c *fiber.Ctx, message string) error {
	if c.Get("HX-Request") == "true" {
		c.Set("Content-Type", "text/html")
		return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf(`
<div id="form-error" class="glass-card border border-red-500/30 bg-red-500/10 rounded-2xl p-4 flex items-start gap-3">
  <svg class="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
  </svg>
  <div>
    <p class="text-red-300 font-semibold text-sm">Registration Error</p>
    <p class="text-red-400/80 text-sm mt-0.5">%s</p>
  </div>
</div>`, escapeHTML(message)))
	}
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": message})
}

// validationErrorResponse returns a list of validation errors as HTML
func (h *ParticipantHandler) validationErrorResponse(c *fiber.Ctx, errs []string) error {
	if c.Get("HX-Request") == "true" {
		items := ""
		for _, e := range errs {
			items += fmt.Sprintf(`<li>%s</li>`, escapeHTML(e))
		}
		c.Set("Content-Type", "text/html")
		return c.Status(fiber.StatusUnprocessableEntity).SendString(fmt.Sprintf(`
<div id="form-error" class="glass-card border border-amber-500/30 bg-amber-500/10 rounded-2xl p-4 flex items-start gap-3">
  <svg class="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/>
  </svg>
  <div>
    <p class="text-amber-300 font-semibold text-sm">Please fix the following:</p>
    <ul class="text-amber-400/80 text-sm mt-1 list-disc list-inside space-y-0.5">%s</ul>
  </div>
</div>`, items))
	}
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"errors": errs})
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
