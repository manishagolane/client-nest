package utils

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"mime"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"

	"github.com/manishagolane/client-nest/constants"
	"github.com/manishagolane/client-nest/logger"

	ulid "github.com/oklog/ulid/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

// Allowed MIME types for security
var allowedMimeTypes = map[string]bool{
	"image/jpeg":         true,
	"image/png":          true,
	"video/mp4":          true,
	"application/pdf":    true,
	"application/msword": true,
}

func GetEndOfDayTime() time.Time {
	location, _ := time.LoadLocation(constants.TIMEZONE)
	year, month, day := time.Now().In(location).Date()
	return time.Date(year, month, day, 23, 59, 59, 0, location)
}

func GetExpiryTime(days int) time.Time {
	location, _ := time.LoadLocation(constants.TIMEZONE)
	expiry := time.Now().In(location).Add(time.Duration(days) * 24 * time.Hour)
	return time.Date(expiry.Year(), expiry.Month(), expiry.Day(), 23, 59, 59, 0, location)
}

func GetLogger() *zap.Logger {
	return logger.GetLogger()
}

func GetCtxLogger(ctx context.Context) *zap.Logger {
	lgr := GetLogger()

	// Add requestID if present
	if requestID, ok := ctx.Value(constants.REQUEST_ID).(string); ok {
		lgr = lgr.With(zap.String("requestID", requestID))
	}

	// Add userID if present
	if userID, ok := ctx.Value(constants.UserID).(string); ok {
		lgr = lgr.With(zap.String(string(constants.UserID), userID))
	}

	return lgr
}

func ToCamelCase(value string) string {
	firstLetter := string(value[0])
	restOfTheString := value[1:]
	return fmt.Sprintf("%s%s", strings.ToLower(firstLetter), restOfTheString)
}

func GetCtxRequestID(ctx context.Context) string {
	return ctx.Value(constants.REQUEST_ID).(string)
}

func GetGrpcLogger(ctx context.Context) *zap.Logger {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return logger.GetLogger()
	}

	values := md.Get(string(constants.REQUEST_ID))
	if len(values) == 0 {
		return logger.GetLogger()
	}
	return logger.GetLogger().With(zap.String("requestID", values[0]))
}

func ToNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func ExtractUserIDFromTokenKey(tokenString string) string {

	parts := strings.Split(tokenString, ".")
	if len(parts) < 2 {
		return ""
	}

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1]) // Decode JWT payload
	if err != nil {
		return ""
	}

	// Extract the "jti" (JWT ID) claim, which stores the user ID
	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}

	if userID, ok := claims["jti"].(string); ok {
		fmt.Println("userID", userID)

		return userID
	}
	return ""
}

func GetCurrentTime() time.Time {
	location, _ := time.LoadLocation(constants.TIMEZONE)
	return time.Now().In(location)
}

func GetUlid() (string, error) {
	entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
	ms := ulid.Timestamp(time.Now())
	id, err := ulid.New(ms, entropy)
	if err != nil {
		return "", errors.New("failed to generate ULID")
	}
	return id.String(), nil
}

func GetFileExtension(mimeType string) (string, error) {
	// Validate if the MIME type is allowed
	if !allowedMimeTypes[mimeType] {
		return "", errors.New("unsupported file type")
	}

	// Extract file extension using Go's built-in MIME package
	exts, _ := mime.ExtensionsByType(mimeType)
	if len(exts) > 0 {
		return exts[0], nil // Use the first matching extension
	}

	// If no known extension, return an error
	return "", errors.New("unable to determine file extension")
}

func Contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

func GetLoggedInUser(c *gin.Context) (string, string, error) {

	loggedInUser, exists := c.Get("userID")
	if !exists {
		return "", "", errors.New("user_id doesn't exist")
	}

	loggedInUserStr, ok := loggedInUser.(string)
	if !ok {
		return "", "", errors.New("invalid userID type")
	}

	loggedInUserType, exists := c.Get("userType")
	if !exists {
		return "", "", errors.New("userType doesn't exist")
	}

	loggedInUserTypeStr, ok := loggedInUserType.(string)
	if !ok {
		return "", "", errors.New("invalid userType type")
	}

	return loggedInUserStr, loggedInUserTypeStr, nil
}
