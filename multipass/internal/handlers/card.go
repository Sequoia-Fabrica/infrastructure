package handlers

import (
	"multipass/internal/middleware"
	"multipass/internal/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// CardHandler handles requests for the digital ID card
func CardHandler(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusTemporaryRedirect, "/login")
		return
	}

	userProfile := user.(*models.UserProfile)

	// Create membership info
	membership := &models.MembershipInfo{
		MembershipType:    userProfile.AccessLevel.String(),
		Status:           models.StatusActive,
		UserLevel:        userProfile.AccessLevel,
	}

	// Get debug info from context
	templateData := gin.H{
		"title":          "Digital ID Card - " + userProfile.GetFullName(),
		"user":           userProfile,
		"membership":     membership,
		"makerspace_name": "Sequoia Fabrica",
		"current_time":   time.Now().Format("January 2, 2006"),
		"qr_data":        generateQRData(userProfile),
	}

	// Add debug info if available
	if debugMode, _ := c.Get("debug_mode"); debugMode != nil && debugMode.(bool) {
		templateData["debug_mode"] = true

		// Get all debug info from middleware
		debugInfo := middleware.GetDebugInfo(c)
		if debugInfo != nil {
			templateData["debug_info"] = debugInfo
		}

		// For backward compatibility
		if debugHeaders, exists := c.Get("debug_headers"); exists {
			templateData["debug_headers"] = debugHeaders
		}
		if debugGroups, exists := c.Get("debug_groups"); exists {
			templateData["debug_groups"] = debugGroups
		}
	}

	// Render unified responsive template
	c.HTML(http.StatusOK, "card.html", templateData)
}

// isMobileUserAgent is deprecated - we now use responsive design with media queries
// Kept for reference but no longer used
// TODO: Remove this function in a future cleanup
func isMobileUserAgent(userAgent string) bool {
	// This function is no longer used as we've switched to responsive design
	return false
}

// generateQRData generates QR code data for the user
func generateQRData(user *models.UserProfile) string {
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

// MobileCardHandler redirects to the main CardHandler for backward compatibility
func MobileCardHandler(c *gin.Context) {
	// Simply use the main CardHandler which now uses responsive design
	CardHandler(c)
}

// DesktopCardHandler redirects to the main CardHandler for backward compatibility
func DesktopCardHandler(c *gin.Context) {
	// Simply use the main CardHandler which now uses responsive design
	CardHandler(c)
}
