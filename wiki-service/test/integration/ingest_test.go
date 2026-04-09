package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IBM/project-ai-services/wiki-service/internal/ingest"
	"github.com/IBM/project-ai-services/wiki-service/internal/llm"
	"github.com/IBM/project-ai-services/wiki-service/internal/wiki"
	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIngestIntegration tests the full ingest pipeline
// Note: This test requires a running LLM endpoint
func TestIngestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if LLM endpoint is configured
	llmEndpoint := os.Getenv("LLM_ENDPOINT")
	if llmEndpoint == "" {
		t.Skip("LLM_ENDPOINT not set, skipping integration test")
	}

	// Create temporary wiki directory
	tmpDir := t.TempDir()

	// Initialize wiki manager
	wikiMgr, err := wiki.NewManager(tmpDir)
	require.NoError(t, err)

	// Initialize LLM client
	llmConfig := types.LLMConfig{
		Endpoint:    llmEndpoint,
		Model:       os.Getenv("LLM_MODEL"),
		MaxTokens:   4096,
		Temperature: 0.2,
	}
	if llmConfig.Model == "" {
		llmConfig.Model = "ibm-granite/granite-3.3-8b-instruct"
	}

	llmClient := llm.NewClient(llmConfig)

	// Test LLM health
	err = llmClient.Health()
	if err != nil {
		t.Skipf("LLM endpoint not healthy: %v", err)
	}

	// Initialize ingest engine
	ingestEngine := ingest.NewEngine(wikiMgr, llmClient)

	// Get test fixture path
	fixturesDir := filepath.Join("..", "fixtures")
	testFile := filepath.Join(fixturesDir, "sample.txt")

	// Verify test file exists
	_, err = os.Stat(testFile)
	require.NoError(t, err, "Test fixture not found")

	t.Run("Ingest single document", func(t *testing.T) {
		request := types.IngestRequest{
			SourcePath:  testFile,
			SourceType:  "text",
			Interactive: false,
		}

		response, err := ingestEngine.Ingest(request)
		require.NoError(t, err)

		// Verify response
		assert.NotEmpty(t, response.Summary)
		assert.NotEmpty(t, response.PagesCreated)
		assert.NotEmpty(t, response.LogEntry)

		// Verify at least one page was created
		assert.Greater(t, len(response.PagesCreated), 0)

		// Verify entities or concepts were found
		totalFound := len(response.EntitiesFound) + len(response.ConceptsFound)
		assert.Greater(t, totalFound, 0, "Should find at least one entity or concept")

		t.Logf("Summary: %s", response.Summary)
		t.Logf("Pages created: %v", response.PagesCreated)
		t.Logf("Entities found: %v", response.EntitiesFound)
		t.Logf("Concepts found: %v", response.ConceptsFound)
	})

	t.Run("Verify wiki structure", func(t *testing.T) {
		// Check that wiki directories exist
		wikiDir := filepath.Join(tmpDir, "wiki")
		assert.DirExists(t, wikiDir)
		assert.DirExists(t, filepath.Join(wikiDir, "sources"))
		assert.DirExists(t, filepath.Join(wikiDir, "entities"))
		assert.DirExists(t, filepath.Join(wikiDir, "concepts"))

		// Check that index exists and has content
		indexPath := filepath.Join(wikiDir, "index.md")
		assert.FileExists(t, indexPath)

		indexContent, err := os.ReadFile(indexPath)
		require.NoError(t, err)
		assert.Contains(t, string(indexContent), "Wiki Index")
		assert.Contains(t, string(indexContent), "Statistics")

		// Check that log exists and has content
		logPath := filepath.Join(wikiDir, "log.md")
		assert.FileExists(t, logPath)

		logContent, err := os.ReadFile(logPath)
		require.NoError(t, err)
		assert.Contains(t, string(logContent), "Wiki Activity Log")
		assert.Contains(t, string(logContent), "ingest")
	})

	t.Run("Verify created pages", func(t *testing.T) {
		// List all pages
		pages, err := wikiMgr.ListPages("all")
		require.NoError(t, err)
		assert.NotEmpty(t, pages)

		// Verify at least source page exists
		sourcePages, err := wikiMgr.ListPages("sources")
		require.NoError(t, err)
		assert.NotEmpty(t, sourcePages, "Should have at least one source page")

		// Read a source page to verify content
		if len(sourcePages) > 0 {
			content, err := wikiMgr.ReadPage(sourcePages[0])
			require.NoError(t, err)
			assert.NotEmpty(t, content)
			assert.Contains(t, content, "#") // Should have markdown headers
		}
	})

	t.Run("Verify index updates", func(t *testing.T) {
		stats, err := wikiMgr.GetStats()
		require.NoError(t, err)

		assert.Greater(t, stats.TotalPages, 0)
		assert.Greater(t, stats.TotalSources, 0)

		t.Logf("Wiki stats: %+v", stats)
	})

	t.Run("Verify log entries", func(t *testing.T) {
		logs, err := wikiMgr.GetRecentLogs(5)
		require.NoError(t, err)
		assert.NotEmpty(t, logs)

		// Verify log has ingest entry
		foundIngest := false
		for _, log := range logs {
			if log.Type == "ingest" {
				foundIngest = true
				assert.NotEmpty(t, log.Title)
				assert.NotEmpty(t, log.Details)
				break
			}
		}
		assert.True(t, foundIngest, "Should have at least one ingest log entry")
	})
}

