package middleware

import (
	"log"
	"multipass/internal/config"

	"github.com/gin-gonic/gin"
)

// DebugAuthMiddleware adds mock Authentik headers for local development and testing
func DebugAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only apply in development mode
		cfg := config.Load()
		if !cfg.IsDevelopment() {
			log.Println("Not in development mode, skipping debug middleware")
			c.Next()
			return
		}

		// Skip if headers are already present
		if c.GetHeader("X-Authentik-Email") != "" {
			log.Println("Headers already present, skipping debug middleware")
			c.Next()
			return
		}

		// Add mock Authentik headers for testing
		c.Request.Header.Set("X-Authentik-Email", "test.user@example.com")
		c.Request.Header.Set("X-Authentik-Given-Name", "Test")
		c.Request.Header.Set("X-Authentik-Family-Name", "User")
		c.Request.Header.Set("X-Authentik-Groups", "members-full,volunteers-limited")
		
		// Log that we've added debug headers
		c.Set("debug_mode", true)
		log.Printf("[DEBUG] Added mock Authentik headers for path: %s", c.Request.URL.Path)
		log.Printf("[DEBUG] Headers: Email=%s, Name=%s %s, Groups=%s", 
			c.GetHeader("X-Authentik-Email"),
			c.GetHeader("X-Authentik-Given-Name"),
			c.GetHeader("X-Authentik-Family-Name"),
			c.GetHeader("X-Authentik-Groups"))

		c.Next()
	}
}
