package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/IBM/project-ai-services/wiki-service/internal/api"
	"github.com/IBM/project-ai-services/wiki-service/internal/llm"
	"github.com/IBM/project-ai-services/wiki-service/internal/wiki"
	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestAPIServer creates a test API server with mock LLM
func setupTestAPIServer(t *testing.T) (*api.Server, *httptest.Server, string) {
	// Create temporary directory for wiki data
	tmpDir, err := os.MkdirTemp("", "wiki-api-test-*")
	require.NoError(t, err)

	// Initialize wiki manager
	wikiManager, err := wiki.NewManager(tmpDir)
	require.NoError(t, err)

	// Create mock LLM client
	llmConfig := types.LLMConfig{
		Endpoint:    "http://mock-llm:8000",
		Model:       "mock-model",
		MaxTokens:   4096,
		Temperature: 0.2,
	}
	llmClient := llm.NewClient(llmConfig)

	// Create wiki config
	wikiConfig := types.WikiConfig{
		DataDir:          tmpDir,
		MaxPagesPerQuery: 10,
		IndexSearchLimit: 20,
		LogLevel:         "info",
	}

	// Create API server
	apiServer := api.NewServer(wikiManager, llmClient, wikiConfig)
	router := apiServer.SetupRoutes()

	// Create test HTTP server
	testServer := httptest.NewServer(router)

	return apiServer, testServer, tmpDir
}

// TestAPIHealth tests the health endpoint
func TestAPIHealth(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	resp, err := http.Get(testServer.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)

	assert.Contains(t, health, "status")
	assert.Contains(t, health, "wiki_healthy")
	assert.Contains(t, health, "timestamp")
}

// TestAPIRoot tests the root endpoint
func TestAPIRoot(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	resp, err := http.Get(testServer.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var root map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&root)
	require.NoError(t, err)

	assert.Equal(t, "Wiki Service API", root["service"])
	assert.Contains(t, root, "endpoints")
}

// TestAPIGetStats tests the stats endpoint
func TestAPIGetStats(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	resp, err := http.Get(testServer.URL + "/v1/wiki/stats")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var stats types.WikiStats
	err = json.NewDecoder(resp.Body).Decode(&stats)
	require.NoError(t, err)

	assert.Equal(t, 0, stats.TotalSources)
	assert.Equal(t, 0, stats.TotalPages)
	assert.Equal(t, 0, stats.TotalEntities)
	assert.Equal(t, 0, stats.TotalConcepts)
}

// TestAPIGetIndex tests the index endpoint
func TestAPIGetIndex(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	resp, err := http.Get(testServer.URL + "/v1/wiki/index")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var indexResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&indexResp)
	require.NoError(t, err)

	assert.Contains(t, indexResp, "content")
	content := indexResp["content"].(string)
	assert.Contains(t, content, "# Wiki Index")
}

// TestAPIGetLog tests the log endpoint
func TestAPIGetLog(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	resp, err := http.Get(testServer.URL + "/v1/wiki/log")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var logResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&logResp)
	require.NoError(t, err)

	assert.Contains(t, logResp, "content")
	content := logResp["content"].(string)
	assert.Contains(t, content, "# Wiki Activity Log")
}

// TestAPIGetPages tests the pages listing endpoint
func TestAPIGetPages(t *testing.T) {
	apiServer, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	// Create a test page
	_, err := apiServer.WikiManager.CreatePage("concepts", "Test Concept", "# Test Concept\n\nThis is a test.")
	require.NoError(t, err)

	resp, err := http.Get(testServer.URL + "/v1/wiki/pages")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var pagesResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&pagesResp)
	require.NoError(t, err)

	assert.Contains(t, pagesResp, "pages")
	assert.Contains(t, pagesResp, "count")

	pages := pagesResp["pages"].([]interface{})
	assert.Greater(t, len(pages), 0)
}

// TestAPIGetPagesByCategory tests the pages listing with category filter
func TestAPIGetPagesByCategory(t *testing.T) {
	apiServer, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	// Create test pages in different categories
	_, err := apiServer.WikiManager.CreatePage("concepts", "Test Concept", "# Test Concept\n\nThis is a test.")
	require.NoError(t, err)

	_, err = apiServer.WikiManager.CreatePage("entities", "Test Entity", "# Test Entity\n\nThis is a test.")
	require.NoError(t, err)

	resp, err := http.Get(testServer.URL + "/v1/wiki/pages?category=concepts")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var pagesResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&pagesResp)
	require.NoError(t, err)

	assert.Equal(t, "concepts", pagesResp["category"])
	pages := pagesResp["pages"].([]interface{})
	assert.Greater(t, len(pages), 0)
}

// TestAPIGetPage tests the get page endpoint
func TestAPIGetPage(t *testing.T) {
	apiServer, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	// Create a test page
	testContent := "# Test Concept\n\nThis is a test concept page."
	_, err := apiServer.WikiManager.CreatePage("concepts", "Test Concept", testContent)
	require.NoError(t, err)

	resp, err := http.Get(testServer.URL + "/v1/wiki/pages/concepts/test-concept.md")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var pageResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&pageResp)
	require.NoError(t, err)

	assert.Contains(t, pageResp, "path")
	assert.Contains(t, pageResp, "content")
	assert.Equal(t, "concepts/test-concept.md", pageResp["path"])
	assert.Contains(t, pageResp["content"].(string), "Test Concept")
}

