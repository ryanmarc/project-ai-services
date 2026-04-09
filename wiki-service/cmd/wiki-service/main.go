package main

import (
	"fmt"
	"log"
	"os"

	"github.com/IBM/project-ai-services/wiki-service/internal/llm"
	"github.com/IBM/project-ai-services/wiki-service/internal/wiki"
	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
)

func main() {
	// Load configuration from environment variables
	config := loadConfig()

	// Initialize wiki manager
	wikiManager, err := wiki.NewManager(config.Wiki.DataDir)
	if err != nil {
		log.Fatalf("Failed to initialize wiki manager: %v", err)
	}

	// Initialize LLM client
	llmClient := llm.NewClient(config.LLM)

	// Test LLM health
	if err := llmClient.Health(); err != nil {
		log.Printf("Warning: LLM health check failed: %v", err)
	} else {
		log.Println("LLM client initialized successfully")
	}

	// Print wiki stats
	stats, err := wikiManager.GetStats()
	if err != nil {
		log.Printf("Warning: Failed to get wiki stats: %v", err)
	} else {
		fmt.Printf("\nWiki Statistics:\n")
		fmt.Printf("  Total Sources: %d\n", stats.TotalSources)
		fmt.Printf("  Total Pages: %d\n", stats.TotalPages)
		fmt.Printf("  Total Entities: %d\n", stats.TotalEntities)
		fmt.Printf("  Total Concepts: %d\n", stats.TotalConcepts)
		fmt.Printf("  Total Queries: %d\n", stats.TotalQueries)
		fmt.Printf("  Last Updated: %s\n", stats.LastUpdated.Format("2006-01-02 15:04:05"))
	}

	fmt.Println("\nWiki service initialized successfully!")
	fmt.Printf("Data directory: %s\n", config.Wiki.DataDir)
	fmt.Printf("LLM endpoint: %s\n", config.LLM.Endpoint)
	fmt.Printf("LLM model: %s\n", config.LLM.Model)
}

// Config holds all configuration
type Config struct {
	Wiki types.WikiConfig
	LLM  types.LLMConfig
}

// loadConfig loads configuration from environment variables
func loadConfig() Config {
	return Config{
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
	}
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
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvFloat gets a float environment variable or returns a default value
func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		var floatValue float64
		if _, err := fmt.Sscanf(value, "%f", &floatValue); err == nil {
			return floatValue
		}
	}
	return defaultValue
}
