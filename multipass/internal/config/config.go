package config

import (
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	// Server configuration
	Port         string
	BindAddress  string
	Environment  string

	// Authentik integration
	AuthentikURL      string
	AuthentikAPIToken string
	TrustedProxyHeaders bool

	// Application settings
	MakerspaceName string
	LogoURL        string
	
	// Security settings
	CSRFEnabled bool
	RateLimit   int
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		Port:         getEnv("PORT", "3000"),
		BindAddress:  getEnv("BIND_ADDRESS", "0.0.0.0"),
		Environment:  getEnv("ENVIRONMENT", "development"),

		AuthentikURL:        getEnv("AUTHENTIK_URL", "https://login.sequoia.garden"),
		AuthentikAPIToken:   getEnv("AUTHENTIK_API_TOKEN", ""),
		TrustedProxyHeaders: getBoolEnv("TRUSTED_PROXY_HEADERS", true),

		MakerspaceName: getEnv("MAKERSPACE_NAME", "Sequoia Fabrica"),
		LogoURL:        getEnv("MAKERSPACE_LOGO_URL", "/static/images/logo.png"),

		CSRFEnabled: getBoolEnv("CSRF_ENABLED", true),
		RateLimit:   getIntEnv("RATE_LIMIT", 100),
	}
}

// getEnv gets environment variable with default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getBoolEnv gets boolean environment variable with default fallback
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getIntEnv gets integer environment variable with default fallback
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// GetServerAddress returns the full server bind address
func (c *Config) GetServerAddress() string {
	return c.BindAddress + ":" + c.Port
}
