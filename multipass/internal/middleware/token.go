package middleware

import (
	"multipass/internal/config"
	"multipass/internal/models"
	"multipass/internal/services"
	"multipass/internal/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// TokenAuthMiddleware validates a token in the URL and sets the user profile in the context
// This middleware is used for public routes that need user information without authentication
func TokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from query parameter
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token required"})
			c.Abort()
			return
		}

		// Get config
		cfg := config.Load()
		// Create logger
		logger := services.NewLogger(cfg)

		// Check if token secret is configured
		if cfg.TokenSecret == "" {
			logger.Error("Warning: TOKEN_SECRET is not configured")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Token verification not configured"})
			c.Abort()
			return
		}

		// Verify token
		tokenData, err := utils.VerifyToken(token, cfg.TokenSecret)
		if err != nil {
			logger.Error("Token verification failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Create Authentik client
		authentikClient := services.NewAuthentikClient(cfg)

		// Try to get user by ID first if it looks like a numeric ID
		var userProfile *models.UserProfile
		if tokenData.UserID != "" {
			// Only try GetUserByID if the ID looks numeric to avoid unnecessary 404s
			_, err := strconv.Atoi(tokenData.UserID)
			if err == nil {
				// ID is numeric, try to get user by ID
				userProfile, err = authentikClient.GetUserByID(tokenData.UserID)
				if err != nil {
					logger.Debug("Failed to get user by ID: %v", err)
					// Fall back to email lookup
				}
			} else {
				// ID is not numeric, skip the ID lookup to avoid 404
				logger.Debug("UserID %s is not numeric, skipping ID lookup", tokenData.UserID)
			}
		}

		// If user not found by ID, try by email
		if userProfile == nil && tokenData.Email != "" {
			userProfile, err = authentikClient.GetUserByEmail(tokenData.Email)
			if err != nil {
				logger.Error("Failed to get user by email: %v", err)
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				c.Abort()
				return
			}
		}

		// If still no user found, return error
		if userProfile == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		// Set user profile in context
		c.Set("user", userProfile)
		c.Set("token_auth", true) // Flag to indicate token-based authentication

		c.Next()
	}
}

// GenerateTokenHandler creates a secure token for the authenticated user
func GenerateTokenHandler(c *gin.Context) {
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

	// Get user ID from Authentik - use the numeric PK for consistent identification
	userID := user.MemberID // Use MemberID which is now set to the numeric PK
	// Fall back to AuthentikID or email if MemberID is not available
	if userID == "" || userID == "TBD" {
		userID = user.AuthentikID
		if userID == "" {
			userID = user.Email
		}
	}

	// Generate token
	token, err := utils.GenerateToken(userID, user.Email, cfg.TokenSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Return token
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"url":   c.Request.Host + "/public/card?token=" + token,
	})
}
