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
	"github.com/gin-contrib/multitemplate"
)

// createTemplateRenderer creates a custom HTML renderer that properly handles template inheritance
func createTemplateRenderer() multitemplate.Renderer {
	r := multitemplate.NewRenderer()
	templateDir := "web/templates"

	// Define template functions
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
	}

	// Get all template files
	templateFiles, err := filepath.Glob(filepath.Join(templateDir, "*.html"))
	if err != nil {
		log.Fatal("Failed to load templates:", err)
	}

	// Find base template
	var baseFile string
	for _, file := range templateFiles {
		if filepath.Base(file) == "base.html" {
			baseFile = file
			break
		}
	}

	if baseFile == "" {
		log.Fatal("Base template not found")
	}

	// Add each template with base
	for _, file := range templateFiles {
		fileName := filepath.Base(file)
		if fileName == "base.html" {
			continue
		}

		// Load both templates together
		tmplFiles := []string{baseFile, file}

		// Create a new template instance for each page
		tmpl, err := template.New(filepath.Base(baseFile)).Funcs(funcMap).ParseFiles(tmplFiles...)
		if err != nil {
			log.Fatalf("Failed to parse template %s: %v", fileName, err)
		}

		// Add to renderer with the full filename
		r.Add(fileName, tmpl)

		log.Printf("Added template: %s", fileName)
	}

	return r
}

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

	// Load HTML templates with proper inheritance
	r.HTMLRender = createTemplateRenderer()

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
		
		// Public token-based routes
		publicToken := public.Group("/public")
		publicToken.Use(middleware.DebugAuthMiddleware()) // Add debug middleware
		publicToken.Use(middleware.TokenAuthMiddleware()) // Add token auth middleware
		{
			publicToken.GET("/card", handlers.PublicCardHandler)
		}
	}

	// Protected routes (require authentication)
	protected := r.Group("/")
	protected.Use(middleware.DebugAuthMiddleware()) // Add debug middleware before auth
	protected.Use(middleware.AuthMiddleware())
	{
		// Root route redirects to card
		protected.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusTemporaryRedirect, "/card")
		})

		// Card routes
		protected.GET("/card", handlers.CardHandler)

		// Profile and API routes
		protected.GET("/profile", handlers.ProfileHandler)
		
		// Token generation route
		protected.GET("/generate-token", middleware.GenerateTokenHandler)
		protected.GET("/share", handlers.GenerateTokenLinkHandler)
	}

	// API routes
	api := r.Group("/api/v1")
	api.Use(middleware.DebugAuthMiddleware()) // Add debug middleware before auth
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
