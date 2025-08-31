package handlers

import (
	"multipass/internal/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// CardHandler displays the digital ID card
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
		AccessPermissions: getAccessPermissions(userProfile.AccessLevel),
	}

	// Determine template based on user agent
	userAgent := c.GetHeader("User-Agent")
	template := "card_desktop.html"
	
	// Simple mobile detection
	if isMobileUserAgent(userAgent) {
		template = "card_mobile.html"
	}

	c.HTML(http.StatusOK, template, gin.H{
		"title":          "Digital ID Card - " + userProfile.GetFullName(),
		"user":           userProfile,
		"membership":     membership,
		"makerspace_name": "Sequoia Fabrica",
		"current_time":   time.Now().Format("January 2, 2006"),
		"qr_data":        generateQRData(userProfile),
	})
}

// MobileCardHandler explicitly serves mobile card layout
func MobileCardHandler(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusTemporaryRedirect, "/login")
		return
	}

	userProfile := user.(*models.UserProfile)
	membership := &models.MembershipInfo{
		MembershipType:    userProfile.AccessLevel.String(),
		Status:           models.StatusActive,
		UserLevel:        userProfile.AccessLevel,
		AccessPermissions: getAccessPermissions(userProfile.AccessLevel),
	}

	c.HTML(http.StatusOK, "card_mobile.html", gin.H{
		"title":          "Digital ID Card - " + userProfile.GetFullName(),
		"user":           userProfile,
		"membership":     membership,
		"makerspace_name": "Sequoia Fabrica",
		"current_time":   time.Now().Format("January 2, 2006"),
		"qr_data":        generateQRData(userProfile),
	})
}

// DesktopCardHandler explicitly serves desktop card layout
func DesktopCardHandler(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.Redirect(http.StatusTemporaryRedirect, "/login")
		return
	}

	userProfile := user.(*models.UserProfile)
	membership := &models.MembershipInfo{
		MembershipType:    userProfile.AccessLevel.String(),
		Status:           models.StatusActive,
		UserLevel:        userProfile.AccessLevel,
		AccessPermissions: getAccessPermissions(userProfile.AccessLevel),
	}

	c.HTML(http.StatusOK, "card_desktop.html", gin.H{
		"title":          "Digital ID Card - " + userProfile.GetFullName(),
		"user":           userProfile,
		"membership":     membership,
		"makerspace_name": "Sequoia Fabrica",
		"current_time":   time.Now().Format("January 2, 2006"),
		"qr_data":        generateQRData(userProfile),
	})
}

// isMobileUserAgent performs simple mobile user agent detection
func isMobileUserAgent(userAgent string) bool {
	mobileKeywords := []string{
		"Mobile", "Android", "iPhone", "iPad", "iPod",
		"BlackBerry", "Windows Phone", "Opera Mini",
	}
	
	for _, keyword := range mobileKeywords {
		if contains(userAgent, keyword) {
			return true
		}
	}
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
