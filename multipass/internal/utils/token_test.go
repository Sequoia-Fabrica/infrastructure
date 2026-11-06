package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
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

// A member found by numeric ID doesn't need their email in the link, so it
// stays out; tokens minted before that still verify.
func TestTokenLeavesEmailOutForNumericIDs(t *testing.T) {
	secret := "test-secret-key"
	token, err := GenerateToken("42", "member@example.com", secret)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	payload, err := base64.URLEncoding.DecodeString(strings.SplitN(token, ":", 2)[0])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if strings.Contains(string(payload), "member@example.com") {
		t.Errorf("payload %q carries the email", payload)
	}
	data, err := VerifyToken(token, secret)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if data.UserID != "42" || data.Email != "" {
		t.Errorf("got %+v, want UserID 42 and no email", data)
	}

	// An older token, with the email in it
	old := base64.URLEncoding.EncodeToString([]byte("42:member@example.com:" + time.Now().UTC().Format(time.RFC3339)))
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(old))
	data, err = VerifyToken(old+":"+hex.EncodeToString(h.Sum(nil)), secret)
	if err != nil || data.UserID != "42" || data.Email != "member@example.com" {
		t.Errorf("old token: got %+v, %v", data, err)
	}
}
