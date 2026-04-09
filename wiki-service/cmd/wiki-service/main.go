package main

import (
	"fmt"
	"log"
	"os"

	"github.com/IBM/project-ai-services/wiki-service/internal/ingest"
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

	// Parse command line arguments
	if len(os.Args) > 1 {
		command := os.Args[1]
		switch command {
		case "ingest":
			handleIngest(wikiManager, llmClient, os.Args[2:])
			return
		case "stats":
			handleStats(wikiManager)
			return
		case "help":
			printHelp()
			return
		default:
			fmt.Printf("Unknown command: %s\n", command)
			printHelp()
			os.Exit(1)
		}
	}

	// Default: print stats
	handleStats(wikiManager)
	fmt.Println("\nWiki service initialized successfully!")
	fmt.Printf("Data directory: %s\n", config.Wiki.DataDir)
	fmt.Printf("LLM endpoint: %s\n", config.LLM.Endpoint)
	fmt.Printf("LLM model: %s\n", config.LLM.Model)
	fmt.Println("\nRun 'wiki-service help' for usage information")
}

// handleIngest processes document ingestion
func handleIngest(wikiManager *wiki.Manager, llmClient *llm.Client, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: No file specified")
		fmt.Println("Usage: wiki-service ingest <file-path>")
		os.Exit(1)
	}

	filePath := args[0]

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("File not found: %s", filePath)
	}

	fmt.Printf("Ingesting document: %s\n", filePath)

	// Initialize ingest engine
	ingestEngine := ingest.NewEngine(wikiManager, llmClient)

	// Create ingest request
	request := types.IngestRequest{
		SourcePath:  filePath,
		SourceType:  "text",
		Interactive: false,
	}

	// Perform ingestion
	response, err := ingestEngine.Ingest(request)
	if err != nil {
		log.Fatalf("Ingestion failed: %v", err)
	}

	// Print results
	fmt.Println("\n✓ Ingestion completed successfully!")
	fmt.Printf("\nSummary:\n%s\n", response.Summary)
	fmt.Printf("\nPages created: %d\n", len(response.PagesCreated))
	for _, page := range response.PagesCreated {
		fmt.Printf("  - %s\n", page)
	}

	if len(response.EntitiesFound) > 0 {
		fmt.Printf("\nEntities found: %d\n", len(response.EntitiesFound))
		for _, entity := range response.EntitiesFound {
			fmt.Printf("  - %s\n", entity)
		}
	}

	if len(response.ConceptsFound) > 0 {
		fmt.Printf("\nConcepts found: %d\n", len(response.ConceptsFound))
		for _, concept := range response.ConceptsFound {
			fmt.Printf("  - %s\n", concept)
		}
	}

	fmt.Printf("\nLog entry: %s\n", response.LogEntry)
}

// handleStats prints wiki statistics
func handleStats(wikiManager *wiki.Manager) {
	stats, err := wikiManager.GetStats()
	if err != nil {
		log.Printf("Warning: Failed to get wiki stats: %v", err)
		return
	}

	fmt.Printf("\nWiki Statistics:\n")
	fmt.Printf("  Total Sources: %d\n", stats.TotalSources)
	fmt.Printf("  Total Pages: %d\n", stats.TotalPages)
	fmt.Printf("  Total Entities: %d\n", stats.TotalEntities)
	fmt.Printf("  Total Concepts: %d\n", stats.TotalConcepts)
	fmt.Printf("  Total Queries: %d\n", stats.TotalQueries)
	fmt.Printf("  Last Updated: %s\n", stats.LastUpdated.Format("2006-01-02 15:04:05"))
}

// printHelp prints usage information
func printHelp() {
	fmt.Println("Wiki Service - LLM-powered knowledge base")
	fmt.Println("\nUsage:")
	fmt.Println("  wiki-service [command] [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  ingest <file>  Ingest a document into the wiki")
	fmt.Println("  stats          Show wiki statistics")
	fmt.Println("  help           Show this help message")
	fmt.Println("\nExamples:")
	fmt.Println("  wiki-service ingest document.txt")
	fmt.Println("  wiki-service stats")
	fmt.Println("\nEnvironment Variables:")
	fmt.Println("  WIKI_DATA_DIR           Wiki data directory (default: ./wiki-data)")
	fmt.Println("  LLM_ENDPOINT            LLM API endpoint (default: http://localhost:8000)")
	fmt.Println("  LLM_MODEL               LLM model name (default: ibm-granite/granite-3.3-8b-instruct)")
	fmt.Println("  LLM_MAX_TOKENS          Max tokens for LLM (default: 4096)")
	fmt.Println("  LLM_TEMPERATURE         LLM temperature (default: 0.2)")
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
