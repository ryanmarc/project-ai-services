package wiki

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create manager
	manager, err := NewManager(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, manager)

	// Verify directory structure
	wikiDir := filepath.Join(tmpDir, "wiki")
	assert.DirExists(t, wikiDir)
	assert.DirExists(t, filepath.Join(wikiDir, "sources"))
	assert.DirExists(t, filepath.Join(wikiDir, "entities"))
	assert.DirExists(t, filepath.Join(wikiDir, "concepts"))
	assert.DirExists(t, filepath.Join(wikiDir, "queries"))

	// Verify index.md exists
	indexPath := filepath.Join(wikiDir, "index.md")
	assert.FileExists(t, indexPath)

	// Verify log.md exists
	logPath := filepath.Join(wikiDir, "log.md")
	assert.FileExists(t, logPath)
}

func TestCreatePage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager, err := NewManager(tmpDir)
	require.NoError(t, err)

	tests := []struct {
		name     string
		category string
		title    string
		content  string
		wantErr  bool
	}{
		{
			name:     "create source page",
			category: "sources",
			title:    "Test Document",
			content:  "# Test Document\n\nThis is a test.",
			wantErr:  false,
		},
		{
			name:     "create entity page",
			category: "entities",
			title:    "John Doe",
			content:  "# John Doe\n\nA test person.",
			wantErr:  false,
		},
		{
			name:     "create concept page",
			category: "concepts",
			title:    "Machine Learning",
			content:  "# Machine Learning\n\nA test concept.",
			wantErr:  false,
		},
		{
			name:     "invalid category",
			category: "invalid",
			title:    "Test",
			content:  "Test",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := manager.CreatePage(tt.category, tt.title, tt.content)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, path)

			// Verify page was created
			fullPath := filepath.Join(manager.wikiDir, path)
			assert.FileExists(t, fullPath)

			// Verify content
			content, err := os.ReadFile(fullPath)
			require.NoError(t, err)
			assert.Equal(t, tt.content, string(content))
		})
	}
}

func TestReadPage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager, err := NewManager(tmpDir)
	require.NoError(t, err)

	// Create a test page
	expectedContent := "# Test Page\n\nThis is test content."
	path, err := manager.CreatePage("sources", "Test Page", expectedContent)
	require.NoError(t, err)

	// Read the page
	content, err := manager.ReadPage(path)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, content)

	// Try to read non-existent page
	_, err = manager.ReadPage("non-existent.md")
	assert.Error(t, err)
}

func TestUpdatePage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager, err := NewManager(tmpDir)
	require.NoError(t, err)

	// Create a test page
	path, err := manager.CreatePage("sources", "Test Page", "Original content")
	require.NoError(t, err)

	// Update the page
	newContent := "Updated content"
	err = manager.UpdatePage(path, newContent)
	require.NoError(t, err)

	// Verify update
	content, err := manager.ReadPage(path)
	require.NoError(t, err)
	assert.Equal(t, newContent, content)

	// Try to update non-existent page
	err = manager.UpdatePage("non-existent.md", "content")
	assert.Error(t, err)
}

func TestDeletePage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager, err := NewManager(tmpDir)
	require.NoError(t, err)

	// Create a test page
	path, err := manager.CreatePage("sources", "Test Page", "Content")
	require.NoError(t, err)

	// Verify page exists
	fullPath := filepath.Join(manager.wikiDir, path)
	assert.FileExists(t, fullPath)

	// Delete the page
	err = manager.DeletePage(path)
	require.NoError(t, err)

	// Verify page is deleted
	assert.NoFileExists(t, fullPath)
}

func TestUpdateIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager, err := NewManager(tmpDir)
	require.NoError(t, err)

	// Add an entry
	entry := types.IndexEntry{
		Path:     "sources/test-doc.md",
		Title:    "Test Document",
		Category: "sources",
		Summary:  "A test document",
	}

	err = manager.UpdateIndex(entry)
	require.NoError(t, err)

	// Read index and verify
	index, err := manager.ReadIndex()
	require.NoError(t, err)
	assert.Contains(t, index, "Test Document")
	assert.Contains(t, index, "A test document")
	assert.Contains(t, index, "Total sources: 1")
}

