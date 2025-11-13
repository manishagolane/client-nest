package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/goccy/go-json"

	"github.com/manishagolane/client-nest/constants"
	"github.com/manishagolane/client-nest/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func getPrettyValidationError(err error) error {
	var vError validator.ValidationErrors
	if errors.As(err, &vError) {
		for _, fieldErr := range vError {
			return fmt.Errorf("%s validation for field %s failed", fieldErr.ActualTag(), utils.ToCamelCase(fieldErr.Field()))
		}
	}
	if errors.Is(err, io.EOF) {
		return errors.New("missing request body")
	}
	if jsonError, ok := err.(*json.UnmarshalTypeError); ok {
		return fmt.Errorf("unexpected type for %s, expected: %s, received: %s",
			utils.ToCamelCase(jsonError.Field),
			jsonError.Type.String(),
			jsonError.Value)
	}
	return err
}

func sendSuccessApiResponse(data interface{}) gin.H {
	return gin.H{
		"status": constants.ApiSuccess,
		"data":   data,
	}
}

func GetTokenFromRequest(c *gin.Context, authKey string) (string, error) {
	ctx := c.Request.Context()
	logger := utils.GetCtxLogger(ctx)
	authToken, err := c.Request.Cookie(authKey)
	if err != nil {
		authToken := c.Request.Header.Get(authKey)
		if authToken == "" {
			return "", err
		}
		return authToken, nil
	} else {
		logger.Info("authToken",
			zap.String("name", authToken.Name),
			zap.String("value", authToken.Value),
			zap.String("path", authToken.Path),
			zap.String("domain", authToken.Domain),
			zap.Time("expires", authToken.Expires),
			zap.Bool("secure", authToken.Secure),
			zap.Bool("httpOnly", authToken.HttpOnly),
		)
	}

	authTokenValue, err := url.QueryUnescape(authToken.Value)
	if err != nil {
		logger.Error("Failed to decode session key", zap.Error(err))
		return "", err
	}

	return authTokenValue, nil
}

func generatePasswordHash(pwd []byte) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(pwd, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func getDeveloperSignUpFromRedisKey(email, requestId string) string {
	return fmt.Sprintf("developer_signup:%s:%s", email, requestId)
}
