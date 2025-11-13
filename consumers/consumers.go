package consumers

import (
	"github.com/manishagolane/client-nest/clients"
	"github.com/manishagolane/client-nest/models"
	"go.uber.org/zap"
)

type Consumers struct {
	TicketCreatedConsumer    *TicketCreatedConsumer
	TicketAssignedConsumer   *TicketAssignedConsumer
	TicketReassignedConsumer *TicketReassignedConsumer
	TicketReminderConsumer   *TicketReminderConsumer
}

func InitializeConsumers(logger *zap.Logger, clients *clients.Clients, queries *models.Queries) *Consumers {
	logger.Info("Initializing consumer")
	ticketCreatedConsumer := NewTicketCreatedConsumer(logger, clients, queries)

	err := ticketCreatedConsumer.StartTicketCreatedConsumer()
	if err != nil {
		logger.Error("Failed to start ticket created consumer:", zap.Error(err))
	}

	ticketAssignedConsumer := NewTicketAssignedConsumer(logger, clients, queries)

	err = ticketAssignedConsumer.StartTicketAssignedConsumer()
	if err != nil {
		logger.Error("Failed to start ticket assigned consumer:", zap.Error(err))
	}

	ticketReassignedConsumer := NewTicketReassignedConsumer(logger, clients, queries)

	err = ticketReassignedConsumer.StartTicketReassignedConsumer()
	if err != nil {
		logger.Error("Failed to start ticket reassigned consumer:", zap.Error(err))
	}

	ticketReminderConsumer := NewTicketReminderConsumer(logger, clients, queries)

	err = ticketReminderConsumer.StartTicketReminderConsumer()
	if err != nil {
		logger.Error("Failed to start ticket reminder consumer:", zap.Error(err))
	}

	return &Consumers{
		TicketCreatedConsumer:    ticketCreatedConsumer,
		TicketAssignedConsumer:   ticketAssignedConsumer,
		TicketReassignedConsumer: ticketReassignedConsumer,
		TicketReminderConsumer:   ticketReminderConsumer,
	}

}
