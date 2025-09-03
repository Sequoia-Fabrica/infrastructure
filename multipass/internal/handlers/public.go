package handlers

import (
	"html/template"
	"multipass/internal/config"
	"multipass/internal/models"
	"multipass/internal/services"
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
	// Create logger
	logger := services.NewLogger(cfg)

	// Get membership info from the membership service
	membershipService := services.NewMembershipService()
	membershipInfo, err := membershipService.GetMembershipInfo(user)
	if err != nil {
		logger.Error("Failed to retrieve membership info: %v", err)
		// Fall back to default membership info if service fails
		membershipInfo = &models.MembershipInfo{
			MembershipType: "Digital Member",
			Status:         models.StatusActive,
			UserLevel:      user.AccessLevel,
			JoinDate:       getDefaultJoinDate(),
			ExpiryDate:     getDefaultExpiryDate(),
		}
	}

	// Get debug info if available
	var debugInfo map[string]interface{}
	if debugInfoRaw, exists := c.Get("debug_info"); exists {
		if di, ok := debugInfoRaw.(map[string]interface{}); ok {
			debugInfo = di
		}
	}

	// Get the full URL for QR code generation
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	baseURL := scheme + "://" + c.Request.Host

	// Get token from query parameters
	token := c.Query("token")

	// Construct URL with token explicitly included
	fullURL := baseURL + "/public/card?token=" + url.QueryEscape(token)

	// Generate QR code as base64 data URI
	qrCodeBase64, err := utils.GenerateQRCodeBase64(fullURL, 250)
	if err != nil {
		logger.Error("Failed to generate QR code: %v", err)
		qrCodeBase64 = ""
	}

	// Convert to template.HTML to prevent escaping
	qrCodeHTML := template.HTML("<img src=\"" + qrCodeBase64 + "\" alt=\"QR Code\" class=\"qr-code\">")

	// Format dates for display
	joinDateStr := "Unknown"
	expiryDateStr := "Unknown"
	
	if membershipInfo.JoinDate != nil {
		joinDateStr = membershipInfo.JoinDate.Format("Jan 2, 2006")
	}
	
	if membershipInfo.ExpiryDate != nil {
		expiryDateStr = membershipInfo.ExpiryDate.Format("Jan 2, 2006")
	}
	
	// Prepare template data
	templateData := gin.H{
		"title":           "Digital ID Card - " + cfg.MakerspaceName,
		"user":            user,
		"membership":      membershipInfo,
		"makerspace_name": cfg.MakerspaceName,
		"logo_url":        cfg.LogoURL,
		"qr_code_html":    qrCodeHTML,          // Add QR code HTML
		"qr_data":         fullURL,             // Keep the URL as data attribute for backward compatibility
		"public_view":     true,                // Flag to indicate this is a public view
		"current_time":    time.Now().Format("Jan 2, 2006 15:04:05"),  // Current time for reference
		"join_date":       joinDateStr,         // Member since date
		"expiry_date":     expiryDateStr,       // Membership expiry date
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
	qrCodeBase64, err := utils.GenerateQRCodeBase64(publicCardURL, 250)
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
