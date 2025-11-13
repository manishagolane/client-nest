package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/manishagolane/client-nest/clients"
	"github.com/manishagolane/client-nest/data"
	"github.com/manishagolane/client-nest/models"
	"github.com/manishagolane/client-nest/utils"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type TicketCreatedConsumer struct {
	logger  *zap.Logger
	clients *clients.Clients
	queries *models.Queries
}

// **Constructor**
func NewTicketCreatedConsumer(logger *zap.Logger, clients *clients.Clients, queries *models.Queries) *TicketCreatedConsumer {
	return &TicketCreatedConsumer{
		logger:  logger,
		clients: clients,
		queries: queries,
	}
}

// **Start Subscription**
func (tc *TicketCreatedConsumer) StartTicketCreatedConsumer() error {
	tc.logger.Info("Starting Ticket Created Consumer...")
	err := tc.clients.NATSClient.QueueSubscribe("CRM_TICKETS", "crm.tickets.created.*", "ticket_created_worker", func(msg *nats.Msg) {
		tc.logger.Info("Received Ticket Created Event!", zap.String("subject", msg.Subject), zap.String("data", string(msg.Data)))
		err := tc.processTicketCreated(msg)

		if err != nil {
			tc.logger.Error("processing failed, message will be retried", zap.Error(err))
			msg.Nak() // Negative Acknowledge (Retries will happen)
			return
		}

		msg.Ack() // Acknowledge success
	})

	if err != nil {
		tc.logger.Error("failed to subscribe to ticket.created", zap.Error(err))
		return err
	}

	return nil
}

// **Process Event**
func (tc *TicketCreatedConsumer) processTicketCreated(msg *nats.Msg) error {
	var event data.TicketEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		tc.logger.Error("failed to parse event", zap.Error(err))
		return err
	}

	tc.logger.Info("Successfully Processed Ticket Created Event!",
		zap.String("ticket_id", event.Ticket.TicketID),
		zap.String("assigned_to", event.Ticket.AssignedTo),
	)

	// Send Email/SMS notification to watchers
	ctx := context.Background()
	// Fetch Watchers' Emails
	watchers, err := tc.queries.GetWatchersEmailsAndRoles(ctx, event.Ticket.TicketID)
	if err != nil {
		tc.logger.Error("Failed to fetch watchers", zap.Error(err))
		return err
	}

	subject := fmt.Sprintf("[Ticket ID: %s] Status Update", event.Ticket.TicketID)
	message := fmt.Sprintf(
		"A new support ticket has been created.\n\nTicket ID: %s\nStatus: %s\nPriority: %s.",
		event.Ticket.TicketID, event.Ticket.Status, event.Ticket.Priority,
	)

	var failedWatchers []string

	// iterate over the watchers and send the email
	for _, watcher := range watchers {
		roleStr, ok := watcher.Role.(string)
		if !ok {
			roleStr = "unknown"
		}
		tc.logger.Info("Sending email", zap.String("recipient", watcher.Email), zap.String("role", roleStr))
		err := tc.clients.EmailClient.SendEmail(ctx, watcher.Email, subject, message)
		if err != nil {
			tc.logger.Error("failed to send email", zap.String("recipient", watcher.Email), zap.Error(err))
			failedWatchers = append(failedWatchers, watcher.Email)
		}
	}

	if len(failedWatchers) > 0 {
		err := fmt.Errorf("failed to send email to: %v", failedWatchers)
		tc.logger.Info("failed to send email", zap.Strings("recipients", failedWatchers))
		tc.moveToDLQ(msg, event, err)
		tc.logger.Error("email sending failed", zap.Error(err))

		return err
	}
	return nil
}

func (tc *TicketCreatedConsumer) moveToDLQ(msg *nats.Msg, event data.TicketEvent, err error) {
	if err == nil {
		err = fmt.Errorf("unknown failure") // Handle nil errors properly
	}

	if tc.clients.NATSClient == nil {
		tc.logger.Error("NATSClient is nil, cannot publish to DLQ")
		return
	}

	originalEvent := ""
	if msg != nil {
		originalEvent = msg.Subject
	}

	ticketID := "unknown_ticket"
	if event.Ticket.TicketID != "" {
		ticketID = event.Ticket.TicketID
	}

	tc.logger.Warn("Moving failed message to DLQ", zap.String("ticket_id", event.Ticket.TicketID), zap.Error(err))

	eventId, ulidErr := utils.GetUlid()
	if ulidErr != nil {
		tc.logger.Error("failed to generate ULID", zap.Error(ulidErr))
		return
	}

	dlqEvent := data.DLQEvent{
		EventID:       eventId,
		OriginalEvent: originalEvent,
		Timestamp:     utils.GetCurrentTime().Format(time.RFC3339),
		Event:         event,
		FailureReason: err.Error(),
		RetryAttempts: 5, // Max Deliver set in NATS
	}

	eventData, jsonErr := json.Marshal(dlqEvent)
	if jsonErr != nil {
		tc.logger.Error("failed to marshal DLQ event", zap.Error(jsonErr))
		return
	}

	subject := "crm.dlq." + ticketID
	natsErr := tc.clients.NATSClient.PublishEvent(subject, eventData)
	if natsErr != nil {
		tc.logger.Error("failed to move event to DLQ", zap.Error(natsErr))
	}
}
