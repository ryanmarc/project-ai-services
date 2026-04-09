package ingest

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/IBM/project-ai-services/wiki-service/internal/llm"
	"github.com/IBM/project-ai-services/wiki-service/internal/wiki"
	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
)

// Engine handles document ingestion into the wiki
type Engine struct {
	wikiManager   *wiki.Manager
	llmClient     *llm.Client
	doclingClient *DoclingClient
}

// NewEngine creates a new ingest engine
func NewEngine(wikiManager *wiki.Manager, llmClient *llm.Client) *Engine {
	return &Engine{
		wikiManager:   wikiManager,
		llmClient:     llmClient,
		doclingClient: NewDoclingClient(""), // POC: Simple text extraction
	}
}

// Ingest processes a source document and creates wiki pages
func (e *Engine) Ingest(request types.IngestRequest) (*types.IngestResponse, error) {
	// Step 1: Extract document content
	content, err := e.extractContent(request.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract content: %w", err)
	}

	// Step 2: Get current wiki index for context
	indexContent, err := e.wikiManager.ReadIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	// Step 3: Analyze document with LLM
	analysis, err := e.analyzeDocument(content.Text, indexContent)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze document: %w", err)
	}

	// Step 4: Create wiki pages
	response := &types.IngestResponse{
		PagesCreated:  []string{},
		PagesUpdated:  []string{},
		EntitiesFound: []string{},
		ConceptsFound: []string{},
		Summary:       analysis.Summary,
	}

	// Create source summary page
	sourceTitle := e.getSourceTitle(request.SourcePath)
	sourcePage, err := e.createSourceSummaryPage(sourceTitle, analysis)
	if err != nil {
		return nil, fmt.Errorf("failed to create source summary: %w", err)
	}
	response.PagesCreated = append(response.PagesCreated, sourcePage)

	// Create entity pages
	for _, entity := range analysis.Entities {
		pagePath, err := e.createEntityPage(entity, analysis.Summary)
		if err != nil {
			// Log error but continue (page might already exist)
			fmt.Printf("Warning: failed to create entity page for %s: %v\n", entity.Name, err)
			continue
		}
		response.PagesCreated = append(response.PagesCreated, pagePath)
		response.EntitiesFound = append(response.EntitiesFound, entity.Name)
	}

	// Create concept pages
	for _, concept := range analysis.Concepts {
		pagePath, err := e.createConceptPage(concept, analysis.Summary)
		if err != nil {
			// Log error but continue (page might already exist)
			fmt.Printf("Warning: failed to create concept page for %s: %v\n", concept.Name, err)
			continue
		}
		response.PagesCreated = append(response.PagesCreated, pagePath)
		response.ConceptsFound = append(response.ConceptsFound, concept.Name)
	}

	// Step 5: Update index
	if err := e.updateIndexForIngest(sourceTitle, sourcePage, analysis); err != nil {
		return nil, fmt.Errorf("failed to update index: %w", err)
	}

	// Step 6: Append to log
	logEntry := e.createLogEntry(sourceTitle, response)
	if err := e.wikiManager.AppendLog(logEntry); err != nil {
		return nil, fmt.Errorf("failed to append log: %w", err)
	}

	response.LogEntry = fmt.Sprintf("[%s] ingest | %s", logEntry.Timestamp.Format(time.RFC3339), sourceTitle)

	return response, nil
}

// extractContent extracts content from a source document
func (e *Engine) extractContent(sourcePath string) (*ExtractedContent, error) {
	return e.doclingClient.ProcessDocument(sourcePath)
}

// analyzeDocument analyzes document content with LLM
func (e *Engine) analyzeDocument(documentContent, indexContent string) (*IngestAnalysisResult, error) {
	// Truncate content if too long (simple approach for POC)
	maxContentLength := 8000
	if len(documentContent) > maxContentLength {
		documentContent = documentContent[:maxContentLength] + "\n\n[Content truncated for analysis]"
	}

	// Generate prompt
	prompt := IngestAnalysisPrompt(documentContent, indexContent)

	// Call LLM
	systemPrompt := "You are a knowledge management assistant helping to maintain a personal wiki. Always respond with valid JSON."

	var result IngestAnalysisResult
	if err := e.llmClient.CompleteJSON(systemPrompt, prompt, &result); err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	return &result, nil
}

// createSourceSummaryPage creates a summary page for the source document
func (e *Engine) createSourceSummaryPage(sourceTitle string, analysis *IngestAnalysisResult) (string, error) {
	// Format entities and concepts
	entitiesList := FormatEntityList(analysis.Entities)
	conceptsList := FormatConceptList(analysis.Concepts)

	// Generate page content with LLM
	prompt := SourceSummaryPrompt(sourceTitle, analysis.Summary, entitiesList, conceptsList)
	systemPrompt := "You are a knowledge management assistant. Generate well-structured markdown content."

	content, err := e.llmClient.Complete(systemPrompt, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate source summary: %w", err)
	}

	// Create page
	pagePath, err := e.wikiManager.CreatePage("sources", sourceTitle, content)
	if err != nil {
		return "", fmt.Errorf("failed to create source page: %w", err)
	}

	return pagePath, nil
}

