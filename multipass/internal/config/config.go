package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// GroupMappingConfig defines the mapping between Authentik groups and access levels
type GroupMappingConfig struct {
	Mappings     map[string]string `yaml:"mappings"`      // Maps Authentik group names to access levels
	DefaultLevel string            `yaml:"default_level"` // Default access level if no matching groups found
}

// Config holds application configuration
type Config struct {
	// Server configuration
	Port        string
	BindAddress string
	Environment string
	DebugMode   bool   // Enable debug logging

	// Authentik integration
	AuthentikURL        string
	AuthentikAPIToken   string
	TrustedProxyHeaders bool
	GroupMappingPath    string
	GroupMappingConfig  *GroupMappingConfig

	// Application settings
	MakerspaceName string
	LogoURL        string

	// Security settings
	CSRFEnabled bool
	RateLimit   int
	TokenSecret string // Secret key for HMAC token generation and verification
}

// Load loads configuration from environment variables
func Load() *Config {
	cfg := &Config{
		Port:        getEnv("PORT", "3000"),
		BindAddress: getEnv("BIND_ADDRESS", "0.0.0.0"),
		Environment: getEnv("ENVIRONMENT", "development"),
		DebugMode:   getBoolEnv("DEBUG_MODE", false),

		AuthentikURL:        getEnv("AUTHENTIK_URL", "https://login.sequoia.garden"),
		AuthentikAPIToken:   getEnv("AUTHENTIK_API_TOKEN", ""),
		TrustedProxyHeaders: getBoolEnv("TRUSTED_PROXY_HEADERS", true),
		GroupMappingPath:    getEnv("GROUP_MAPPING_CONFIG", "./config/group_mapping.yaml"),

		MakerspaceName: getEnv("MAKERSPACE_NAME", "Sequoia Fabrica"),
		LogoURL:        getEnv("MAKERSPACE_LOGO_URL", "/static/images/logo.png"),

		CSRFEnabled: getBoolEnv("CSRF_ENABLED", true),
		RateLimit:   getIntEnv("RATE_LIMIT", 100),
		TokenSecret: getEnv("TOKEN_SECRET", ""),
	}

	// Check if Authentik API token is specified
	if cfg.AuthentikAPIToken == "" {
		log.Fatalf("Error: AUTHENTIK_API_TOKEN environment variable not specified")
	}

	// Check if token secret is specified
	if cfg.TokenSecret == "" {
		log.Fatalf("Error: TOKEN_SECRET environment variable not specified")
	}

	// Check if group mapping path is specified
	if cfg.GroupMappingPath == "" {
		log.Fatalf("Error: GROUP_MAPPING_CONFIG environment variable not specified")
	}

	// Load group mapping configuration
	groupConfig, err := LoadGroupMapping(cfg.GroupMappingPath)
	if err != nil {
		// All errors (including file not found) are fatal
		log.Fatalf("Error loading group mapping config from %s: %v", cfg.GroupMappingPath, err)
	}
	cfg.GroupMappingConfig = groupConfig

	return cfg
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

// LoadGroupMapping loads group mapping configuration from a YAML file
func LoadGroupMapping(configPath string) (*GroupMappingConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err

	}

	var config GroupMappingConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	// Check if mappings is empty
	if len(config.Mappings) == 0 {
		return nil, fmt.Errorf("no group mappings found in config file: %s", configPath)
	}

	// Set default level to NoAccess if not specified
	if config.DefaultLevel == "" {
		config.DefaultLevel = "NoAccess"
	}

	return &config, nil
}
