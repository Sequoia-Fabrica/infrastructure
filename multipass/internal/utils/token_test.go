package utils

import (
	"testing"
	"time"
)

func TestTokenGenerationAndVerification(t *testing.T) {
	// Test data
	userID := "test-user-123"
	email := "test@example.com"
	secret := "test-secret-key"

	// Generate token
	token, err := GenerateToken(userID, email, secret)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Verify token
	tokenData, err := VerifyToken(token, secret)
	if err != nil {
		t.Fatalf("Failed to verify token: %v", err)
	}

	// Check token data
	if tokenData.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, tokenData.UserID)
	}
	if tokenData.Email != email {
		t.Errorf("Expected Email %s, got %s", email, tokenData.Email)
	}
	if time.Since(tokenData.Timestamp) > time.Minute {
		t.Errorf("Token timestamp is too old: %v", tokenData.Timestamp)
	}
}

func TestTokenVerificationWithInvalidSignature(t *testing.T) {
	// Test data
	userID := "test-user-123"
	email := "test@example.com"
	secret := "test-secret-key"
	wrongSecret := "wrong-secret-key"

	// Generate token
	token, err := GenerateToken(userID, email, secret)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Verify token with wrong secret
	_, err = VerifyToken(token, wrongSecret)
	if err == nil {
		t.Error("Expected verification to fail with wrong secret, but it succeeded")
	}
}

func TestTokenVerificationWithInvalidFormat(t *testing.T) {
	// Test invalid token formats
	invalidTokens := []string{
		"invalid-token",
		"part1:part2:part3",
		":",
		"base64data:",
	}

	secret := "test-secret-key"

	for _, token := range invalidTokens {
		_, err := VerifyToken(token, secret)
		if err == nil {
			t.Errorf("Expected verification to fail for invalid token %q, but it succeeded", token)
		}
	}
}

func TestTokenExpiration(t *testing.T) {
	// This test would ideally mock time.Now() to test expiration
	// For now, we'll just ensure the token validity duration is set correctly
	if TokenValidityDuration != 24*30*time.Hour {
		t.Errorf("Expected token validity duration to be 30 days, got %v", TokenValidityDuration)
	}
}