// TestIngestMultipleDocuments tests ingesting multiple documents
func TestIngestMultipleDocuments(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	llmEndpoint := os.Getenv("LLM_ENDPOINT")
	if llmEndpoint == "" {
		t.Skip("LLM_ENDPOINT not set, skipping integration test")
	}

	tmpDir := t.TempDir()
	wikiMgr, err := wiki.NewManager(tmpDir)
	require.NoError(t, err)

	llmConfig := types.LLMConfig{
		Endpoint:    llmEndpoint,
		Model:       os.Getenv("LLM_MODEL"),
		MaxTokens:   4096,
		Temperature: 0.2,
	}
	if llmConfig.Model == "" {
		llmConfig.Model = "ibm-granite/granite-3.3-8b-instruct"
	}

	llmClient := llm.NewClient(llmConfig)
	ingestEngine := ingest.NewEngine(wikiMgr, llmClient)

	fixturesDir := filepath.Join("..", "fixtures")
	testFiles := []string{
		filepath.Join(fixturesDir, "sample.txt"),
		filepath.Join(fixturesDir, "machine-learning.txt"),
	}

	// Ingest first document
	t.Run("Ingest first document", func(t *testing.T) {
		request := types.IngestRequest{
			SourcePath:  testFiles[0],
			SourceType:  "text",
			Interactive: false,
		}

		response, err := ingestEngine.Ingest(request)
		require.NoError(t, err)
		assert.NotEmpty(t, response.PagesCreated)

		t.Logf("First document - Pages created: %d", len(response.PagesCreated))
	})

	// Get stats after first ingest
	stats1, err := wikiMgr.GetStats()
	require.NoError(t, err)

	// Ingest second document
	t.Run("Ingest second document", func(t *testing.T) {
		request := types.IngestRequest{
			SourcePath:  testFiles[1],
			SourceType:  "text",
			Interactive: false,
		}

		response, err := ingestEngine.Ingest(request)
		require.NoError(t, err)
		assert.NotEmpty(t, response.PagesCreated)

		t.Logf("Second document - Pages created: %d", len(response.PagesCreated))
	})

	// Get stats after second ingest
	stats2, err := wikiMgr.GetStats()
	require.NoError(t, err)

	// Verify knowledge accumulation
	t.Run("Verify knowledge accumulation", func(t *testing.T) {
		assert.Greater(t, stats2.TotalPages, stats1.TotalPages, "Total pages should increase")
		assert.Greater(t, stats2.TotalSources, stats1.TotalSources, "Total sources should increase")

		t.Logf("Stats after first ingest: %+v", stats1)
		t.Logf("Stats after second ingest: %+v", stats2)
	})
}
