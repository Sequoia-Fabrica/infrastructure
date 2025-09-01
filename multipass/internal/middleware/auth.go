package middleware

import (
	"fmt"
	"multipass/internal/config"
	"multipass/internal/models"
	"multipass/internal/services"
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
		authentikUID := c.GetHeader("X-Authentik-Uid") // Extract the Authentik UID

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

		// Create initial user profile
		userProfile := &models.UserProfile{
			Email:       email,
			FullName:    fullName,
			Groups:      groups,
			MemberID:    "TBD", // Will be updated when we fetch from Authentik API
			AccessLevel: models.DetermineUserLevel(groups),
			Avatar:      &gravatarURL,
			AuthentikID: authentikUID,
		}
		
		// Try to get the user's PK from Authentik API
		if email != "" {
			// Create Authentik client using config
			cfg := config.Load()
			authentikClient := services.NewAuthentikClient(cfg)
			
			// Look up user by email
			apiUserProfile, err := authentikClient.GetUserByEmail(email)
			if err == nil && apiUserProfile != nil {
				// Update the Member ID with the PK from the API
				userProfile.MemberID = apiUserProfile.MemberID
				fmt.Printf("[AUTH] Updated Member ID to %s from Authentik API\n", apiUserProfile.MemberID)
			} else if err != nil {
				fmt.Printf("[AUTH] Error getting user from Authentik API: %v\n", err)
			}
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
