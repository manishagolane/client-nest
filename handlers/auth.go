package handlers

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/manishagolane/client-nest/clients"
	"github.com/manishagolane/client-nest/constants"

	"github.com/manishagolane/client-nest/cache"
	"github.com/manishagolane/client-nest/config"
	"github.com/manishagolane/client-nest/data"
	"github.com/manishagolane/client-nest/models"
	"github.com/manishagolane/client-nest/utils"

	"github.com/cristalhq/jwt/v4"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
	rl "github.com/jsjain/go-rate-limiter"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	queries         *models.Queries
	cache           *cache.Cache
	lock            *cache.CacheLock
	conn            *pgxpool.Pool
	userRateLimiter *rl.Limiter
	client          *clients.Clients
	customerBuilder *jwt.Builder
	employeeBuilder *jwt.Builder
}

func NewAuthHandler(logger *zap.Logger, cache *cache.Cache, lock *cache.CacheLock, queries *models.Queries, conn *pgxpool.Pool, client *clients.Clients) *AuthHandler {

	customerKey := config.GetString("authentication.jwt.customer_secret")
	employeeKey := config.GetString("authentication.jwt.employee_secret")

	if customerKey == "" || employeeKey == "" {
		logger.Fatal("jwt secrets are not set in configuration")
	}

	customerDecodedKey, err := hex.DecodeString(customerKey)
	if err != nil {
		logger.Fatal("failed to decode Customer JWT secret key", zap.Error(err))
	}

	employeeDecodedKey, err := hex.DecodeString(employeeKey)
	if err != nil {
		logger.Fatal("failed to decode Employee JWT secret key", zap.Error(err))
	}

	customerSigner, err := jwt.NewSignerEdDSA(customerDecodedKey)
	if err != nil {
		logger.Fatal("failed to create JWT signer for customer", zap.Error(err))
	}

	employeeSigner, err := jwt.NewSignerEdDSA(employeeDecodedKey)
	if err != nil {
		logger.Fatal("failed to create JWT signer for employee", zap.Error(err))
	}

	customerBuilder := jwt.NewBuilder(customerSigner)
	employeeBuilder := jwt.NewBuilder(employeeSigner)

	userRateLimiter := rl.NewLimiter(*cache.GetClient(), rl.WithRateLimit(rl.PerSecond(config.GetInt("limit.user"))))

	return &AuthHandler{
		queries:         queries,
		cache:           cache,
		lock:            lock,
		conn:            conn,
		userRateLimiter: userRateLimiter,
		client:          client,
		customerBuilder: customerBuilder,
		employeeBuilder: employeeBuilder,
	}
}

