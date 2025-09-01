package handlers

import (
	"html/template"
	"multipass/internal/config"
	"multipass/internal/models"
	"multipass/internal/utils"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// PublicCardHandler renders the card for a user based on a token
// This handler is protected by the TokenAuthMiddleware
func PublicCardHandler(c *gin.Context) {
	// Get user profile from context (set by TokenAuthMiddleware)
	userProfile, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Cast to UserProfile
	user, ok := userProfile.(*models.UserProfile)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user data"})
		return
	}

	// Load config
	cfg := config.Load()

	// Build membership info (simplified version)
	// In a real implementation, this would be fetched from a database or service
	membershipInfo := &models.MembershipInfo{
		MembershipType: "Digital Member",
		Status:         models.StatusActive,
		UserLevel:      user.AccessLevel,
		JoinDate:       getDefaultJoinDate(),
		ExpiryDate:     getDefaultExpiryDate(),
	}

	// Get debug info if available
	var debugInfo map[string]interface{}
	if debugInfoRaw, exists := c.Get("debug_info"); exists {
		if di, ok := debugInfoRaw.(map[string]interface{}); ok {
			debugInfo = di
		}
	}

	// Prepare template data
	templateData := gin.H{
		"title":           "Digital ID Card - " + cfg.MakerspaceName,
		"user":            user,
		"membership":      membershipInfo,
		"makerspace_name": cfg.MakerspaceName,
		"logo_url":        cfg.LogoURL,
		"qr_data":         c.Request.URL.String(), // Use the current URL as QR code data
		"public_view":     true,                   // Flag to indicate this is a public view
	}

	// Add debug info if available
	if debugInfo != nil {
		templateData["debug"] = debugInfo
	}

	// Render card template
	c.HTML(http.StatusOK, "card.html", templateData)
}

// Helper function to get a default join date (1 year ago)
func getDefaultJoinDate() *time.Time {
	t := time.Now().AddDate(-1, 0, 0)
	return &t
}

// Helper function to get a default expiry date (1 year from now)
func getDefaultExpiryDate() *time.Time {
	t := time.Now().AddDate(1, 0, 0)
	return &t
}

// GenerateTokenLinkHandler creates a page with a QR code containing the token link
func GenerateTokenLinkHandler(c *gin.Context) {
	// Get user profile from context
	userProfile, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	// Cast to UserProfile
	user, ok := userProfile.(*models.UserProfile)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user data"})
		return
	}

	// Get config
	cfg := config.Load()

	// Check if token secret is configured
	if cfg.TokenSecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token generation not configured"})
		return
	}

	// Generate token
	userID := user.AuthentikID // Use the Authentik ID for consistent identification
	if userID == "" {
		// Fall back to email if Authentik ID is not available
		userID = user.Email
	}
	token, err := utils.GenerateToken(userID, user.Email, cfg.TokenSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Get base URL
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	baseURL := scheme + "://" + c.Request.Host

	// Create public card URL with properly encoded token
	publicCardURL := baseURL + "/public/card?token=" + url.QueryEscape(token)

	// Generate QR code as base64 data URI
	qrCodeBase64, err := utils.GenerateQRCodeBase64(publicCardURL, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR code"})
		return
	}
	
	// Convert to template.HTML to prevent escaping
	qrCodeHTML := template.HTML("<img src=\"" + qrCodeBase64 + "\" alt=\"QR Code\" class=\"qr-code\">")

	// Render token link template
	c.HTML(http.StatusOK, "token_link.html", gin.H{
		"title":           "Share Your Digital ID - " + cfg.MakerspaceName,
		"makerspace_name": cfg.MakerspaceName,
		"logo_url":        cfg.LogoURL,
		"token":           token,
		"public_url":      publicCardURL,
		"qr_code_html":    qrCodeHTML,
	})
}
