package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase",
			input:    "hello world",
			expected: "hello-world",
		},
		{
			name:     "mixed case",
			input:    "Hello World",
			expected: "hello-world",
		},
		{
			name:     "special characters",
			input:    "Hello, World!",
			expected: "hello-world",
		},
		{
			name:     "multiple spaces",
			input:    "hello   world",
			expected: "hello-world",
		},
		{
			name:     "leading/trailing spaces",
			input:    "  hello world  ",
			expected: "hello-world",
		},
		{
			name:     "numbers",
			input:    "test 123",
			expected: "test-123",
		},
		{
			name:     "already sanitized",
			input:    "hello-world",
			expected: "hello-world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateSummary(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLen   int
		expected string
	}{
		{
			name:     "short text",
			text:     "Hello",
			maxLen:   10,
			expected: "Hello",
		},
		{
			name:     "exact length",
			text:     "Hello",
			maxLen:   5,
			expected: "Hello",
		},
		{
			name:     "needs truncation",
			text:     "Hello World",
			maxLen:   5,
			expected: "Hello...",
		},
		{
			name:     "empty text",
			text:     "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateSummary(tt.text, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSourceTitle(t *testing.T) {
	engine := &Engine{}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple filename",
			path:     "/path/to/document.txt",
			expected: "document",
		},
		{
			name:     "filename with multiple dots",
			path:     "/path/to/my.document.txt",
			expected: "my.document",
		},
		{
			name:     "no extension",
			path:     "/path/to/document",
			expected: "document",
		},
		{
			name:     "relative path",
			path:     "document.txt",
			expected: "document",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.getSourceTitle(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatEntityList(t *testing.T) {
	tests := []struct {
		name     string
		entities []Entity
		expected string
	}{
		{
			name:     "empty list",
			entities: []Entity{},
			expected: "None",
		},
		{
			name: "single entity",
			entities: []Entity{
				{Name: "John Doe", Type: "person", Description: "A researcher"},
			},
			expected: "- John Doe (person): A researcher",
		},
		{
			name: "multiple entities",
			entities: []Entity{
				{Name: "John Doe", Type: "person", Description: "A researcher"},
				{Name: "Acme Corp", Type: "organization", Description: "A company"},
			},
			expected: "- John Doe (person): A researcher\n- Acme Corp (organization): A company",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatEntityList(tt.entities)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatConceptList(t *testing.T) {
	tests := []struct {
		name     string
		concepts []Concept
		expected string
	}{
		{
			name:     "empty list",
			concepts: []Concept{},
			expected: "None",
		},
		{
			name: "single concept",
			concepts: []Concept{
				{Name: "Machine Learning", Description: "AI technique"},
			},
			expected: "- Machine Learning: AI technique",
		},
		{
			name: "multiple concepts",
			concepts: []Concept{
				{Name: "Machine Learning", Description: "AI technique"},
				{Name: "Neural Networks", Description: "Computing systems"},
			},
			expected: "- Machine Learning: AI technique\n- Neural Networks: Computing systems",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatConceptList(tt.concepts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDoclingClient(t *testing.T) {
	client := NewDoclingClient("")

	t.Run("IsEnabled returns false for POC", func(t *testing.T) {
		assert.False(t, client.IsEnabled())
	})

	t.Run("GetSupportedFormats returns text formats for POC", func(t *testing.T) {
		formats := client.GetSupportedFormats()
		assert.Contains(t, formats, ".txt")
		assert.Contains(t, formats, ".md")
	})

	t.Run("ProcessDocument with unsupported format", func(t *testing.T) {
		_, err := client.ProcessDocument("test.pdf")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported format")
	})
}

func TestExtractContent(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "This is a test document."

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	client := NewDoclingClient("")

	t.Run("Extract from text file", func(t *testing.T) {
		content, err := client.ProcessDocument(testFile)
		require.NoError(t, err)
		assert.Equal(t, testContent, content.Text)
		assert.Empty(t, content.Tables)
		assert.Empty(t, content.Images)
	})

	t.Run("Extract from non-existent file", func(t *testing.T) {
		_, err := client.ProcessDocument(filepath.Join(tmpDir, "nonexistent.txt"))
		assert.Error(t, err)
	})
}

func TestIngestAnalysisPrompt(t *testing.T) {
	docContent := "Test document content"
	indexContent := "Test index content"

	prompt := IngestAnalysisPrompt(docContent, indexContent)

	assert.Contains(t, prompt, docContent)
	assert.Contains(t, prompt, indexContent)
	assert.Contains(t, prompt, "JSON format")
	assert.Contains(t, prompt, "entities")
	assert.Contains(t, prompt, "concepts")
	assert.Contains(t, prompt, "summary")
}

func TestEntityPageContentPrompt(t *testing.T) {
	prompt := EntityPageContentPrompt("John Doe", "person", "A researcher", "Related info")

	assert.Contains(t, prompt, "John Doe")
	assert.Contains(t, prompt, "person")
	assert.Contains(t, prompt, "A researcher")
	assert.Contains(t, prompt, "Related info")
	assert.Contains(t, prompt, "markdown")
}

func TestConceptPageContentPrompt(t *testing.T) {
	prompt := ConceptPageContentPrompt("Machine Learning", "AI technique", "Related info")

	assert.Contains(t, prompt, "Machine Learning")
	assert.Contains(t, prompt, "AI technique")
	assert.Contains(t, prompt, "Related info")
	assert.Contains(t, prompt, "markdown")
}

func TestSourceSummaryPrompt(t *testing.T) {
	prompt := SourceSummaryPrompt("Test Doc", "Summary", "Entities", "Concepts")

	assert.Contains(t, prompt, "Test Doc")
	assert.Contains(t, prompt, "Summary")
	assert.Contains(t, prompt, "Entities")
	assert.Contains(t, prompt, "Concepts")
	assert.Contains(t, prompt, "markdown")
}

func TestUpdateExistingPagePrompt(t *testing.T) {
	prompt := UpdateExistingPagePrompt("path/to/page.md", "Current content", "New info")

	assert.Contains(t, prompt, "path/to/page.md")
	assert.Contains(t, prompt, "Current content")
	assert.Contains(t, prompt, "New info")
	assert.Contains(t, prompt, "update")
}
