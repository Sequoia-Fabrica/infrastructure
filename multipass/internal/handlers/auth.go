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
		AccessPermissions: getAccessPermissions(userProfile.AccessLevel),
	}

	c.JSON(http.StatusOK, gin.H{
		"user":       userProfile,
		"membership": membership,
	})
}

// getAccessPermissions returns access permissions based on user level
func getAccessPermissions(level models.UserLevel) []string {
	switch level {
	case models.LimitedVolunteer:
		return []string{
			"Basic workspace access",
			"Supervised 3D printer use",
			"Hand tools access",
			"Common area access",
		}
	case models.FullMember:
		return []string{
			"Full workspace access",
			"Independent equipment use",
			"3D printer access",
			"Laser cutter access",
			"Electronics workbench",
			"Woodworking tools",
			"24/7 access",
		}
	case models.Staff:
		return []string{
			"All member permissions",
			"Equipment training authorization",
			"New member orientation",
			"Maintenance access",
			"Administrative tools",
		}
	case models.Admin:
		return []string{
			"Full administrative access",
			"System configuration",
			"User management",
			"Equipment management",
			"Financial access",
		}
	default:
		return []string{"No access"}
	}
}
