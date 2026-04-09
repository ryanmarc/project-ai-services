package integration

import (
	"os"
	"testing"

	"github.com/IBM/project-ai-services/wiki-service/internal/wiki"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateExistingPageLogic tests the page update logic
// This is a simpler unit test that doesn't require a full LLM mock
func TestUpdateExistingPageLogic(t *testing.T) {
	// Create temporary directory for test wiki
	tempDir, err := os.MkdirTemp("", "wiki-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize wiki manager
	wikiManager, err := wiki.NewManager(tempDir)
	require.NoError(t, err)

	// Test 1: Create initial page
	t.Run("Create Initial Page", func(t *testing.T) {
		_, err := wikiManager.CreatePage("entities", "machine-learning", `# Machine Learning

**Type:** Technology

## Overview
Machine Learning is a subset of artificial intelligence.

## Key Characteristics
- Learns from data
- Makes predictions

## References
- Introduction to Machine Learning basics`)
		require.NoError(t, err)

		// Verify page was created
		content, err := wikiManager.ReadPage("entities/machine-learning.md")
		require.NoError(t, err)
		assert.Contains(t, content, "Machine Learning")
		assert.Contains(t, content, "subset of artificial intelligence")
	})

	// Test 2: Update existing page
	t.Run("Update Existing Page", func(t *testing.T) {
		updatedContent := `# Machine Learning

**Type:** Technology

## Overview
Machine Learning is a subset of artificial intelligence that includes both traditional and deep learning approaches.

## Key Characteristics
- Learns from data
- Makes predictions
- Includes deep learning techniques
- Uses neural networks

## Advanced Techniques
- Deep Learning
- Neural Networks
- Transfer Learning

## References
- Introduction to Machine Learning basics
- Advanced Machine Learning techniques`

		err := wikiManager.UpdatePage("entities/machine-learning.md", updatedContent)
		require.NoError(t, err)

		// Verify page was updated
		content, err := wikiManager.ReadPage("entities/machine-learning.md")
		require.NoError(t, err)
		assert.Contains(t, content, "deep learning", "Updated page should mention deep learning")
		assert.Contains(t, content, "Advanced Techniques", "Updated page should have new section")
		assert.Contains(t, content, "subset of artificial intelligence", "Should retain original content")
	})

	// Test 3: Verify page exists after update
	t.Run("Page Exists After Update", func(t *testing.T) {
		// List pages to verify it exists
		pages, err := wikiManager.ListPages("entities")
		require.NoError(t, err)

		assert.Len(t, pages, 1, "Should have 1 entity page")
		assert.Contains(t, pages[0], "machine-learning.md", "Should be the ML page")
	})
}
