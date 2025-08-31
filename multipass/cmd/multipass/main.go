package main

import (
	"html/template"
	"log"
	"multipass/internal/config"
	"multipass/internal/handlers"
	"multipass/internal/middleware"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Set Gin mode based on environment
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	r := gin.Default()

	// Add template functions
	r.SetFuncMap(template.FuncMap{
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
	})

	// Load HTML templates
	templatePath := filepath.Join("web", "templates", "*.html")
	r.LoadHTMLGlob(templatePath)

	// Serve static files
	r.Static("/static", "./web/static")

	// Apply middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// CORS middleware for development
	if cfg.IsDevelopment() {
		r.Use(func(c *gin.Context) {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
			
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}
			
			c.Next()
		})
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "multipass",
			"version": "1.0.0",
		})
	})

	// Public routes (no authentication required)
	public := r.Group("/")
	{
		public.GET("/login", handlers.LoginHandler)
	}

	// Protected routes (require authentication)
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware())
	{
		// Root route redirects to card
		protected.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusTemporaryRedirect, "/card")
		})

		// Card routes
		protected.GET("/card", handlers.CardHandler)
		protected.GET("/card/mobile", handlers.MobileCardHandler)
		protected.GET("/card/desktop", handlers.DesktopCardHandler)

		// Profile and API routes
		protected.GET("/profile", handlers.ProfileHandler)
	}

	// API routes
	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware())
	{
		api.GET("/user", handlers.ProfileHandler)
		api.GET("/health", func(c *gin.Context) {
			user, exists := c.Get("user")
			if exists {
				c.JSON(http.StatusOK, gin.H{
					"status": "authenticated",
					"user":   user,
				})
			} else {
				c.JSON(http.StatusOK, gin.H{
					"status": "unauthenticated",
				})
			}
		})
	}

	// 404 handler
	r.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "login.html", gin.H{
			"title":           "Page Not Found - " + cfg.MakerspaceName,
			"makerspace_name": cfg.MakerspaceName,
			"error":           "The page you're looking for doesn't exist.",
		})
	})

	// Start server
	log.Printf("Starting Multipass server on %s", cfg.GetServerAddress())
	log.Printf("Environment: %s", cfg.Environment)
	log.Printf("Makerspace: %s", cfg.MakerspaceName)
	
	if err := r.Run(cfg.GetServerAddress()); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
