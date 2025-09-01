package middleware

import (
	"crypto/md5"
	"fmt"
	"multipass/internal/models"
	"multipass/internal/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware extracts user information from Authentik reverse proxy headers
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract user data from Authentik headers
		email := c.GetHeader("X-Authentik-Email")
		fullName := c.GetHeader("X-Authentik-Name")
		groupsHeader := c.GetHeader("X-Authentik-Groups")

		// If no email, user is not authenticated
		if email == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		// Parse groups
		var groups []string
		if groupsHeader != "" {
			// First try splitting by pipe, which is what we see in the actual headers
			if strings.Contains(groupsHeader, "|") {
				groups = strings.Split(groupsHeader, "|")
			} else {
				// Fall back to comma if no pipes found
				groups = strings.Split(groupsHeader, ",")
			}

			// Trim whitespace from group names
			for i, group := range groups {
				groups[i] = strings.TrimSpace(group)
			}

			// Log the parsed groups for debugging
			fmt.Printf("[AUTH] Parsed groups: %v\n", groups)
		}

		// Generate Gravatar URL for the user's email
		gravatarURL := utils.GenerateGravatarURL(email, 256, "identicon")

		// Create user profile
		userProfile := &models.UserProfile{
			Email:       email,
			FullName:    fullName,
			Groups:      groups,
			MemberID:    generateMemberID(email),
			AccessLevel: models.DetermineUserLevel(groups),
			Avatar:      &gravatarURL,
		}

		// Store user profile in context
		c.Set("user", userProfile)
		c.Next()
	}
}

// RequireAuth ensures user is authenticated
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		// Ensure user is active
		userProfile := user.(*models.UserProfile)
		if userProfile.Email == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user profile"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireLevel ensures user has minimum access level
func RequireLevel(minLevel models.UserLevel) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		userProfile := user.(*models.UserProfile)
		if userProfile.AccessLevel < minLevel {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient privileges"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// generateMemberID creates a unique member ID from email
func generateMemberID(email string) string {
	hash := md5.Sum([]byte(email))
	return fmt.Sprintf("SF-%X", hash[:4]) // SF for Sequoia Fabrica
}
