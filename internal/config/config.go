package config

import (
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	Port              string
	BooksDir          string
	INPXPath          string
	BasicAuthEnabled  bool
	BasicAuthUser     string
	BasicAuthPass     string
	CatalogTitle      string
	OPDS2Enabled      bool
	PageSize          int
	LogLevel          string
	CacheDir          string
	DatabasePath      string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Port:              getEnvOrDefault("PORT", "9090"),
		BooksDir:          getEnvOrDefault("BOOKS_DIR", "./books"),
		INPXPath:          getEnvOrDefault("INPX_PATH", "./sample-data/flibusta_fb2_local.inpx"),
		BasicAuthEnabled:  getEnvBool("BASIC_AUTH_ENABLED", false),
		BasicAuthUser:     getEnvOrDefault("BASIC_AUTH_USER", "reader"),
		BasicAuthPass:     getEnvOrDefault("BASIC_AUTH_PASS", "secret"),
		CatalogTitle:      getEnvOrDefault("CATALOG_TITLE", "Pushkinlib"),
		OPDS2Enabled:      getEnvBool("OPDS2_ENABLED", false),
		PageSize:          getEnvInt("PAGE_SIZE", 30),
		LogLevel:          getEnvOrDefault("LOG_LEVEL", "info"),
		CacheDir:          getEnvOrDefault("CACHE_DIR", "./cache"),
		DatabasePath:      getEnvOrDefault("DATABASE_PATH", "./cache/pushkinlib.db"),
	}
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool returns environment variable as boolean or default
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvInt returns environment variable as int or default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}