package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/manishagolane/client-nest/auth"
	"github.com/manishagolane/client-nest/cache"
	"github.com/manishagolane/client-nest/constants"
	"github.com/manishagolane/client-nest/handlers"
	"github.com/manishagolane/client-nest/models"
	"github.com/manishagolane/client-nest/utils"
	"go.uber.org/zap"
)

// GinLogger receives the default log of the gin framework
func ginLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()

		logger := utils.GetCtxLogger(c.Request.Context())
		cost := time.Since(start)
		logger.Info(path,
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.Duration("cost", cost),
		)
	}
}

// GinRecovery removes the possible panic of the project
func ginRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, true)
				logger := utils.GetCtxLogger(c.Request.Context())
				if brokenPipe {
					logger.Sugar().Error(c.Request.URL.Path,
						zap.Any("error", err),
						zap.String("requestId", c.Request.Context().Value(constants.REQUEST_ID).(string)))
					logger.Sugar().Error(string(httpRequest))
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error))
					c.Abort()
					return
				}
				logger.Sugar().Error(err)
				logger.Sugar().Error(string(debug.Stack()))
				logger.Sugar().Error("[raw http request] ", string(httpRequest))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"message": "Something went wrong, please try again later",
				})
			}
		}()
		c.Next()
	}
}

func requestIdInterceptor() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestId := uuid.NewString()
		r := c.Request.Context()
		updatedContext := context.WithValue(r, constants.REQUEST_ID, requestId)
		c.Request = c.Request.WithContext(updatedContext)
		c.Writer.Header().Add("x-request-id", requestId)
		c.Next()
	}
}

func authMiddleware(cache *cache.Cache, jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := utils.GetCtxLogger(c.Request.Context())

		// Extract token from Authorization header
		tokenStr, err := handlers.GetTokenFromRequest(c, constants.API_TOKEN)
		if err != nil {
			logger.Error("missing Authorization token", zap.Error(err))
			handlers.SendUnauthorizedError(c, errors.New("missing Authorization token"))
			c.Abort()
			return
		}

		tokenParts := strings.Split(tokenStr, ".")
		if len(tokenParts) != 3 {
			logger.Error("invalid token format")
			handlers.SendUnauthorizedError(c, errors.New("invalid token format"))
			c.Abort()
			return
		}

		signature := tokenParts[2]
		tokenKey := fmt.Sprintf("jwt_sig:%s", signature)
		_, err = cache.Get(c, tokenKey)
		if err != nil {
			logger.Error("invalid JWT token", zap.Error(err))
			handlers.SendUnauthorizedError(c, errors.New("invalid token"))
			c.Abort()
			return
		}

		// If not in Redis, verify JWT
		userType, userID, roleID, err := verifyJWT(tokenStr, c, jwtManager)
		if err != nil {
			logger.Error("invalid JWT token", zap.Error(err))
			handlers.SendUnauthorizedError(c, errors.New("invalid token"))
			c.Abort()
			return
		}

		eod := time.Until(utils.GetEndOfDayTime())
		// Store token signature in Redis
		err = cache.Set(c.Request.Context(), tokenKey, "", eod) // Set TTL
		if err != nil {
			logger.Error("error while storing key in cache", zap.Error(err))
			handlers.SendUnauthorizedError(c, errors.New("internal server error token"))
			c.Abort()
		}

		// Store user info in Gin context
		c.Set("userID", userID)
		c.Set("roleID", roleID)
		c.Set("userType", userType)

		logger.Info("User session created in Redis", zap.String("userID", userID), zap.String("roleID", roleID))
		c.Next()
	}
}

// Function to verify JWT and extract claims
func verifyJWT(tokenString string, c *gin.Context, jwtManager *auth.JWTManager) (string, string, string, error) {
	logger := utils.GetCtxLogger(c.Request.Context())

	claims, err := jwtManager.JWTParser(tokenString)
	if err != nil {
		return "", "", "", err
	}
	userID := claims.ID
	roleID := "customer"
	userType := "customer"

	if len(claims.Audience) > 0 {
		audience := claims.Audience[0]
		audienceParts := strings.SplitN(audience, ":", 2) // Split by ":"
		roleID = audienceParts[0]
		if len(audienceParts) > 1 {
			userType = audienceParts[1] // Extract userType from audience
		}
	}

	logger.Info("claims", zap.Any("claims", claims), zap.String("userID", userID), zap.String("roleID", roleID), zap.String("userType", userType))
	return userType, userID, roleID, nil
}

// Middleware for RBAC
func rbacMiddleware(queries *models.Queries, cache *cache.Cache, requiredObject string, requiredAction string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		logger := utils.GetCtxLogger(ctx).With(zap.String("source", "rbacmiddleware"))

		userID, exists := c.Get("userID")
		roleID, roleExists := c.Get("roleID")

		if !exists || !roleExists {
			logger.Error("user_id or role_id doesn't exist")
			handlers.SendUnauthorizedError(c, errors.New("unauthorized"))
			c.Abort()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			logger.Error("invalid userID type")
			handlers.SendUnauthorizedError(c, errors.New("unauthorized"))
			c.Abort()
			return

		}
		roleIDStr, ok := roleID.(string)
		if !ok {
			logger.Error("invalid roleID type")
			handlers.SendUnauthorizedError(c, errors.New("unauthorized"))
			c.Abort()
			return

		}

		ticketID := c.Param("id")
		if ticketID == "" {
			logger.Error("ticket iD is missing in URL")
			handlers.SendBadRequestError(c, errors.New("ticket ID is required"))
			return
		}

		// Customers can only access their own tickets
		if roleIDStr == "customer" && requiredObject == "ticket" {
			// Check Redis for ticket owner
			ticketOwner, err := cache.GetTicketOwner(ctx, ticketID, queries)
			if err != nil || ticketOwner != userIDStr {
				logger.Error("unauthorized ticket access", zap.String("ticket_id", ticketID))
				handlers.SendForbiddenError(c, errors.New("unauthorized ticket access"))
				return
			}
		}

		// Cache key specific to (roleID, Object, Action)
		cacheKey := fmt.Sprintf("permission:%s:%s:%s", roleID, requiredObject, requiredAction)
		// logger.Info("checking permission cache:", zap.String("cacheKey", cacheKey))

		// Try getting permission from cache
		cachedPermission, err := cache.GetBool(ctx, cacheKey)
		if err == nil && cachedPermission {
			c.Next()
			return
		}

		// If not in cache, check DB for role-based access
		hasPermission, err := queries.HasPermission(ctx, models.HasPermissionParams{
			RoleID: roleIDStr,
			Object: requiredObject,
			Action: requiredAction,
		})
		if err != nil {
			handlers.SendApplicationError(c, errors.New("internal server error"))
			return
		}

		cache.SetBool(ctx, cacheKey, hasPermission, time.Minute*10)

		// Allow or deny access
		if hasPermission {
			c.Next()
			return
		}

		handlers.SendForbiddenError(c, errors.New("permission denied"))
	}
}
