package main

import (
	"html/template"
	"multipass/internal/badge"
	"multipass/internal/config"
	"multipass/internal/handlers"
	"multipass/internal/middleware"
	"multipass/internal/services"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-gonic/gin"
)

// createTemplateRenderer creates a custom HTML renderer that properly handles template inheritance
func createTemplateRenderer(logger *services.Logger) multitemplate.Renderer {
	r := multitemplate.NewRenderer()
	templateDir := "web/templates"

	// Define template functions
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
	}

	// Get all template files
	templateFiles, err := filepath.Glob(filepath.Join(templateDir, "*.html"))
	if err != nil {
		logger.Fatal("Failed to load templates: %v", err)
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
		logger.Fatal("Base template not found")
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
			logger.Fatal("Failed to parse template %s: %v", fileName, err)
		}

		// Add to renderer with the full filename
		r.Add(fileName, tmpl)

		logger.Debug("Added template: %s", fileName)
	}

	return r
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Create logger
	logger := services.NewLogger(cfg)

	// Set Gin mode based on environment
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Open the record of issued wooden badges
	badgeStore, err := badge.OpenStore(cfg.BadgeDB)
	if err != nil {
		logger.Fatal("Failed to open badge store: %v", err)
	}
	r := newRouter(cfg, logger, badgeStore)

	// Start server
	logger.Info("Starting Multipass server on %s", cfg.GetServerAddress())
	logger.Info("Environment: %s", cfg.Environment)
	logger.Info("Makerspace: %s", cfg.MakerspaceName)

	if err := r.Run(cfg.GetServerAddress()); err != nil {
		logger.Fatal("Failed to start server: %v", err)
	}
}

// newRouter sets up every route: public pages, the Authentik-protected app,
// and the API.
func newRouter(cfg *config.Config, logger *services.Logger, badgeStore *badge.Store) *gin.Engine {
	badgeHandlers := &handlers.BadgeHandlers{Store: badgeStore}

	// Create Gin router
	r := gin.Default()

	// Add template functions
	r.SetFuncMap(template.FuncMap{
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
	})

	// Load HTML templates with proper inheritance
	r.HTMLRender = createTemplateRenderer(logger)

	// Serve static files. Browsers must check back before reusing one (a 304
	// when unchanged): without this they guess a freshness from the file's
	// age and keep serving stale CSS and JS for weeks after a deploy
	static := r.Group("/static", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
	})
	static.Static("/", "./web/static")

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

		// Wooden badges: the QR code is engraved in uppercase, so take both
		public.GET("/B/:serial/:sig", middleware.BadgeAuthMiddleware(badgeStore), handlers.PublicCardHandler)
		public.GET("/b/:serial/:sig", middleware.BadgeAuthMiddleware(badgeStore), handlers.PublicCardHandler)
		// The DID that signs badge barcodes, and its list of voided badges
		public.GET("/.well-known/did.json", handlers.DIDDocument)
		public.GET("/badge-status/revocation/:list", badgeHandlers.StatusList)
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

		// Wooden badges: make one, download its laser files, report it
		// failed or lost
		badges := protected.Group("/")
		badges.Use(middleware.SameOriginMiddleware())
		badges.GET("/badge", badgeHandlers.Page)
		badges.POST("/badge", badgeHandlers.Issue)
		badges.GET("/badge/preview.svg", badgeHandlers.Preview)
		badges.GET("/badge/preview-back.svg", badgeHandlers.PreviewBack)
		badges.GET("/badges/:serial/front.svg", badgeHandlers.Front)
		badges.GET("/badges/:serial/back.svg", badgeHandlers.Back)
		badges.POST("/badges/:serial/failed", badgeHandlers.Void(badge.Failed))
		badges.POST("/badges/:serial/lost", badgeHandlers.Void(badge.Lost))
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

	return r
}
