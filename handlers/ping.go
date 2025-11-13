package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type PingHandler struct{}

func NewPingHandler() *PingHandler {
	return &PingHandler{}
}

func (h *PingHandler) Ping(c *gin.Context) {
	c.JSON(http.StatusOK, sendSuccessApiResponse(gin.H{
		"message": "pong",
	}))
}