func TestAppendLog(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager, err := NewManager(tmpDir)
	require.NoError(t, err)

	// Append a log entry
	entry := types.LogEntry{
		Timestamp:   time.Now(),
		Type:        "ingest",
		Title:       "Test Document",
		Description: "Ingested test document",
		Details:     "- Created 3 pages\n- Found 2 entities",
	}

	err = manager.AppendLog(entry)
	require.NoError(t, err)

	// Read log and verify
	log, err := manager.ReadLog()
	require.NoError(t, err)
	assert.Contains(t, log, "ingest")
	assert.Contains(t, log, "Test Document")
	assert.Contains(t, log, "Created 3 pages")
}

func TestGetRecentLogs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager, err := NewManager(tmpDir)
	require.NoError(t, err)

	// Add multiple log entries
	for i := 0; i < 5; i++ {
		entry := types.LogEntry{
			Timestamp:   time.Now().Add(time.Duration(i) * time.Minute),
			Type:        "test",
			Title:       "Test Entry",
			Description: "Test",
			Details:     "Details",
		}
		err = manager.AppendLog(entry)
		require.NoError(t, err)
	}

	// Get recent logs
	logs, err := manager.GetRecentLogs(3)
	require.NoError(t, err)
	assert.Len(t, logs, 3)
}

func TestExtractLinks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager, err := NewManager(tmpDir)
	require.NoError(t, err)

	content := `# Test Page

This page links to [Entity A](entities/entity-a.md) and [Concept B](concepts/concept-b.md).

Also see [External Link](https://example.com).
`

	links := manager.ExtractLinks(content)
	assert.Len(t, links, 3)
	assert.Contains(t, links, "entities/entity-a.md")
	assert.Contains(t, links, "concepts/concept-b.md")
	assert.Contains(t, links, "https://example.com")
}

func TestSanitizeFilename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager, err := NewManager(tmpDir)
	require.NoError(t, err)

	tests := []struct {
		input    string
		expected string
	}{
		{"Simple Title", "simple-title"},
		{"Title With Numbers 123", "title-with-numbers-123"},
		{"Title!@#$%^&*()", "title"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"Title-With-Hyphens", "title-with-hyphens"},
		{"UPPERCASE", "uppercase"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := manager.sanitizeFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestListPages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager, err := NewManager(tmpDir)
	require.NoError(t, err)

	// Create test pages
	_, err = manager.CreatePage("sources", "Source 1", "Content 1")
	require.NoError(t, err)
	_, err = manager.CreatePage("sources", "Source 2", "Content 2")
	require.NoError(t, err)
	_, err = manager.CreatePage("entities", "Entity 1", "Content 3")
	require.NoError(t, err)

	// List sources
	sources, err := manager.ListPages("sources")
	require.NoError(t, err)
	assert.Len(t, sources, 2)

	// List entities
	entities, err := manager.ListPages("entities")
	require.NoError(t, err)
	assert.Len(t, entities, 1)

	// List all
	all, err := manager.ListPages("all")
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestGetStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager, err := NewManager(tmpDir)
	require.NoError(t, err)

	// Initial stats
	stats, err := manager.GetStats()
	require.NoError(t, err)
	assert.Equal(t, 0, stats.TotalSources)
	assert.Equal(t, 0, stats.TotalPages)

	// Add some entries
	err = manager.UpdateIndex(types.IndexEntry{
		Path:     "sources/test.md",
		Title:    "Test",
		Category: "sources",
		Summary:  "Test",
	})
	require.NoError(t, err)

	// Check updated stats
	stats, err = manager.GetStats()
	require.NoError(t, err)
	assert.Equal(t, 1, stats.TotalSources)
	assert.Equal(t, 1, stats.TotalPages)
}
