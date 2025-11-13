package handlers

import (
	"errors"
	"net/http"

	"github.com/manishagolane/client-nest/config"
	"github.com/manishagolane/client-nest/constants"

	"github.com/gin-gonic/gin"
)

type RequestError struct {
	RawError     error            `json:"-"`
	ErrorMessage string           `json:"message"`
	Status       constants.Status `json:"status"`
	ErrorCode    int              `json:"errorCode"`
	ErrorData    gin.H            `json:"errorData,omitempty"`
}

type APIResponse struct {
	Status  constants.Status `json:"status"`
	Message string           `json:"message"`
	Data    interface{}      `json:"data,omitempty"`
}

func SendForbiddenError(c *gin.Context, err error) {
	response := RequestError{
		Status:       constants.ApiFailure,
		ErrorMessage: err.Error(),
		ErrorCode:    http.StatusForbidden,
	}
	c.AbortWithStatusJSON(http.StatusForbidden, response)
}

func SendSuccessResponse(c *gin.Context, message string, data interface{}) {
	response := APIResponse{
		Status:  constants.ApiSuccess,
		Message: message,
		Data:    data,
	}
	c.JSON(http.StatusOK, response)
}

func SendBadRequestError(c *gin.Context, err error) {
	response := RequestError{
		Status:       constants.ApiFailure,
		ErrorMessage: getPrettyValidationError(err).Error(),
		ErrorCode:    http.StatusBadRequest,
	}
	c.AbortWithStatusJSON(http.StatusBadRequest, response)
}

func SendApplicationError(c *gin.Context, err error) {
	environment := config.GetString("environment")
	response := RequestError{
		Status:       constants.ApiFailure,
		ErrorMessage: "unable to process request",
		ErrorCode:    http.StatusInternalServerError,
	}
	if environment != "production" {
		response.ErrorMessage = err.Error()
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, response)
}

func SendUnauthorizedError(c *gin.Context, err error) {
	response := RequestError{
		Status:       constants.ApiFailure,
		ErrorMessage: err.Error(),
		ErrorCode:    http.StatusUnauthorized,
	}
	c.AbortWithStatusJSON(http.StatusUnauthorized, response)
}

func SendUnProcessableRequestError(c *gin.Context, err error) {
	response := RequestError{
		Status:       constants.ApiFailure,
		ErrorMessage: err.Error(),
		ErrorCode:    http.StatusUnprocessableEntity,
	}
	c.AbortWithStatusJSON(http.StatusUnprocessableEntity, response)
}

func HandleOTPValidationError(c *gin.Context, err error) {
	if errors.Is(err, constants.ErrorInvalidOTP) {
		SendBadRequestError(c, err)
	} else if errors.Is(err, constants.ErrorMaxAttempts) {
		SendBadRequestError(c, err)
	} else if errors.Is(err, constants.ErrorInvalidRequest) {
		SendBadRequestError(c, err)
	} else if errors.Is(err, constants.ErrorOtpExpired) {
		SendUnProcessableRequestError(c, err)
	} else {
		SendApplicationError(c, err)
	}
}
