package services

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/trisfproject/gatherhub/internal/models"
	"gorm.io/gorm"
)

// NotificationProvider defines the interface for all notification channels
type NotificationProvider interface {
	Send(recipient string, message string) error
}

// NotificationService manages notification templates and dispatches them via registered channels
type NotificationService struct {
	db              *gorm.DB
	providers       map[string]NotificationProvider
	auditLogService *AuditLogService
}

// NewNotificationService creates a new NotificationService
func NewNotificationService(db *gorm.DB, auditLog *AuditLogService) *NotificationService {
	mock := NewMockProvider(db, auditLog)
	return &NotificationService{
		db:              db,
		auditLogService: auditLog,
		providers: map[string]NotificationProvider{
			"whatsapp": mock,
			"email":    mock,
			"telegram": mock,
			"webhook":  mock,
		},
	}
}

// SetProvider allows dynamically updating a provider for a specific channel
func (s *NotificationService) SetProvider(channel string, provider NotificationProvider) {
	s.providers[strings.ToLower(channel)] = provider
}

// Templates mapping
var notificationTemplates = map[string]string{
	"SUBMITTED": `Halo {{.Participant.FullName}}, pendaftaran Anda untuk acara "{{.Event.Title}}" telah kami terima dengan nomor registrasi {{.Participant.RegistrationNumber}}. Status pendaftaran Anda saat ini adalah PENDING. Silakan tunggu proses verifikasi pembayaran oleh admin.`,
	"VERIFIED":  `Halo {{.Participant.FullName}}, selamat! Pendaftaran Anda untuk acara "{{.Event.Title}}" dengan nomor registrasi {{.Participant.RegistrationNumber}} telah VERIFIKASI & DISETUJUI. Terima kasih atas partisipasi Anda!`,
	"REJECTED":  `Halo {{.Participant.FullName}}, mohon maaf pendaftaran Anda untuk acara "{{.Event.Title}}" dengan nomor registrasi {{.Participant.RegistrationNumber}} telah DITOLAK. Silakan hubungi admin untuk informasi lebih lanjut.`,
}

