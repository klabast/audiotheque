package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"audiod/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims for a user
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func getJWTSecret() []byte {
	// 1. Check JWT_SECRET env var first (override)
	secret := os.Getenv("JWT_SECRET")
	if secret != "" {
		return []byte(secret)
	}

	// 2. Check for persisted secret file
	dataDir := config.GetDataDir()

	secretPath := dataDir + "/jwt-secret.key"

	// Try to read existing secret
	data, err := os.ReadFile(secretPath)
	if err == nil {
		return data
	}

	// 3. Generate new secret and persist it
	log.Println("Generating new JWT secret...")
	newSecret := make([]byte, 32)
	_, err = rand.Read(newSecret)
	if err != nil {
		log.Fatalf("FATAL: Failed to generate JWT secret: %v", err)
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("FATAL: Failed to create data directory: %v", err)
	}

	// Write secret with restricted permissions
	if err := os.WriteFile(secretPath, newSecret, 0600); err != nil {
		log.Fatalf("FATAL: Failed to save JWT secret: %v", err)
	}

	log.Printf("JWT secret saved to %s - backup this file!", secretPath)
	return newSecret
}

// GenerateToken creates a JWT token for a user with 7-day expiry
func GenerateToken(userID int64, username string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(getJWTSecret())
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("token is empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return getJWTSecret(), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}
