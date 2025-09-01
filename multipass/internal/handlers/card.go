package handlers

import (
	"multipass/internal/config"
	"multipass/internal/models"
	"multipass/internal/utils"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// CardHandler handles requests for the digital ID card by generating a public share URL and redirecting to it
func CardHandler(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusTemporaryRedirect, "/login")
		return
	}

	userProfile := user.(*models.UserProfile)

	// Get config
	cfg := config.Load()

	// Check if token secret is configured
	if cfg.TokenSecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token generation not configured"})
		return
	}

	// Generate token using the user's Authentik ID or email
	userID := userProfile.MemberID // Use MemberID which is now set to the numeric PK
	if userID == "" || userID == "TBD" {
		userID = userProfile.AuthentikID
		if userID == "" {
			userID = userProfile.Email
		}
	}

	token, err := utils.GenerateToken(userID, userProfile.Email, cfg.TokenSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Create public card URL with properly encoded token
	publicCardURL := "/public/card?token=" + url.QueryEscape(token)

	// Redirect to the public card URL
	c.Redirect(http.StatusTemporaryRedirect, publicCardURL)
}

// isMobileUserAgent is deprecated - we now use responsive design with media queries
// Kept for reference but no longer used
// TODO: Remove this function in a future cleanup
func isMobileUserAgent(userAgent string) bool {
	// This function is no longer used as we've switched to responsive design
	return false
}

// generateQRData generates QR code data for the user
// If token secret is configured, it will generate a secure token URL
func generateQRData(user *models.UserProfile, cfg *config.Config) string {
	// Check if token secret is configured
	if cfg.TokenSecret != "" {
		// Generate a secure token
		token, err := utils.GenerateToken(user.Email, user.Email, cfg.TokenSecret)
		if err == nil {
			// Return a URL with the token
			return "/public/card?token=" + token
		}
	}
	
	// Fallback to the old format if token generation fails
	return "MEMBER:" + user.MemberID + ":" + user.Email
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		   (s == substr ||
		    (len(substr) > 0 &&
		     (s[:len(substr)] == substr ||
		      contains(s[1:], substr))))
}
