package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// TokenValidityDuration is the duration for which a token is valid
	TokenValidityDuration = 24 * 30 * time.Hour // 30 days
)

// TokenData represents the data encoded in a token
type TokenData struct {
	UserID    string    // Authentik user ID
	Email     string    // User email
	Timestamp time.Time // Token creation time
}

// GenerateToken creates a secure token for a user that can be used in public URLs
// The token format is: base64(userID:email:timestamp):hmac
func GenerateToken(userID, email, secret string) (string, error) {
	if userID == "" || email == "" || secret == "" {
		return "", errors.New("userID, email, and secret are required")
	}

	// Create timestamp
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Create payload
	payload := fmt.Sprintf("%s:%s:%s", userID, email, timestamp)
	
	// Base64 encode the payload
	encodedPayload := base64.URLEncoding.EncodeToString([]byte(payload))
	
	// Generate HMAC
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(encodedPayload))
	signature := hex.EncodeToString(h.Sum(nil))
	
	// Combine payload and signature
	token := fmt.Sprintf("%s:%s", encodedPayload, signature)
	
	return token, nil
}

// VerifyToken verifies a token and returns the user data if valid
func VerifyToken(token, secret string) (*TokenData, error) {
	// Split token into payload and signature
	parts := strings.Split(token, ":")
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}
	
	encodedPayload := parts[0]
	providedSignature := parts[1]
	
	// Verify HMAC
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(encodedPayload))
	expectedSignature := hex.EncodeToString(h.Sum(nil))
	
	if !hmac.Equal([]byte(providedSignature), []byte(expectedSignature)) {
		return nil, errors.New("invalid token signature")
	}
	
	// Decode payload
	payloadBytes, err := base64.URLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}
	
	// Parse payload
	payloadStr := string(payloadBytes)
	payloadParts := strings.SplitN(payloadStr, ":", 3)
	if len(payloadParts) != 3 {
		return nil, fmt.Errorf("invalid payload format: %s", payloadStr)
	}
	
	userID := payloadParts[0]
	email := payloadParts[1]
	timestampStr := payloadParts[2]
	
	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}
	
	// Check if token is expired
	if time.Since(timestamp) > TokenValidityDuration {
		return nil, errors.New("token expired")
	}
	
	return &TokenData{
		UserID:    userID,
		Email:     email,
		Timestamp: timestamp,
	}, nil
}