// createEntityPage creates a page for an entity
func (e *Engine) createEntityPage(entity Entity, relatedInfo string) (string, error) {
	// Generate page content with LLM
	prompt := EntityPageContentPrompt(entity.Name, entity.Type, entity.Description, relatedInfo)
	systemPrompt := "You are a knowledge management assistant. Generate well-structured markdown content."

	content, err := e.llmClient.Complete(systemPrompt, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate entity page: %w", err)
	}

	// Create page
	pagePath, err := e.wikiManager.CreatePage("entities", entity.Name, content)
	if err != nil {
		// Check if page already exists - if so, we might want to update it
		if strings.Contains(err.Error(), "already exists") {
			return "", fmt.Errorf("entity page already exists: %s", entity.Name)
		}
		return "", fmt.Errorf("failed to create entity page: %w", err)
	}

	return pagePath, nil
}

// createConceptPage creates a page for a concept
func (e *Engine) createConceptPage(concept Concept, relatedInfo string) (string, error) {
	// Generate page content with LLM
	prompt := ConceptPageContentPrompt(concept.Name, concept.Description, relatedInfo)
	systemPrompt := "You are a knowledge management assistant. Generate well-structured markdown content."

	content, err := e.llmClient.Complete(systemPrompt, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate concept page: %w", err)
	}

	// Create page
	pagePath, err := e.wikiManager.CreatePage("concepts", concept.Name, content)
	if err != nil {
		// Check if page already exists
		if strings.Contains(err.Error(), "already exists") {
			return "", fmt.Errorf("concept page already exists: %s", concept.Name)
		}
		return "", fmt.Errorf("failed to create concept page: %w", err)
	}

	return pagePath, nil
}

// updateIndexForIngest updates the wiki index after ingestion
func (e *Engine) updateIndexForIngest(sourceTitle, sourcePath string, analysis *IngestAnalysisResult) error {
	// Add source to index
	sourceEntry := types.IndexEntry{
		Title:      sourceTitle,
		Path:       sourcePath,
		Summary:    truncateSummary(analysis.Summary, 100),
		Category:   "sources",
		References: len(analysis.Entities) + len(analysis.Concepts),
	}

	if err := e.wikiManager.UpdateIndex(sourceEntry); err != nil {
		return fmt.Errorf("failed to update index with source: %w", err)
	}

	// Add entities to index
	for _, entity := range analysis.Entities {
		entityEntry := types.IndexEntry{
			Title:      entity.Name,
			Path:       fmt.Sprintf("entities/%s.md", sanitizeFilename(entity.Name)),
			Summary:    truncateSummary(entity.Description, 100),
			Category:   "entities",
			References: 1,
		}

		if err := e.wikiManager.UpdateIndex(entityEntry); err != nil {
			// Log but don't fail if entity already in index
			fmt.Printf("Warning: failed to add entity to index: %v\n", err)
		}
	}

	// Add concepts to index
	for _, concept := range analysis.Concepts {
		conceptEntry := types.IndexEntry{
			Title:      concept.Name,
			Path:       fmt.Sprintf("concepts/%s.md", sanitizeFilename(concept.Name)),
			Summary:    truncateSummary(concept.Description, 100),
			Category:   "concepts",
			References: 1,
		}

		if err := e.wikiManager.UpdateIndex(conceptEntry); err != nil {
			// Log but don't fail if concept already in index
			fmt.Printf("Warning: failed to add concept to index: %v\n", err)
		}
	}

	return nil
}

// createLogEntry creates a log entry for the ingestion
func (e *Engine) createLogEntry(sourceTitle string, response *types.IngestResponse) types.LogEntry {
	details := fmt.Sprintf(`- Created: %s
- Entities found: %d (%s)
- Concepts found: %d (%s)
- Pages created: %d
- Summary: %s`,
		strings.Join(response.PagesCreated, ", "),
		len(response.EntitiesFound),
		strings.Join(response.EntitiesFound, ", "),
		len(response.ConceptsFound),
		strings.Join(response.ConceptsFound, ", "),
		len(response.PagesCreated),
		truncateSummary(response.Summary, 200),
	)

	return types.LogEntry{
		Timestamp: time.Now(),
		Type:      "ingest",
		Title:     sourceTitle,
		Details:   details,
	}
}

// Helper functions

func (e *Engine) getSourceTitle(sourcePath string) string {
	filename := filepath.Base(sourcePath)
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}

func truncateSummary(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func sanitizeFilename(title string) string {
	// Convert to lowercase
	filename := strings.ToLower(title)

	// Replace spaces with hyphens
	filename = strings.ReplaceAll(filename, " ", "-")

	// Remove special characters (keep only alphanumeric and hyphens)
	var result strings.Builder
	for _, r := range filename {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	// Remove multiple consecutive hyphens
	filename = result.String()
	for strings.Contains(filename, "--") {
		filename = strings.ReplaceAll(filename, "--", "-")
	}

	// Trim hyphens from start and end
	filename = strings.Trim(filename, "-")

	return filename
}