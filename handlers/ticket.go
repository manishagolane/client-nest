package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/manishagolane/client-nest/cache"
	"github.com/manishagolane/client-nest/clients"
	"github.com/manishagolane/client-nest/data"
	"github.com/manishagolane/client-nest/models"
	"github.com/manishagolane/client-nest/utils"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/zap"
)

type TicketHandler struct {
	queries *models.Queries
	client  *clients.Clients
	conn    *pgxpool.Pool
	cache   *cache.Cache
}

func NewTicketHandler(queries *models.Queries, client *clients.Clients, conn *pgxpool.Pool, cache *cache.Cache) *TicketHandler {
	return &TicketHandler{
		queries: queries,
		client:  client,
		conn:    conn,
		cache:   cache,
	}
}

func (th *TicketHandler) CreateTicket(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.GetCtxLogger(ctx)

	var req data.CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		SendBadRequestError(c, getPrettyValidationError(err))
		return
	}

	loggedInUser, loggedInUserType, err := utils.GetLoggedInUser(c)
	if err != nil {
		SendUnauthorizedError(c, errors.New("unauthorized"))
		return
	}

	tx, err := th.conn.Begin(ctx)
	if err != nil {
		logger.Error("unable to begin transaction", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}
	deferredRollback := true

	defer func() {
		if deferredRollback {
			if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
				// Handle the rollback error here
				logger.Error("unable to rollback transaction", zap.Error(err))
				SendApplicationError(c, errors.New("internal server error"))
				return
			}
		}
	}()

	queriesWithTx := th.queries.WithTx(tx)

	watchers := []string{loggedInUser}

	// Add admin to watchers (if applicable)
	adminIDs, err := queriesWithTx.GetAdminIDs(ctx)
	if err != nil {
		logger.Error("failed to fetch admin IDs", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	} else {
		watchers = append(watchers, adminIDs...)
	}

	// Convert watchers list to JSON
	watchersJSON, err := json.Marshal(watchers)
	if err != nil {
		logger.Error("failed to marshal watchers JSON", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	ticketId, err := utils.GetUlid()
	if err != nil {
		logger.Error("failed to generate ULID", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	params := models.CreateTicketParams{
		ID:            ticketId,
		CreatedBy:     loggedInUser,
		CreatedByType: utils.ToNullString(loggedInUserType),
		AssignedTo:    utils.ToNullString(req.AssignedTo),
		TeamID:        utils.ToNullString(req.TeamID),
		Status:        strings.ToLower(req.Status),
		Category:      req.Category,
		Priority:      strings.ToLower(req.Priority),
		Tags:          req.Tags,
		Watchers:      pgtype.JSONB{Bytes: watchersJSON, Status: pgtype.Present},
		ResponseTime:  sql.NullInt64{Valid: false},
	}

	// Create Ticket
	ticket, err := queriesWithTx.CreateTicket(ctx, params)
	if err != nil {
		logger.Error("failed to create ticket", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	// Commit Transaction
	err = tx.Commit(ctx)
	if err != nil {
		logger.Error("transaction commit failed", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}
	deferredRollback = false
	// logger.Info("watchersList", zap.Reflect("watchersList", watchersList))

	eventId, err := utils.GetUlid()
	if err != nil {
		logger.Error("failed to generate ULID", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	// Publish Event to NATS JetStream
	ticketCreatedEvent := data.TicketEvent{
		EventID:   eventId,
		EventType: "ticket_created",
		Timestamp: utils.GetCurrentTime().Format(time.RFC3339),
		Actor: data.Actor{
			UserID: loggedInUser,
		},
		Ticket: data.TicketDetails{
			TicketID:   ticket.ID,
			Status:     ticket.Status,
			Priority:   ticket.Priority,
			CreatedAt:  ticket.CreatedAt.Time.Format(time.RFC3339),
			AssignedTo: ticket.AssignedTo.String,
		},
	}

	eventData, _ := json.Marshal(ticketCreatedEvent)
	subject := "crm.tickets.created." + ticket.ID

	err = th.client.NATSClient.PublishEvent(subject, eventData)
	if err != nil {
		logger.Error("failed to publish ticket.created event", zap.Error(err))
	}

	SendSuccessResponse(c, "Ticket created successfully", ticket.ID)
}

func (th *TicketHandler) GeneratePresignedUploadURL(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.GetCtxLogger(ctx)

	var req data.GenerateUploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		SendBadRequestError(c, getPrettyValidationError(err))
		return
	}

	attachmentId, err := utils.GetUlid()
	if err != nil {
		logger.Error("failed to generate ULID", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	// Generate S3 Key for file storage
	s3Key := fmt.Sprintf("attachments/%s/%s", req.TicketID, attachmentId)
	logger.Info("s3Key", zap.String("s3Key", s3Key))

	// Generate Pre-Signed URL
	// uploadUrl, err := th.s3Client.GeneratePresignedURL(ctx, s3Key, "PUT", time.Minute*10)
	uploadUrl, err := th.client.S3Client.GeneratePresignedURL(ctx, s3Key, "PUT", time.Hour*1)
	if err != nil {
		logger.Error("failed to generate pre-signed URL", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	// TODO cache, remove revoked token, file size
	attachmentIdKey := fmt.Sprintf("attachmentIdKey:%s", attachmentId)
	err = th.cache.Set(ctx, attachmentIdKey, "", time.Minute*60)
	if err != nil {
		logger.Error("error while setting cache")
		SendUnauthorizedError(c, errors.New("internal server error"))
		return
	}

	s3Id := fmt.Sprintf("s3Id:%s", attachmentId)
	err = th.cache.Set(ctx, s3Id, s3Key, time.Minute*60)
	if err != nil {
		logger.Error("error while setting cache")
		SendUnauthorizedError(c, errors.New("internal server error"))
		return
	}

	loggedInUser, loggedInUserType, err := utils.GetLoggedInUser(c)
	if err != nil {
		SendUnauthorizedError(c, errors.New("unauthorized"))
		return
	}

	// Insert into `ticket_attachments`
	params := models.InsertTicketAttachmentParams{
		ID:           attachmentId,
		TicketID:     utils.ToNullString(req.TicketID),
		UploadedBy:   loggedInUser,
		UploaderType: utils.ToNullString(loggedInUserType), // Convert to sql.NullString
	}

	_, err = th.queries.InsertTicketAttachment(ctx, params)
	if err != nil {
		logger.Error("failed to store attachment", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	SendSuccessResponse(c, "Presigned URL generated successfully", gin.H{"uploadUrl": uploadUrl, "attachementId": attachmentId})
}

func (th *TicketHandler) ConfirmS3Upload(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.GetCtxLogger(ctx)

	var req data.ConfirmUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		SendBadRequestError(c, getPrettyValidationError(err))
		return
	}

	attachmentKey := fmt.Sprintf("attachmentIdKey:%s", req.AttachmentID)
	_, err := th.cache.Get(ctx, attachmentKey)
	if err != nil {
		logger.Error("token not found in Redis, possibly already logged out or expired")
		SendUnauthorizedError(c, errors.New("invalid session"))
		return
	}

	s3Key := fmt.Sprintf("s3Id:%s", req.AttachmentID)
	// logger.Info("s3Key", zap.String("s3Key", s3Key))
	storedS3Key, err := th.cache.Get(ctx, s3Key)
	if err != nil {
		logger.Error("storedS3Key not found in Redis, possibly already logged out or expired")
		SendUnauthorizedError(c, errors.New("invalid session"))
		return
	}

	// Update  `ticket_attachments`
	params := models.UpdateTickeAttachmentFileUrlParams{
		ID:      req.AttachmentID,
		FileUrl: utils.ToNullString(storedS3Key),
	}

	err = th.queries.UpdateTickeAttachmentFileUrl(ctx, params)
	if err != nil {
		logger.Error("failed to store attachment", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	th.cache.Delete(ctx, s3Key)
	th.cache.Delete(ctx, attachmentKey)

	SendSuccessResponse(c, "File uploaded successfully", nil)
}

// Manual Assignment: Admin/Manager assigns a ticket to a employee
func (th *TicketHandler) AssignTicket(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.GetCtxLogger(ctx)

	var req data.AssignTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		SendBadRequestError(c, errors.New("invalid request payload"))
		return
	}

	ticketID := c.Param("id")
	if ticketID == "" {
		logger.Error("ticket id is missing in URL")
		SendBadRequestError(c, errors.New("ticket ID is required"))
		return
	}

	loggedInUser, userType, err := utils.GetLoggedInUser(c)
	if err != nil {
		SendUnauthorizedError(c, errors.New("unauthorized"))
		return
	}

	tx, err := th.conn.Begin(ctx)
	if err != nil {
		logger.Error("failed to start transaction", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}
	deferredRollback := true

	defer func() {
		if deferredRollback {
			if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
				// Handle the rollback error here
				logger.Error("unable to rollback transaction", zap.Error(err))
				SendApplicationError(c, errors.New("internal server error"))
				return
			}
		}
	}()

	queriesWithTx := th.queries.WithTx(tx)

	assignedEmployee, err := queriesWithTx.GetEmployeeById(ctx, req.AssignedTo)
	if err != nil {
		logger.Error("employee doesn't exist", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	// fetch existing ticket details
	ticket, err := queriesWithTx.GetTicketByID(ctx, ticketID)
	if err != nil {
		logger.Error("failed to fetch ticket details", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	if ticket.Status == "resolved" || ticket.Status == "closed" {
		logger.Error("cannot assign a closed or resolved ticket", zap.String("ticket_id", ticketID))
		SendForbiddenError(c, errors.New("can't assign a closed or resolved ticket"))
		return
	}

	// If manager, ensure the assigned employee belongs to their team
	if userType == "manager" {
		manager, err := queriesWithTx.GetEmployeeById(ctx, loggedInUser)
		if err != nil {
			logger.Error("failed to fetch manager details", zap.Error(err))
			SendApplicationError(c, errors.New("internal server error"))
			return
		}

		if assignedEmployee.TeamID.String != manager.TeamID.String {
			logger.Error("managers can only assign tickets to their own team", zap.String("manager_id", loggedInUser))
			SendForbiddenError(c, errors.New("you can only assign tickets within your team"))
			return
		}
	}

	// determine event type based on reassignment
	eventType := "ticket_assigned"
	subject := "crm.tickets.assigned." + ticket.ID

	if ticket.AssignedTo.Valid {
		if ticket.AssignedTo.String == req.AssignedTo {
			logger.Info("Ticket is already assigned to the same user, no reassignment needed")
			SendSuccessResponse(c, "Ticket is already assigned to this user", ticketID)
			return
		}
		eventType = "ticket_reassigned"
		subject = "crm.tickets.reassigned." + ticket.ID
	}

	// Update Ticket in Database
	updatePramas := models.UpdateTicketAssignmentParams{
		ID:         ticketID,
		AssignedTo: utils.ToNullString(req.AssignedTo),
		TeamID:     utils.ToNullString(assignedEmployee.TeamID.String),
	}

	err = queriesWithTx.UpdateTicketAssignment(ctx, updatePramas)
	if err != nil {
		logger.Error("failed to assign the ticket", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	// Update Watchers
	var watchers []string
	if err := json.Unmarshal(ticket.Watchers.Bytes, &watchers); err != nil {
		logger.Error("failed to parse watchers list", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	// Add Assigned Employee to watchers
	if !utils.Contains(watchers, req.AssignedTo) {
		watchers = append(watchers, req.AssignedTo)
	}

	// Add their manager to watchers
	if assignedEmployee.ManagerID.Valid && !utils.Contains(watchers, assignedEmployee.ManagerID.String) {
		watchers = append(watchers, assignedEmployee.ManagerID.String)
	}

	watchersJSON, err := json.Marshal(watchers)
	if err != nil {
		logger.Error("failed to marshal watchers JSON", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	// Update watchers list in the database
	updateWatchersParams := models.UpdateWachersListParams{
		ID:       ticketID,
		Watchers: pgtype.JSONB{Bytes: watchersJSON, Status: pgtype.Present},
	}
	err = queriesWithTx.UpdateWachersList(ctx, updateWatchersParams)
	if err != nil {
		logger.Error("failed to update watchers", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	// Commit Transaction
	err = tx.Commit(ctx)
	if err != nil {
		logger.Error("transaction commit failed", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}
	deferredRollback = false

	// Publish `ticket.assigned` event

	eventId, err := utils.GetUlid()
	if err != nil {
		logger.Error("failed to generate ULID", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	// Publish Event to NATS JetStream
	ticketCreatedEvent := data.TicketEvent{
		EventID:   eventId,
		EventType: eventType,
		Timestamp: utils.GetCurrentTime().Format(time.RFC3339),
		Actor: data.Actor{
			UserID: loggedInUser,
		},
		Ticket: data.TicketDetails{
			TicketID:   ticket.ID,
			Status:     ticket.Status,
			Priority:   ticket.Priority,
			CreatedAt:  ticket.CreatedAt.Time.Format(time.RFC3339),
			AssignedTo: ticket.AssignedTo.String,
		},
		Changes: data.Changes{
			AssignedTo: req.AssignedTo,
		},
	}

	eventData, _ := json.Marshal(ticketCreatedEvent)
	err = th.client.NATSClient.PublishEvent(subject, eventData)
	if err != nil {
		logger.Error("failed to publish event", zap.Error(err), zap.String("event type:", eventType))
	}

	message := "Ticket assigned successfully"
	if eventType == "ticket_reassigned" {
		message = "Ticket reassigned successfully"
	}
	SendSuccessResponse(c, message, ticketID)
}

func (th *TicketHandler) SetTicketReminder(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.GetCtxLogger(ctx)

	var req data.ReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		SendBadRequestError(c, errors.New("invalid request payload"))
		return
	}

	ticketID := c.Param("id")
	if ticketID == "" {
		logger.Error("ticket id is missing in URL")
		SendBadRequestError(c, errors.New("ticket ID is required"))
		return
	}

	reminderTimeUTC := req.ReminderAt.UTC()

	// Ensure reminder is set in the future
	if reminderTimeUTC.Before(time.Now().UTC()) {
		SendBadRequestError(c, errors.New("reminder time must be in the future"))
		return
	}

	// fetch existing ticket details
	ticket, err := th.queries.GetTicketByID(ctx, ticketID)
	if err != nil {
		logger.Error("failed to fetch ticket details", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	loggedInUser, _, err := utils.GetLoggedInUser(c)
	if err != nil {
		SendUnauthorizedError(c, errors.New("unauthorized"))
		return
	}

	// Publish `ticket.reminder` event
	eventId, err := utils.GetUlid()
	if err != nil {
		logger.Error("failed to generate ULID", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	recipients := req.Recipients
	if len(recipients) == 0 || !utils.Contains(recipients, loggedInUser) {
		recipients = append(recipients, loggedInUser)
	}

	// Publish Event to NATS JetStream
	ticketReminderEvent := data.TicketEvent{
		EventID:   eventId,
		EventType: "ticket.reminder",
		Timestamp: utils.GetCurrentTime().Format(time.RFC3339),
		Actor: data.Actor{
			UserID: loggedInUser,
		},
		Ticket: data.TicketDetails{
			TicketID:   ticket.ID,
			Status:     ticket.Status,
			Priority:   ticket.Priority,
			CreatedAt:  ticket.CreatedAt.Time.Format(time.RFC3339),
			AssignedTo: ticket.AssignedTo.String,
		},
		ReminderDetails: data.ReminderDetails{
			ScheduledTime: reminderTimeUTC.Format(time.RFC3339),
			Message:       req.Message,
			Recipients:    recipients,
		},
	}

	// subject := "crm.tickets.reminder." + ticket.ID
	// Generate Unique Key for Each User's Reminder
	reminderKey := fmt.Sprintf("reminder_%s_%s", ticketID, loggedInUser)

	eventData, _ := json.Marshal(ticketReminderEvent)

	err = th.client.NATSClient.StoreReminderInKV(ctx, reminderKey, reminderTimeUTC, eventData)
	if err != nil {
		logger.Error("Failed to store reminder in KV Store", zap.Error(err))
		SendApplicationError(c, errors.New("failed to save reminder"))
		return
	}

	// err = th.client.NATSClient.PublishDelayedEvent(subject, eventData, reminderTime)
	// if err != nil {
	// 	logger.Error("failed to publish reminder event", zap.Error(err))
	// 	SendApplicationError(c, errors.New("failed to schedule reminder"))
	// 	return
	// }

	SendSuccessResponse(c, "Reminder set successfully", gin.H{"reminderKey": reminderKey})
}

func (th *TicketHandler) CancelTicketReminder(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.GetCtxLogger(ctx)

	var req data.CancelReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		SendBadRequestError(c, errors.New("invalid request payload"))
		return
	}

	ticketID := c.Param("id")
	if ticketID == "" {
		logger.Error("ticket id is missing in URL")
		SendBadRequestError(c, errors.New("ticket ID is required"))
		return
	}

	// fetch existing ticket details
	ticket, err := th.queries.GetTicketByID(ctx, ticketID)
	if err != nil {
		logger.Error("failed to fetch ticket details", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	loggedInUser, _, err := utils.GetLoggedInUser(c)
	if err != nil {
		SendUnauthorizedError(c, errors.New("unauthorized"))
		return
	}

	// Check if the reminder exists
	reminderEntry, err := th.client.NATSClient.KV.Get(ctx, req.ReminderID)
	if err != nil {
		logger.Error("failed to fetch reminder from KV store", zap.Error(err))
		SendForbiddenError(c, errors.New("reminder not found"))
		return
	}

	// Publish `ticket.reminder` event
	eventId, err := utils.GetUlid()
	if err != nil {
		logger.Error("failed to generate ULID", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	// Publish Event to NATS JetStream
	cancelEvent := data.TicketEvent{
		EventID:   eventId,
		EventType: "ticket.reminder.cancel",
		Timestamp: utils.GetCurrentTime().Format(time.RFC3339),
		Actor: data.Actor{
			UserID: loggedInUser,
		},
		Ticket: data.TicketDetails{
			TicketID: ticket.ID,
		},
		Changes: data.Changes{
			Status: "cancel reminder",
		},
	}

	// subject := "crm.tickets.reminder." + ticket.ID
	// Generate Unique Key for Each User's Reminder
	// reminderKey := fmt.Sprintf("reminder_%s_%s", ticketID, loggedInUser)

	eventData, err := json.Marshal(cancelEvent)
	if err != nil {
		logger.Error("failed to update reminder status:", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	var reminder data.Reminder
	err = json.Unmarshal(reminderEntry.Value(), &reminder)
	if err != nil {
		logger.Error("failed to unmarshal reminder from KV store", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	err = th.client.NATSClient.CancelReminder(ctx, req.ReminderID, eventData, reminder)
	if err != nil {
		logger.Error("Failed to store reminder in KV Store", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	SendSuccessResponse(c, "Reminder canceled successfully", ticketID)
}

func (th *TicketHandler) SnoozeTicketReminderHandler(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.GetCtxLogger(ctx)

	var req data.SnoozeReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("invalid request", zap.Error(err))
		SendBadRequestError(c, errors.New("invalid request payload"))
		return
	}

	ticketID := c.Param("id")
	if ticketID == "" {
		logger.Error("ticket id is missing in URL")
		SendBadRequestError(c, errors.New("ticket ID is required"))
		return
	}

	// fetch existing ticket details
	_, err := th.queries.GetTicketByID(ctx, ticketID)
	if err != nil {
		logger.Error("failed to fetch ticket details", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	_, _, err = utils.GetLoggedInUser(c)
	if err != nil {
		SendUnauthorizedError(c, errors.New("unauthorized"))
		return
	}

	// Check if the reminder exists
	_, err = th.client.NATSClient.KV.Get(ctx, req.ReminderID)
	if err != nil {
		logger.Error("failed to fetch reminder from KV store", zap.Error(err))
		SendForbiddenError(c, errors.New("reminder not found"))
		return
	}

	// Call SnoozeReminder function
	err = th.client.NATSClient.SnoozeReminder(c.Request.Context(), req.ReminderID, req.SnoozeDuration)
	if err != nil {
		logger.Error("failed to snooze reminder", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reminder snoozed successfully"})
}
