package auth

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/cristalhq/jwt/v4"
	"github.com/manishagolane/client-nest/config"
	"go.uber.org/zap"
)

type JWTManager struct {
	customerVerifier *jwt.EdDSAAlg
	employeeVerifier *jwt.EdDSAAlg
	logger           *zap.Logger
}

// NewJWTManager initializes and returns a JWTManager
func NewJWTManager(logger *zap.Logger) *JWTManager {
	customerSecret := config.GetString("authentication.jwt.customer_secret")
	employeeSecret := config.GetString("authentication.jwt.employee_secret")

	customerPrivateKey, err := hex.DecodeString(customerSecret)
	if err != nil {
		logger.Fatal("invalid customer private key", zap.Error(err))
		return nil
	}
	customerPublicKey := ed25519.PrivateKey(customerPrivateKey).Public().(ed25519.PublicKey)
	customerVerifier, err := jwt.NewVerifierEdDSA(customerPublicKey)
	if err != nil {
		logger.Fatal("failed to create customer verifier", zap.Error(err))
		return nil
	}

	// Decode Employee Secret Key
	employeePrivateKey, err := hex.DecodeString(employeeSecret)
	if err != nil {
		logger.Fatal("invalid employee private key", zap.Error(err))
		return nil
	}
	employeePublicKey := ed25519.PrivateKey(employeePrivateKey).Public().(ed25519.PublicKey)
	employeeVerifier, err := jwt.NewVerifierEdDSA(employeePublicKey)
	if err != nil {
		logger.Fatal("failed to create employee verifier", zap.Error(err))
		return nil
	}

	return &JWTManager{
		customerVerifier: customerVerifier,
		employeeVerifier: employeeVerifier,
		logger:           logger,
	}
}

func (jm *JWTManager) JWTParser(tokenString string) (jwt.RegisteredClaims, error) {
	token, err := jwt.Parse([]byte(tokenString), jm.customerVerifier)
	if err != nil {
		// If customer verification fails, try employee key
		token, err = jwt.Parse([]byte(tokenString), jm.employeeVerifier)
		if err != nil {
			return jwt.RegisteredClaims{}, errors.New("invalid token signature")
		}
	}

	// Extract Claims
	var claims jwt.RegisteredClaims
	err = json.Unmarshal(token.Claims(), &claims)
	if err != nil {
		return jwt.RegisteredClaims{}, errors.New("failed to parse claims")
	}

	return claims, nil
}
