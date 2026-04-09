package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
)

// Config holds all application configuration
type Config struct {
	Wiki types.WikiConfig
	LLM  types.LLMConfig
	API  APIConfig
}

// APIConfig holds API server configuration
type APIConfig struct {
	Host string
	Port int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		Wiki: types.WikiConfig{
			DataDir:          getEnv("WIKI_DATA_DIR", "./wiki-data"),
			MaxPagesPerQuery: getEnvInt("WIKI_MAX_PAGES_PER_QUERY", 10),
			IndexSearchLimit: getEnvInt("WIKI_INDEX_SEARCH_LIMIT", 20),
			LogLevel:         getEnv("LOG_LEVEL", "info"),
		},
		LLM: types.LLMConfig{
			Endpoint:    getEnv("LLM_ENDPOINT", "http://localhost:8000"),
			Model:       getEnv("LLM_MODEL", "ibm-granite/granite-3.3-8b-instruct"),
			MaxTokens:   getEnvInt("LLM_MAX_TOKENS", 4096),
			Temperature: getEnvFloat("LLM_TEMPERATURE", 0.2),
		},
		API: APIConfig{
			Host: getEnv("API_HOST", "0.0.0.0"),
			Port: getEnvInt("API_PORT", 8080),
		},
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate Wiki config
	if c.Wiki.DataDir == "" {
		return fmt.Errorf("WIKI_DATA_DIR cannot be empty")
	}
	if c.Wiki.MaxPagesPerQuery < 1 {
		return fmt.Errorf("WIKI_MAX_PAGES_PER_QUERY must be at least 1")
	}
	if c.Wiki.IndexSearchLimit < 1 {
		return fmt.Errorf("WIKI_INDEX_SEARCH_LIMIT must be at least 1")
	}

	// Validate LLM config
	if c.LLM.Endpoint == "" {
		return fmt.Errorf("LLM_ENDPOINT cannot be empty")
	}
	if c.LLM.Model == "" {
		return fmt.Errorf("LLM_MODEL cannot be empty")
	}
	if c.LLM.MaxTokens < 1 {
		return fmt.Errorf("LLM_MAX_TOKENS must be at least 1")
	}
	if c.LLM.Temperature < 0 || c.LLM.Temperature > 2 {
		return fmt.Errorf("LLM_TEMPERATURE must be between 0 and 2")
	}

	// Validate API config
	if c.API.Port < 1 || c.API.Port > 65535 {
		return fmt.Errorf("API_PORT must be between 1 and 65535")
	}

	return nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable or returns a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvFloat gets a float environment variable or returns a default value
func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}
