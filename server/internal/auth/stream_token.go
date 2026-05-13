package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// streamTokenKind identifies stream-scoped tokens. Stored in the `kind` claim
// so a session token can never be substituted for a stream token (or vice
// versa) even though both are HS256-signed with the same JWT secret.
const streamTokenKind = "stream"

// StreamClaims are the claims for stream-scoped signed URLs handed to MPD
// devices. The token is bound to a single track and user; an attacker who
// captures the URL can only fetch that one track for the validity window.
type StreamClaims struct {
	Kind    string `json:"kind"`
	TrackID int64  `json:"track_id"`
	UserID  int64  `json:"user_id"`
	jwt.RegisteredClaims
}

// MintStreamToken signs a token for streaming a specific track. The token has
// no use for any other endpoint or track.
func MintStreamToken(trackID, userID int64, ttl time.Duration) (string, error) {
	claims := &StreamClaims{
		Kind:    streamTokenKind,
		TrackID: trackID,
		UserID:  userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(getJWTSecret())
	if err != nil {
		return "", fmt.Errorf("failed to sign stream token: %w", err)
	}
	return signed, nil
}

// ValidateStreamToken checks the signature, expiry, kind, and trackID binding.
// On success returns the userID encoded in the token.
func ValidateStreamToken(tokenString string, trackID int64) (int64, error) {
	if tokenString == "" {
		return 0, errors.New("stream token is empty")
	}
	token, err := jwt.ParseWithClaims(tokenString, &StreamClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return getJWTSecret(), nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to parse stream token: %w", err)
	}
	claims, ok := token.Claims.(*StreamClaims)
	if !ok || !token.Valid {
		return 0, errors.New("invalid stream token claims")
	}
	if claims.Kind != streamTokenKind {
		return 0, errors.New("token is not a stream token")
	}
	if claims.TrackID != trackID {
		return 0, errors.New("stream token trackID mismatch")
	}
	return claims.UserID, nil
}
