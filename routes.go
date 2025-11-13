package main

import (
	"github.com/manishagolane/client-nest/handlers"

	"github.com/gin-gonic/gin"
)

func (app *App) registerRoutes(router *gin.Engine) {
	authHandler := handlers.NewAuthHandler(app.logger, app.cache, app.lock, app.queries, app.poolConn, app.clients)
	ticketHandler := handlers.NewTicketHandler(app.queries, app.clients, app.poolConn, app.cache)

	publicRoutes := router.Group("/api")
	{
		publicRoutes.POST("/login-customer", authHandler.CustomerLogin)
		publicRoutes.POST("/login-employee", authHandler.EmployeeLogin)
		// publicRoutes.POST("/employee/signup", authHandler.EmployeeSignup)
		// publicRoutes.POST("/employee/verify-otp", authHandler.EmployeeOtpVerify)
	}

	protectedRoutes := router.Group("/api")
	protectedRoutes.Use(authMiddleware(app.cache, app.jwtManager))
	{
		protectedRoutes.POST("/logout", authHandler.Logout)
		protectedRoutes.POST("/create-ticket", ticketHandler.CreateTicket)
		protectedRoutes.POST("/generate-upload-url", ticketHandler.GeneratePresignedUploadURL)
		protectedRoutes.POST("/confirm-media-upload", ticketHandler.ConfirmS3Upload)

		protectedRoutes.POST("/tickets/:id/assign", rbacMiddleware(app.queries, app.cache, "ticket", "assign"), ticketHandler.AssignTicket)
		protectedRoutes.POST("/tickets/:id/reminder", rbacMiddleware(app.queries, app.cache, "ticket", "reminder"), ticketHandler.SetTicketReminder)
		protectedRoutes.POST("/tickets/:id/cancel-reminder", rbacMiddleware(app.queries, app.cache, "ticket", "cancel_reminder"), ticketHandler.CancelTicketReminder)
		protectedRoutes.POST("/tickets/:id/snooze-reminder", rbacMiddleware(app.queries, app.cache, "ticket", "snooze_reminder"), ticketHandler.SnoozeTicketReminderHandler)

		// protectedRoutes.POST("/tickets/:id/status", rbacMiddleware(app.queries, app.cache, "ticket", "update_status"), ticketHandler.UpdateTicketStatus)
		// protectedRoutes.POST("/tickets/:id/close", rbacMiddleware(app.queries, app.cache, "ticket", "close"), ticketHandler.CloseTicket)
		// protectedRoutes.DELETE("/tickets/:id/delete", rbacMiddleware(app.queries, app.cache, "ticket", "delete"), ticketHandler.DeleteTicket)

	}
}