func (ah AuthHandler) EmployeeLogin(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.GetCtxLogger(ctx)
	logger = logger.With(zap.String("source", "EmployeeLogin"))

	var loginRequest data.LoginRequest
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		logger.Error("invalid request", zap.Error(err))
		SendBadRequestError(c, getPrettyValidationError(err))
		return
	}
	// logger.Info("loginRequest", zap.Reflect("loginRequest", any(loginRequest)))

	// Acquire mutex lock for handling parallel requests from same user.
	mutex := ah.lock.Mutex(loginRequest.UserID)
	if err := mutex.LockContext(ctx); err != nil {
		logger.Error("error acquiring lock", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}
	defer mutex.UnlockContext(ctx)

	user, err := ah.queries.GetEmployeeById(ctx, loginRequest.UserID)
	if err != nil {
		logger.Error("error fetching user from db", zap.Error(err))
		SendBadRequestError(c, errors.New("user not found"))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginRequest.Password)); err != nil {
		logger.Error("error checking for the number of attempts user retried the wrong password", zap.Error(err))
		SendApplicationError(c, errors.New("invalid credentials"))
		return
	}

	ctx = context.WithValue(ctx, constants.UserID, user.ID)

	// logger.Info("user added to context", zap.Reflect("user", user))
	roles, err := ah.queries.GetRoleById(ctx, user.RoleID)
	if err != nil {
		logger.Error("error fetching role from db", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	daysToExpire := utils.GetExpiryTime(7)
	userStruct := data.AuthUser{
		ID:        user.ID,
		UserType:  roles.Name,
		RoleID:    user.RoleID,
		ExpiresAt: daysToExpire,
	}

	token, err := ah.createAuthToken(ctx, userStruct)
	if err != nil {
		logger.Error("error building JWT token", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}
	SendSuccessResponse(c, "Login successful", gin.H{"token": token.String()})
}

func (ah AuthHandler) CustomerLogin(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.GetCtxLogger(ctx)
	logger = logger.With(zap.String("source", "CustomerLogin"))

	var loginRequest data.LoginRequest
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		logger.Error("invalid request", zap.Error(err))
		SendBadRequestError(c, getPrettyValidationError(err))
		return
	}
	// logger.Info("loginRequest", zap.Reflect("loginRequest", loginRequest))

	// Acquire mutex lock for handling parallel requests from same user.
	mutex := ah.lock.Mutex(loginRequest.UserID)
	if err := mutex.LockContext(ctx); err != nil {
		logger.Error("error acquiring lock", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}
	defer mutex.UnlockContext(ctx)

	user, err := ah.queries.GetCustomerById(ctx, loginRequest.UserID)
	if err != nil {
		logger.Error("error fetching user from db", zap.Error(err))
		SendBadRequestError(c, errors.New("user not found"))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginRequest.Password)); err != nil {
		logger.Error("error checking for the number of attempts user retried the wrong password",
			zap.Error(err))
		SendApplicationError(c, errors.New("invalid credentials"))
		return
	}

	ctx = context.WithValue(ctx, constants.UserID, user.ID)

	// logger.Info("user added to context", zap.Reflect("user", user))
	roles, err := ah.queries.GetRoleByName(ctx, "customer")
	if err != nil {
		logger.Error("error fetching role from db", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	userStruct := data.AuthUser{
		ID:        user.ID,
		UserType:  "customer",
		RoleID:    roles.ID,
		ExpiresAt: utils.GetEndOfDayTime(), // Token valid for 1 day
	}
	token, err := ah.createAuthToken(ctx, userStruct)
	if err != nil {
		logger.Error("error building JWT token", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	SendSuccessResponse(c, "Login successful", gin.H{"token": token.String()})
}

func (ah AuthHandler) createAuthToken(ctx context.Context, user data.AuthUser) (*jwt.Token, error) {
	logger := utils.GetCtxLogger(ctx)
	audience := fmt.Sprintf("%s:%s", user.RoleID, user.UserType)

	claims := &jwt.RegisteredClaims{
		ID:        user.ID,
		ExpiresAt: jwt.NewNumericDate(user.ExpiresAt),
		IssuedAt:  jwt.NewNumericDate(utils.GetCurrentTime()),
		Audience:  jwt.Audience{audience},
	}

	// logger.Info("Generated JWT claims", zap.Reflect("claims", claims))

	token, err := func() (*jwt.Token, error) {
		if user.UserType == "customer" {
			return ah.customerBuilder.Build(claims)
		}
		return ah.employeeBuilder.Build(claims)
	}()
	if err != nil {
		logger.Error("error building JWT token", zap.Error(err))
		return nil, err
	}

	tokenParts := strings.Split(token.String(), ".")
	if len(tokenParts) != 3 {
		logger.Error("invalid token format")
		return nil, err
	}

	signature := tokenParts[2]

	tokenKey := fmt.Sprintf("jwt_sig:%s", signature)
	err = ah.cache.Set(ctx, tokenKey, "", time.Until(user.ExpiresAt))
	if err != nil {
		logger.Error("failed to store key in cache")
		return nil, err
	}
	return token, nil
}

func (ah AuthHandler) Logout(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.GetCtxLogger(ctx)

	// Extract the session token from the Authorization header
	tokenString, err := GetTokenFromRequest(c, constants.API_TOKEN)
	if err != nil {
		logger.Error("missing Authorization token", zap.Error(err))
		SendUnauthorizedError(c, errors.New("missing Authorization token"))
		return
	}

	// Extract signature from JWT
	tokenParts := strings.Split(tokenString, ".")
	if len(tokenParts) != 3 {
		logger.Error("invalid token format")
		SendUnauthorizedError(c, errors.New("invalid token format"))
		return
	}

	signature := tokenParts[2]

	tokenKey := fmt.Sprintf("jwt_sig:%s", signature)
	_, err = ah.cache.Get(ctx, tokenKey)
	if err != nil {
		logger.Error("token not found in Redis, possibly already logged out or expired")
		SendUnauthorizedError(c, errors.New("invalid session"))
		return
	}

	// Delete the token from Redis (logout)
	err = ah.cache.Delete(ctx, tokenKey)
	if err != nil {
		logger.Error("failed to delete JWT token from Redis", zap.Error(err))
		SendApplicationError(c, errors.New("internal server error"))
		return
	}

	SendSuccessResponse(c, "Logged out successfully", nil)
}

// func (ah AuthHandler) EmployeeSignup(c *gin.Context) {
// 	ctx := c.Request.Context()
// 	logger := utils.GetCtxLogger(ctx)

// 	var signUpRequest data.SignUpRequest
// 	err := c.ShouldBindJSON(&signUpRequest)
// 	if err != nil {
// 		logger.Error("invalid request", zap.Error(err))
// 		SendBadRequestError(c, getPrettyValidationError(err))
// 		return
// 	}

// 	logger.Info("signUpRequest", zap.Reflect("signUpRequest", signUpRequest))
// 	mutex := ah.lock.Mutex(signUpRequest.Email)
// 	if err := mutex.LockContext(ctx); err != nil {
// 		logger.Error("error acquiring lock", zap.Error(err))
// 		SendApplicationError(c, errors.New("internal server error"))
// 		return
// 	}
// 	defer mutex.UnlockContext(ctx)

// 	hashedPassword, err := generatePasswordHash([]byte(signUpRequest.Password))
// 	if err != nil {
// 		logger.Error("error hashing password", zap.Error(err))
// 		SendApplicationError(c, errors.New("internal server error"))
// 		return
// 	}

// 	// Hashing the password before storing it in redis
// 	signUpRequest.Password = hashedPassword

// 	_, err = ah.client.OTPClient.SendOTP(ctx, signUpRequest.Email, constants.Login)
// 	if err != nil {
// 		logger.Error("error sending otp to developer", zap.Error(err))
// 		SendApplicationError(c, errors.New("internal server error"))
// 		return
// 	}

// 	requestId := uuid.NewString()

// 	err = ah.cache.SetJSON(ctx, getDeveloperSignUpFromRedisKey(signUpRequest.Email, requestId), signUpRequest, time.Minute*5)
// 	if err != nil {
// 		logger.Error("error setting redis value for developer signup", zap.Error(err))
// 		SendApplicationError(c, errors.New("internal server error"))
// 		return
// 	}
// 	SendSuccessResponse(c, "OTP sent to email successfully", gin.H{"requestId": requestId})
// }

// func (ah AuthHandler) EmployeeOtpVerify(c *gin.Context) {
// 	ctx := c.Request.Context()
// 	logger := utils.GetCtxLogger(ctx)

// 	var developerSignUpOtpVerification data.DeveloperSignUpOtpVerificationRequest
// 	if err := c.ShouldBindJSON(&developerSignUpOtpVerification); err != nil {
// 		logger.Error("error while binding json", zap.Error(err))
// 		SendBadRequestError(c, getPrettyValidationError(err))
// 		return
// 	}

// 	var signUpRequest data.SignUpRequest

// 	err := ah.cache.GetJSON(ctx, getDeveloperSignUpFromRedisKey(developerSignUpOtpVerification.Email, developerSignUpOtpVerification.RequestID), &signUpRequest)
// 	if err != nil {
// 		if errors.Is(err, cache.ErrorNotFound) {
// 			logger.Error("invalid or missing request ID")
// 			SendBadRequestError(c, errors.New("invalid request"))
// 		} else {
// 			logger.Error("error while getting reset password context from cache", zap.Error(err))
// 			SendApplicationError(c, errors.New("internal server error"))
// 		}
// 		return
// 	}

// 	err = ah.client.OTPClient.CheckOTP(ctx, developerSignUpOtpVerification.Email, developerSignUpOtpVerification.OTP)
// 	if err != nil {
// 		HandleOTPValidationError(c, err)
// 		logger.Error("error verifying OTP", zap.Error(err))
// 		return
// 	}

// 	role, err := ah.queries.GetRoleByName(ctx, strings.ToLower(signUpRequest.Role))
// 	if err != nil {
// 		logger.Error("error fetching role by name", zap.Error(err))
// 		SendApplicationError(c, errors.New("internal server error"))
// 		return
// 	}

// 	ulidId, err := utils.GetUlid()
// 	if err != nil {
// 		logger.Error("failed to generate ULID", zap.Error(err))
// 		SendApplicationError(c, errors.New("internal server error"))
// 		return
// 	}

// 	_, err = ah.queries.AddEmployee(ctx, models.AddEmployeeParams{
// 		ID:       ulidId,
// 		Name:     signUpRequest.Name,
// 		Password: signUpRequest.Password,
// 		Email:    signUpRequest.Email,
// 		Phone:    signUpRequest.Phone,
// 		RoleID:   role.ID,
// 	})

// 	if err != nil {
// 		pgxErr, ok := err.(*pgconn.PgError)
// 		if ok {
// 			if pgxErr.Code == pgerrcode.UniqueViolation {
// 				logger.Error("unique constraint violated", zap.Error(err))
// 				SendApplicationError(c, errors.New("internal server error"))
// 				return
// 			}
// 		}
// 		logger.Error("error inserting developer in db", zap.Error(err))
// 		SendApplicationError(c, errors.New("internal server error"))
// 		return
// 	}

// 	ah.cache.Delete(ctx, getDeveloperSignUpFromRedisKey(developerSignUpOtpVerification.Email, developerSignUpOtpVerification.RequestID))

// 	SendSuccessResponse(c, "Developer account created successfully", gin.H{"userId": ulidId})
// }