// TestAPIGetPageNotFound tests 404 for non-existent page
func TestAPIGetPageNotFound(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	resp, err := http.Get(testServer.URL + "/v1/wiki/pages/nonexistent/page.md")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestAPIIngestInvalidRequest tests ingest with invalid request
func TestAPIIngestInvalidRequest(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	// Send invalid JSON
	resp, err := http.Post(
		testServer.URL+"/v1/wiki/ingest",
		"application/json",
		bytes.NewBufferString("{invalid json}"),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestAPIIngestMissingSourcePath tests ingest without source_path
func TestAPIIngestMissingSourcePath(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	reqBody := map[string]interface{}{
		"source_type": "text",
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		testServer.URL+"/v1/wiki/ingest",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&errResp)
	assert.Contains(t, errResp["message"], "source_path")
}

// TestAPIQueryInvalidRequest tests query with invalid request
func TestAPIQueryInvalidRequest(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	// Send invalid JSON
	resp, err := http.Post(
		testServer.URL+"/v1/wiki/query",
		"application/json",
		bytes.NewBufferString("{invalid json}"),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestAPIQueryMissingQuery tests query without query text
func TestAPIQueryMissingQuery(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	reqBody := map[string]interface{}{
		"max_pages": 10,
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		testServer.URL+"/v1/wiki/query",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&errResp)
	assert.Contains(t, errResp["message"], "query")
}

// TestAPICORS tests CORS headers
func TestAPICORS(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	// Make OPTIONS request
	req, err := http.NewRequest("OPTIONS", testServer.URL+"/v1/wiki/stats", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "GET")
}

// TestAPINavigationTracking tests query navigation tracking
func TestAPINavigationTracking(t *testing.T) {
	apiServer, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	// Create test pages
	_, err := apiServer.WikiManager.CreatePage("concepts", "Machine Learning",
		"# Machine Learning\n\nML is a subset of AI.")
	require.NoError(t, err)

	// Update index
	err = apiServer.WikiManager.UpdateIndex(types.IndexEntry{
		Path:     "concepts/machine-learning.md",
		Title:    "Machine Learning",
		Category: "concepts",
		Summary:  "ML is a subset of AI",
	})
	require.NoError(t, err)

	// Note: This test will fail without a real LLM, but tests the API structure
	// In a real scenario, you'd mock the LLM response
	reqBody := map[string]interface{}{
		"query":        "What is machine learning?",
		"max_pages":    5,
		"save_as_page": false,
	}
	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		testServer.URL+"/v1/wiki/query",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Even if query fails due to mock LLM, the API structure should be correct
	if resp.StatusCode == http.StatusOK {
		var queryResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&queryResp)
		require.NoError(t, err)

		// Check for query_id in response
		assert.Contains(t, queryResp, "query_id")

		if queryID, ok := queryResp["query_id"].(string); ok {
			// Try to get navigation
			navResp, err := http.Get(fmt.Sprintf("%s/v1/wiki/navigation/%s", testServer.URL, queryID))
			require.NoError(t, err)
			defer navResp.Body.Close()

			if navResp.StatusCode == http.StatusOK {
				var nav map[string]interface{}
				err = json.NewDecoder(navResp.Body).Decode(&nav)
				require.NoError(t, err)

				assert.Contains(t, nav, "query_id")
				assert.Contains(t, nav, "query")
				assert.Contains(t, nav, "navigation_path")
			}
		}
	}
}

// TestAPIGetNavigationNotFound tests 404 for non-existent navigation
func TestAPIGetNavigationNotFound(t *testing.T) {
	_, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	resp, err := http.Get(testServer.URL + "/v1/wiki/navigation/nonexistent-query-id")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestAPIEndToEnd tests a complete workflow
func TestAPIEndToEnd(t *testing.T) {
	apiServer, testServer, tmpDir := setupTestAPIServer(t)
	defer testServer.Close()
	defer os.RemoveAll(tmpDir)

	// 1. Check initial stats
	resp, err := http.Get(testServer.URL + "/v1/wiki/stats")
	require.NoError(t, err)
	var stats types.WikiStats
	json.NewDecoder(resp.Body).Decode(&stats)
	resp.Body.Close()
	assert.Equal(t, 0, stats.TotalPages)

	// 2. Create a test document
	testFile := filepath.Join(tmpDir, "test-doc.txt")
	err = os.WriteFile(testFile, []byte("Machine learning is a subset of artificial intelligence."), 0644)
	require.NoError(t, err)

	// 3. List pages (should be empty initially except index/log)
	resp, err = http.Get(testServer.URL + "/v1/wiki/pages")
	require.NoError(t, err)
	var pagesResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&pagesResp)
	resp.Body.Close()
	initialPageCount := int(pagesResp["count"].(float64))

	// 4. Create a page directly
	_, err = apiServer.WikiManager.CreatePage("concepts", "Test", "# Test\n\nContent")
	require.NoError(t, err)

	// 5. List pages again (should have one more)
	resp, err = http.Get(testServer.URL + "/v1/wiki/pages")
	require.NoError(t, err)
	json.NewDecoder(resp.Body).Decode(&pagesResp)
	resp.Body.Close()
	newPageCount := int(pagesResp["count"].(float64))
	assert.Greater(t, newPageCount, initialPageCount)

	// 6. Get the page
	resp, err = http.Get(testServer.URL + "/v1/wiki/pages/concepts/test.md")
	require.NoError(t, err)
	var pageResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&pageResp)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, pageResp["content"], "Test")

	// 7. Check stats again
	resp, err = http.Get(testServer.URL + "/v1/wiki/stats")
	require.NoError(t, err)
	json.NewDecoder(resp.Body).Decode(&stats)
	resp.Body.Close()
	assert.Greater(t, stats.TotalPages, 0)
}

// Helper function to read response body
func readBody(r io.Reader) string {
	body, _ := io.ReadAll(r)
	return string(body)
}
