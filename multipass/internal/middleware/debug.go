package middleware

import (
	"log"
	"multipass/internal/config"
	"multipass/internal/utils"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// DebugAuthMiddleware adds mock Authentik headers for local development and testing
// and collects debug information to display in templates
func DebugAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if debug mode is enabled via environment variable
		debugMode := os.Getenv("DEBUG_MODE") == "true"
		
		// Only apply mock headers in development mode
		cfg := config.Load()
		if cfg.IsDevelopment() && c.GetHeader("X-Authentik-Email") == "" {
			// Add mock Authentik headers for testing
			testEmail := "test.user@example.com"
			c.Request.Header.Set("X-Authentik-Email", testEmail)
			c.Request.Header.Set("X-Authentik-Given-Name", "Test")
			c.Request.Header.Set("X-Authentik-Family-Name", "User")
			
			// Use the Members group from group_mapping.yaml
			c.Request.Header.Set("X-Authentik-Groups", "Members")
			
			// Generate Gravatar URL for test user
			gravatarURL := utils.GenerateGravatarURL(testEmail, 256, "identicon")
			c.Set("debug_avatar", gravatarURL)
			
			log.Printf("[DEBUG] Added mock Authentik headers for path: %s", c.Request.URL.Path)
		} else if c.GetHeader("X-Authentik-Email") != "" {
			log.Println("Headers already present, skipping debug middleware")
		}

		// Always collect debug information if debug mode is enabled
		if debugMode || cfg.IsDevelopment() {
			// Collect all headers for debugging
			debugInfo := map[string]string{
				"X-Authentik-Email":      c.GetHeader("X-Authentik-Email"),
				"X-Authentik-Given-Name": c.GetHeader("X-Authentik-Given-Name"),
				"X-Authentik-Family-Name": c.GetHeader("X-Authentik-Family-Name"),
				"X-Authentik-Groups":    c.GetHeader("X-Authentik-Groups"),
			}
			
			// Add debug info to context for templates
			c.Set("debug_mode", true)
			c.Set("debug_headers", debugInfo)
			
			// Log debug information
			log.Printf("[DEBUG] Headers: Email=%s, Name=%s %s, Groups=%s", 
				debugInfo["X-Authentik-Email"],
				debugInfo["X-Authentik-Given-Name"],
				debugInfo["X-Authentik-Family-Name"],
				debugInfo["X-Authentik-Groups"])
			
			// Parse groups for debugging
			groups := []string{}
			if groupsHeader := debugInfo["X-Authentik-Groups"]; groupsHeader != "" {
				// First try splitting by pipe, which is what we see in the actual headers
				if strings.Contains(groupsHeader, "|") {
					groups = strings.Split(groupsHeader, "|")
				} else {
					// Fall back to comma if no pipes found
					groups = strings.Split(groupsHeader, ",")
				}
				
				// Trim spaces from each group
				for i, group := range groups {
					groups[i] = strings.TrimSpace(group)
				}
				
				// Log the parsed groups
				log.Printf("[DEBUG] Parsed groups: %v", groups)
			}
			c.Set("debug_groups", groups)
		}

		c.Next()
	}
}

// GetDebugInfo returns debug information from the context if available
func GetDebugInfo(c *gin.Context) map[string]interface{} {
	debugMode, exists := c.Get("debug_mode")
	if !exists || !debugMode.(bool) {
		return nil
	}
	
	debugInfo := map[string]interface{}{
		"headers": c.MustGet("debug_headers"),
	}
	
	if groups, exists := c.Get("debug_groups"); exists {
		debugInfo["groups"] = groups
	}
	
	if avatar, exists := c.Get("debug_avatar"); exists {
		debugInfo["avatar"] = avatar
	}
	
	return debugInfo
}
