package handlers

import (
	"multipass/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// LoginHandler handles SSO login requests
func LoginHandler(c *gin.Context) {
	// Since we're using reverse proxy authentication,
	// this handler mainly serves the login page for users not authenticated
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title":          "Sequoia Fabrica - Login",
		"makerspace_name": "Sequoia Fabrica",
	})
}

// ProfileHandler displays user profile information
func ProfileHandler(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	userProfile := user.(*models.UserProfile)
	
	// Create membership info
	membership := &models.MembershipInfo{
		MembershipType:    userProfile.AccessLevel.String(),
		Status:           models.StatusActive,
		UserLevel:        userProfile.AccessLevel,
	}

	c.JSON(http.StatusOK, gin.H{
		"user":       userProfile,
		"membership": membership,
	})
}
