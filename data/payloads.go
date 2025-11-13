package data

import (
	"encoding/json"
	"time"
)

type AuthUser struct {
	ID        string
	UserType  string
	RoleID    string
	ExpiresAt time.Time
}

type LoginRequest struct {
	UserID   string `json:"userId" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginSessions struct {
	Token          string            `json:"token"`
	ActiveSessions map[string]string `json:"sessions"`
}

type SignUpRequest struct {
	Email string `json:"email" binding:"required"`
	Name  string `json:"name" binding:"required"`
	Phone string `json:"phone" binding:"required"`
	Role  string `json:"role"`
	// UserID   string `json:"userId" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type DeveloperSignUpOtpVerificationRequest struct {
	OTP       string `json:"otp" binding:"required"`
	Email     string `json:"email" binding:"required"`
	RequestID string `json:"requestId" binding:"required"`
}

type CreateTicketRequest struct {
	// TicketID         string   `json:"ticketId" binding:"required"`
	// CreatedBy  string   `json:"createdBy" binding:"required"`
	AssignedTo string   `json:"assignedTo,omitempty"`
	TeamID     string   `json:"teamId,omitempty"`
	Category   []string `json:"category" binding:"required"`
	Priority   string   `json:"priority" binding:"required"`
	Status     string   `json:"status" binding:"required"`
	Tags       []string `json:"tags,omitempty"`
	// ResponseTime     int64    `json:"responseTime,omitempty"`
	// NeedsMediaUpload bool `json:"needsMediaUpload" binding:"required"`
}

type GenerateUploadURLRequest struct {
	// UploadedBy string `json:"uploadedBy" binding:"required"`
	TicketID string `json:"ticketId" binding:"required"`
	// FileType   string `json:"fileType" binding:"required"`
}

type ConfirmUploadRequest struct {
	// TicketID     string `json:"ticketId" binding:"required"`
	AttachmentID string `json:"attachmentId" binding:"required"`
	// UploadedBy   string `json:"uploadedBy" binding:"required"`
}

type AssignTicketRequest struct {
	// TicketID   string `json:"ticketId" binding:"required"`
	AssignedTo string `json:"assignedTo" binding:"required"`
	// TeamID     string `json:"teamId" binding:"required"`
}

type UpdateTicketStatusRequest struct {
	// TicketID string `json:"ticketId" binding:"required"`
	Status string `json:"status" binding:"required"`
}

type TicketEvent struct {
	EventID   string        `json:"event_id"`
	EventType string        `json:"event_type"`
	Timestamp string        `json:"timestamp"`
	Actor     Actor         `json:"actor"`
	Ticket    TicketDetails `json:"ticket"`
	// Metadata  Metadata      `json:"metadata"`
	ReminderDetails ReminderDetails `json:"reminder"`
	Changes         Changes         `json:"changes"`
}

type Actor struct {
	UserID string `json:"user_id"`
}

type TicketDetails struct {
	TicketID   string `json:"ticket_id"`
	Status     string `json:"status"`
	Priority   string `json:"priority"`
	CreatedAt  string `json:"created_at"`
	AssignedTo string `json:"assigned_to"`
}

// type Metadata struct {
// 	RequestID string `json:"request_id"`
// }

type Changes struct {
	Status     string `json:"status"`
	AssignedTo string `json:"assigned_to"`
}

type ReminderDetails struct {
	ScheduledTime string   `json:"timestamp"`
	Message       string   `json:"message"`
	Recipients    []string `json:"recipients"`
}

type DLQEvent struct {
	EventID       string      `json:"event_id"`
	OriginalEvent string      `json:"original_event"`
	Timestamp     string      `json:"timestamp"`
	Event         TicketEvent `json:"event"`
	FailureReason string      `json:"failure_reason"`
	RetryAttempts int         `json:"retry_attempts"`
}

// Reminder struct
type Reminder struct {
	Key        string          `json:"key"`
	EventData  json.RawMessage `json:"eventData"`
	RemindTime time.Time       `json:"remind_time"`
	Status     string          `json:"status"` // "pending" or "completed"
	Revision   uint64          `json:"revision"`
}

type ReminderRequest struct {
	ReminderAt time.Time `json:"reminderAt" binding:"required"` // Time in utc
	Message    string    `json:"message,omitempty"`             // Optional reminder note
	Recipients []string  `json:"recipients,omitempty"`          // Optional
}

type CancelReminderRequest struct {
	ReminderID string `json:"reminderId" binding:"required"`
}

type SnoozeReminderRequest struct {
	ReminderID     string        `json:"reminderId" binding:"required"`
	SnoozeDuration time.Duration `json:"snooze_duration"`
}
