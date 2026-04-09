package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DoclingClient wraps document processing functionality
// For POC: Simple text extraction
// For Production: Full Docling Python integration via subprocess or HTTP API
type DoclingClient struct {
	pythonPath string
	enabled    bool
}

// NewDoclingClient creates a new Docling client
func NewDoclingClient(pythonPath string) *DoclingClient {
	return &DoclingClient{
		pythonPath: pythonPath,
		enabled:    false, // Disabled for POC - using simple text extraction
	}
}

// ProcessDocument processes a document and extracts content
// For POC: Delegates to simple text extraction
// For Production: Would call Python Docling via subprocess or HTTP
func (d *DoclingClient) ProcessDocument(sourcePath string) (*ExtractedContent, error) {
	if !d.enabled {
		// POC: Use simple text extraction
		return d.simpleExtract(sourcePath)
	}

	// Future: Call Python Docling
	// This would involve:
	// 1. Spawning Python subprocess with docling
	// 2. Passing file path
	// 3. Receiving JSON output with text, tables, images
	// 4. Parsing and returning ExtractedContent

	return nil, fmt.Errorf("full Docling integration not implemented in POC")
}

// simpleExtract performs basic text extraction for POC
func (d *DoclingClient) simpleExtract(sourcePath string) (*ExtractedContent, error) {
	ext := strings.ToLower(filepath.Ext(sourcePath))

	// Support basic text formats for POC
	if ext != ".txt" && ext != ".md" {
		return nil, fmt.Errorf("unsupported format for POC: %s (only .txt and .md supported)", ext)
	}

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &ExtractedContent{
		Text:   string(content),
		Tables: []Table{},
		Images: []Image{},
	}, nil
}

// IsEnabled returns whether full Docling is enabled
func (d *DoclingClient) IsEnabled() bool {
	return d.enabled
}

// GetSupportedFormats returns supported file formats
func (d *DoclingClient) GetSupportedFormats() []string {
	if d.enabled {
		// Full Docling supports many formats
		return []string{".pdf", ".docx", ".pptx", ".html", ".txt", ".md"}
	}
	// POC only supports text
	return []string{".txt", ".md"}
}
