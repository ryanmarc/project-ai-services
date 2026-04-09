package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IBM/project-ai-services/wiki-service/internal/llm"
	"github.com/IBM/project-ai-services/wiki-service/internal/query"
	"github.com/IBM/project-ai-services/wiki-service/internal/wiki"
	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
)

// TestQueryIntegration tests the full query workflow
func TestQueryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Skip if LLM endpoint not configured
	llmEndpoint := os.Getenv("LLM_ENDPOINT")
	if llmEndpoint == "" {
		t.Skip("LLM_ENDPOINT not set, skipping integration test")
	}

	// Create temporary directory for test wiki
	tmpDir, err := os.MkdirTemp("", "wiki-query-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize wiki manager
	wikiManager, err := wiki.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create wiki manager: %v", err)
	}

	// Create some test pages
	setupTestWiki(t, wikiManager)

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

	// Initialize query engine
	queryEngine := query.NewEngine(wikiManager, llmClient, 10)

	// Test query
	request := types.QueryRequest{
		Query:        "What is machine learning?",
		MaxPages:     5,
		SaveAsPage:   false,
		OutputFormat: "markdown",
	}

	response, err := queryEngine.Query(request)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Verify response
	if response.Answer == "" {
		t.Error("expected non-empty answer")
	}

	if len(response.PagesRead) == 0 {
		t.Error("expected at least one page to be read")
	}

	if len(response.NavigationPath) == 0 {
		t.Error("expected non-empty navigation path")
	}

	t.Logf("Query successful!")
	t.Logf("Answer length: %d characters", len(response.Answer))
	t.Logf("Pages read: %d", len(response.PagesRead))
	t.Logf("Citations: %d", len(response.Citations))
	t.Logf("Navigation path: %v", response.NavigationPath)
}

// TestQueryWithSave tests query with page saving
func TestQueryWithSave(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Skip if LLM endpoint not configured
	llmEndpoint := os.Getenv("LLM_ENDPOINT")
	if llmEndpoint == "" {
		t.Skip("LLM_ENDPOINT not set, skipping integration test")
	}

	// Create temporary directory for test wiki
	tmpDir, err := os.MkdirTemp("", "wiki-query-save-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize wiki manager
	wikiManager, err := wiki.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create wiki manager: %v", err)
	}

	// Create some test pages
	setupTestWiki(t, wikiManager)

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

	// Initialize query engine
	queryEngine := query.NewEngine(wikiManager, llmClient, 10)

	// Test query with save
	request := types.QueryRequest{
		Query:        "What is deep learning?",
		MaxPages:     5,
		SaveAsPage:   true,
		OutputFormat: "markdown",
	}

	response, err := queryEngine.Query(request)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Verify response
	if response.SavedPagePath == "" {
		t.Error("expected saved page path")
	}

	// Verify page was created
	savedContent, err := wikiManager.ReadPage(response.SavedPagePath)
	if err != nil {
		t.Errorf("failed to read saved page: %v", err)
	}

	if savedContent == "" {
		t.Error("expected non-empty saved page content")
	}

	// Verify index was updated
	indexContent, err := wikiManager.ReadIndex()
	if err != nil {
		t.Errorf("failed to read index: %v", err)
	}

	if !strings.Contains(indexContent, "Queries") {
		t.Error("expected index to contain Queries section")
	}

	// Verify log was updated
	logContent, err := wikiManager.ReadLog()
	if err != nil {
		t.Errorf("failed to read log: %v", err)
	}

	if !strings.Contains(logContent, "query") {
		t.Error("expected log to contain query entry")
	}

	t.Logf("Query with save successful!")
	t.Logf("Saved page: %s", response.SavedPagePath)
}

// TestMultipleQueries tests multiple sequential queries
func TestMultipleQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Skip if LLM endpoint not configured
	llmEndpoint := os.Getenv("LLM_ENDPOINT")
	if llmEndpoint == "" {
		t.Skip("LLM_ENDPOINT not set, skipping integration test")
	}

	// Create temporary directory for test wiki
	tmpDir, err := os.MkdirTemp("", "wiki-multi-query-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize wiki manager
	wikiManager, err := wiki.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create wiki manager: %v", err)
	}

	// Create some test pages
	setupTestWiki(t, wikiManager)

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

	// Initialize query engine
	queryEngine := query.NewEngine(wikiManager, llmClient, 10)

	// Test multiple queries
	queries := []string{
		"What is machine learning?",
		"What is deep learning?",
		"How are they related?",
	}

	for i, q := range queries {
		request := types.QueryRequest{
			Query:        q,
			MaxPages:     5,
			SaveAsPage:   true,
			OutputFormat: "markdown",
		}

		response, err := queryEngine.Query(request)
		if err != nil {
			t.Errorf("query %d failed: %v", i+1, err)
			continue
		}

		if response.Answer == "" {
			t.Errorf("query %d: expected non-empty answer", i+1)
		}

		t.Logf("Query %d successful: %s", i+1, q)
	}

	// Verify wiki stats
	stats, err := wikiManager.GetStats()
	if err != nil {
		t.Errorf("failed to get stats: %v", err)
	}

	if stats.TotalQueries < 3 {
		t.Errorf("expected at least 3 queries, got %d", stats.TotalQueries)
	}

	t.Logf("Multiple queries successful!")
	t.Logf("Total queries: %d", stats.TotalQueries)
}

// setupTestWiki creates test pages in the wiki
func setupTestWiki(t *testing.T, wikiManager *wiki.Manager) {
	// Create concept pages
	concepts := map[string]string{
		"Machine Learning": `# Machine Learning

Machine learning is a subset of artificial intelligence (AI) that focuses on building systems that can learn from and make decisions based on data.

## Key Concepts
- Supervised learning
- Unsupervised learning
- Reinforcement learning

## Related Topics
- [Deep Learning](../concepts/deep-learning.md)
- [Neural Networks](../concepts/neural-networks.md)

## Applications
- Image recognition
- Natural language processing
- Recommendation systems`,

		"Deep Learning": `# Deep Learning

Deep learning is a subset of machine learning that uses neural networks with multiple layers (deep neural networks) to learn hierarchical representations of data.

## Architecture
- Input layer
- Hidden layers (multiple)
- Output layer

## Related Topics
- [Machine Learning](../concepts/machine-learning.md)
- [Neural Networks](../concepts/neural-networks.md)

## Applications
- Computer vision
- Speech recognition
- Language translation`,

		"Neural Networks": `# Neural Networks

Neural networks are computing systems inspired by biological neural networks. They consist of interconnected nodes (neurons) that process information.

## Components
- Neurons
- Weights
- Activation functions
- Layers

## Related Topics
- [Machine Learning](../concepts/machine-learning.md)
- [Deep Learning](../concepts/deep-learning.md)

## Types
- Feedforward networks
- Convolutional networks
- Recurrent networks`,
	}

	for name, content := range concepts {
		_, err := wikiManager.CreatePage("concepts", name, content)
		if err != nil {
			t.Fatalf("failed to create concept page %s: %v", name, err)
		}

		// Update index
		entry := types.IndexEntry{
			Title:    name,
			Path:     filepath.Join("concepts", strings.ToLower(strings.ReplaceAll(name, " ", "-"))+".md"),
			Summary:  "Concept page for " + name,
			Category: "concepts",
		}
		if err := wikiManager.UpdateIndex(entry); err != nil {
			t.Fatalf("failed to update index for %s: %v", name, err)
		}
	}

	t.Logf("Test wiki setup complete with %d concept pages", len(concepts))
}