// SendNotification formats and triggers notifications across all registered channels
func (s *NotificationService) SendNotification(p *models.Participant, eventType string) error {
	tmplStr, ok := notificationTemplates[strings.ToUpper(eventType)]
	if !ok {
		return fmt.Errorf("unknown notification event type: %s", eventType)
	}

	tmpl, err := template.New("notif").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	data := struct {
		Participant *models.Participant
		Event       models.Event
	}{
		Participant: p,
		Event:       p.Event,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	message := buf.String()

	// Dispatch to active channels asynchronously
	go func() {
		// WhatsApp
		if p.Phone != "" {
			if prov, ok := s.providers["whatsapp"]; ok {
				if err := prov.Send(p.Phone, message); err != nil {
					log.Printf("Failed to send WhatsApp notif to %s: %v", p.Phone, err)
				}
			}
		}

		// Email
		if p.Email != "" {
			if prov, ok := s.providers["email"]; ok {
				if err := prov.Send(p.Email, message); err != nil {
					log.Printf("Failed to send Email notif to %s: %v", p.Email, err)
				}
			}
		}

		// Telegram
		if p.TelegramUsername != "" {
			tgRecipient := p.TelegramUsername
			if !strings.HasPrefix(tgRecipient, "@") {
				tgRecipient = "@" + tgRecipient
			}
			if prov, ok := s.providers["telegram"]; ok {
				if err := prov.Send(tgRecipient, message); err != nil {
					log.Printf("Failed to send Telegram notif to %s: %v", tgRecipient, err)
				}
			}
		}

		// Webhook
		webhookURL := "https://api.gatherhub.local/webhook"
		if prov, ok := s.providers["webhook"]; ok {
			if err := prov.Send(webhookURL, message); err != nil {
				log.Printf("Failed to send Webhook notif: %v", err)
			}
		}
	}()

	return nil
}

// GetAllLogs retrieves all notification logs ordered by created_at DESC
func (s *NotificationService) GetAllLogs() ([]models.NotificationLog, error) {
	var logs []models.NotificationLog
	err := s.db.Order("created_at DESC").Find(&logs).Error
	return logs, err
}

// ─────────────────────── MockProvider ───────────────────────

type MockProvider struct {
	db              *gorm.DB
	auditLogService *AuditLogService
}

func NewMockProvider(db *gorm.DB, auditLog *AuditLogService) *MockProvider {
	return &MockProvider{db: db, auditLogService: auditLog}
}

func (p *MockProvider) Send(recipient, message string) error {
	log.Printf("[MOCK NOTIFICATION] Recipient: %s, Message: %s", recipient, message)

	channel := determineChannel(recipient)

	// Look up participant ID and event ID to satisfy logs requirements
	var participant models.Participant
	var participantID uint
	var eventID uint

	// Extract registration number (e.g. GH-2026-0015)
	regNum := ""
	re := regexp.MustCompile(`GH-\d{4}-\d{4}`)
	if match := re.FindString(message); match != "" {
		regNum = match
	}

	var err error
	if regNum != "" {
		err = p.db.Where("registration_number = ?", regNum).First(&participant).Error
	} else {
		err = p.db.Where("email = ? OR phone = ? OR telegram_username = ? OR ('@' || telegram_username) = ?", recipient, recipient, recipient, recipient).
			Order("created_at DESC").
			First(&participant).Error
	}

	if err == nil {
		participantID = participant.ID
		eventID = participant.EventID
	} else {
		// Fallback: if there's at least one participant in the DB, link it to prevent foreign key errors.
		var firstPart models.Participant
		if p.db.First(&firstPart).Error == nil {
			participantID = firstPart.ID
			eventID = firstPart.EventID
		} else {
			log.Printf("No participants found in database to link to notification log.")
		}
	}

	logEntry := models.NotificationLog{
		ParticipantID: participantID,
		EventID:       eventID,
		Recipient:     recipient,
		Channel:       channel,
		Message:       message,
		Status:        "SUCCESS",
		CreatedAt:     time.Now(),
	}

	if err := p.db.Create(&logEntry).Error; err != nil {
		log.Printf("Failed to save notification log: %v", err)
		if p.auditLogService != nil {
			_ = p.auditLogService.Log("system", "FAILED", "NOTIFICATION", 0, nil, map[string]interface{}{
				"recipient": recipient,
				"channel":   channel,
				"error":     err.Error(),
				"message":   message,
			}, "", "")
		}
		return err
	}

	if p.auditLogService != nil {
		if err := p.auditLogService.Log("system", "SENT", "NOTIFICATION", logEntry.ID, nil, logEntry, "", ""); err != nil {
			log.Printf("Warning: failed to write notification audit log: %v", err)
		}
	}

	return nil
}

func determineChannel(recipient string) string {
	if strings.Contains(recipient, "@") && !strings.HasPrefix(recipient, "@") {
		return "EMAIL"
	}
	if strings.HasPrefix(recipient, "http://") || strings.HasPrefix(recipient, "https://") {
		return "WEBHOOK"
	}
	if strings.HasPrefix(recipient, "@") {
		return "TELEGRAM"
	}
	return "WHATSAPP"
}

// ─────────────────────── Skeletons for Future Providers ───────────────────────

type FonnteProvider struct {
	ApiKey string
}

func (p *FonnteProvider) Send(recipient, message string) error {
	log.Printf("[Fonnte] Sending WhatsApp message to %s (API Key configured: %v)", recipient, p.ApiKey != "")
	// Future implementation will invoke Fonnte API endpoint
	return nil
}

type WablasProvider struct {
	ApiKey string
	Domain string
}

func (p *WablasProvider) Send(recipient, message string) error {
	log.Printf("[Wablas] Sending WhatsApp message to %s via domain %s", recipient, p.Domain)
	// Future implementation will invoke Wablas API endpoint
	return nil
}

type SMTPProvider struct {
	Host string
	Port int
	User string
	Pass string
}

func (p *SMTPProvider) Send(recipient, message string) error {
	log.Printf("[SMTP] Sending Email to %s via host %s:%d", recipient, p.Host, p.Port)
	// Future implementation will configure net/smtp
	return nil
}

type TelegramBotProvider struct {
	BotToken string
	ChatID   string
}

func (p *TelegramBotProvider) Send(recipient, message string) error {
	tokenDesc := "none"
	if len(p.BotToken) > 5 {
		tokenDesc = p.BotToken[:5] + "..."
	}
	log.Printf("[Telegram Bot] Sending Telegram to %s (Token prefix: %s)", recipient, tokenDesc)
	// Future telegram API request using BotToken
	return nil
}

type WebhookProvider struct {
	URL string
}

func (p *WebhookProvider) Send(recipient, message string) error {
	log.Printf("[Webhook] Dispatching webhook payload to %s", recipient)
	// Future POST request with JSON payload
	return nil
}
