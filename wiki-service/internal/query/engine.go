package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/IBM/project-ai-services/wiki-service/internal/llm"
	"github.com/IBM/project-ai-services/wiki-service/internal/wiki"
	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
)

// Engine handles wiki queries with LLM navigation
type Engine struct {
	wikiManager  *wiki.Manager
	llmClient    *llm.Client
	toolExecutor *ToolExecutor
	maxPages     int
}

// NewEngine creates a new query engine
func NewEngine(wikiManager *wiki.Manager, llmClient *llm.Client, maxPages int) *Engine {
	return &Engine{
		wikiManager:  wikiManager,
		llmClient:    llmClient,
		toolExecutor: NewToolExecutor(wikiManager),
		maxPages:     maxPages,
	}
}

// Query executes a query against the wiki with LLM navigation
func (e *Engine) Query(request types.QueryRequest) (*types.QueryResponse, error) {
	// Clear cache for fresh query
	e.toolExecutor.ClearCache()

	// Step 1: Get wiki index
	indexContent, err := e.wikiManager.ReadIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	// Step 2: LLM navigation - let LLM decide which pages to read
	pagesRead, pageContents, err := e.navigateWiki(request.Query, indexContent, request.MaxPages)
	if err != nil {
		return nil, fmt.Errorf("navigation failed: %w", err)
	}

	// Step 3: Synthesize answer from pages read
	synthesis, err := e.synthesizeAnswer(request.Query, pagesRead, pageContents)
	if err != nil {
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}

	// Step 4: Build response
	response := &types.QueryResponse{
		Answer:         synthesis.Answer,
		Citations:      e.convertCitations(synthesis.Citations),
		PagesRead:      pagesRead,
		NavigationPath: pagesRead, // Navigation path is the order pages were read
		Suggestions:    synthesis.FollowUpQuestions,
	}

	// Step 5: Optionally save as page
	if request.SaveAsPage {
		savedPath, err := e.saveQueryPage(request.Query, response)
		if err != nil {
			return nil, fmt.Errorf("failed to save query page: %w", err)
		}
		response.SavedPagePath = savedPath

		// Update index
		if err := e.updateIndexForQuery(request.Query, savedPath, response); err != nil {
			return nil, fmt.Errorf("failed to update index: %w", err)
		}
	}

	// Step 6: Log query
	if err := e.logQuery(request.Query, response); err != nil {
		// Log error but don't fail the query
		fmt.Printf("Warning: failed to log query: %v\n", err)
	}

	return response, nil
}

// navigateWiki lets the LLM navigate the wiki by reading pages and following links
func (e *Engine) navigateWiki(query, indexContent string, maxPages int) ([]string, map[string]string, error) {
	if maxPages <= 0 {
		maxPages = e.maxPages
	}

	pagesRead := []string{}
	pageContents := make(map[string]string)

	// Start with initial navigation prompt
	systemPrompt := "You are a knowledge navigator. You read wiki pages and follow links to gather information. Always respond with valid JSON."

	// Use a simpler approach: ask LLM which pages to read based on index
	prompt := SimpleQueryPrompt(query, indexContent, []string{})

	var initialAnalysis struct {
		PagesToRead []string `json:"pages_to_read"`
		Reasoning   string   `json:"reasoning"`
	}

	if err := e.llmClient.CompleteJSON(systemPrompt, prompt, &initialAnalysis); err != nil {
		return nil, nil, fmt.Errorf("failed to get initial page selection: %w", err)
	}

	// Read the pages LLM identified
	for i, pagePath := range initialAnalysis.PagesToRead {
		if i >= maxPages {
			break
		}

		// Clean up path (remove leading/trailing whitespace)
		pagePath = strings.TrimSpace(pagePath)

		// Read the page
		content, err := e.wikiManager.ReadPage(pagePath)
		if err != nil {
			fmt.Printf("Warning: failed to read page %s: %v\n", pagePath, err)
			continue
		}

		pagesRead = append(pagesRead, pagePath)
		pageContents[pagePath] = content

		// Extract links and potentially follow them
		links := e.wikiManager.ExtractLinks(content)

		// Follow up to 2 relevant links per page (if we have budget)
		followedCount := 0
		for _, link := range links {
			if len(pagesRead) >= maxPages || followedCount >= 2 {
				break
			}

			// Skip if already read
			if _, alreadyRead := pageContents[link]; alreadyRead {
				continue
			}

			// Skip external links
			if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
				continue
			}

			// Read linked page
			linkedContent, err := e.wikiManager.ReadPage(link)
			if err != nil {
				continue // Skip if can't read
			}

			pagesRead = append(pagesRead, link)
			pageContents[link] = linkedContent
			followedCount++
		}
	}

	return pagesRead, pageContents, nil
}

// synthesizeAnswer synthesizes an answer from the pages read
func (e *Engine) synthesizeAnswer(query string, pagesRead []string, pageContents map[string]string) (*QueryAnalysisResult, error) {
	prompt := SynthesisPrompt(query, pagesRead, pageContents)
	systemPrompt := "You are a knowledge synthesizer. Generate comprehensive answers with citations. Always respond with valid JSON."

	var result QueryAnalysisResult
	if err := e.llmClient.CompleteJSON(systemPrompt, prompt, &result); err != nil {
		return nil, fmt.Errorf("failed to synthesize answer: %w", err)
	}

	return &result, nil
}

// saveQueryPage saves the query result as a wiki page
func (e *Engine) saveQueryPage(query string, response *types.QueryResponse) (string, error) {
	// Generate page content
	prompt := QueryPageContentPrompt(
		query,
		response.Answer,
		e.convertToInternalCitations(response.Citations),
		response.PagesRead,
	)
	systemPrompt := "You are a documentation writer. Generate well-structured markdown pages."

	content, err := e.llmClient.Complete(systemPrompt, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate query page content: %w", err)
	}

	// Create page with sanitized title
	title := e.sanitizeQueryTitle(query)
	pagePath, err := e.wikiManager.CreatePage("queries", title, content)
	if err != nil {
		return "", fmt.Errorf("failed to create query page: %w", err)
	}

	return pagePath, nil
}

// updateIndexForQuery updates the index with the saved query
func (e *Engine) updateIndexForQuery(query, pagePath string, response *types.QueryResponse) error {
	title := e.sanitizeQueryTitle(query)
	summary := e.truncate(response.Answer, 100)

	entry := types.IndexEntry{
		Title:      title,
		Path:       pagePath,
		Summary:    summary,
		Category:   "queries",
		References: len(response.PagesRead),
	}

	return e.wikiManager.UpdateIndex(entry)
}

// logQuery logs the query to the wiki log
func (e *Engine) logQuery(query string, response *types.QueryResponse) error {
	details := fmt.Sprintf(`- Pages read: %d (%s)
- Citations: %d
- Navigation path: %s
- Answer length: %d characters`,
		len(response.PagesRead),
		strings.Join(response.PagesRead, ", "),
		len(response.Citations),
		strings.Join(response.NavigationPath, " → "),
		len(response.Answer),
	)

	if response.SavedPagePath != "" {
		details += fmt.Sprintf("\n- Saved as: %s", response.SavedPagePath)
	}

	entry := types.LogEntry{
		Timestamp: time.Now(),
		Type:      "query",
		Title:     e.truncate(query, 100),
		Details:   details,
	}

	return e.wikiManager.AppendLog(entry)
}

// Helper functions

func (e *Engine) convertCitations(citations []Citation) []types.Citation {
	result := make([]types.Citation, len(citations))
	for i, cit := range citations {
		result[i] = types.Citation{
			PagePath:  cit.Page,
			PageTitle: e.extractTitle(cit.Page),
			Excerpt:   cit.Excerpt,
			Relevance: cit.Relevance,
		}
	}
	return result
}

func (e *Engine) convertToInternalCitations(citations []types.Citation) []Citation {
	result := make([]Citation, len(citations))
	for i, cit := range citations {
		result[i] = Citation{
			Page:      cit.PagePath,
			Excerpt:   cit.Excerpt,
			Relevance: cit.Relevance,
		}
	}
	return result
}

func (e *Engine) extractTitle(path string) string {
	// Extract title from path (e.g., "concepts/machine-learning.md" -> "Machine Learning")
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return path
	}

	filename := parts[len(parts)-1]
	title := strings.TrimSuffix(filename, ".md")

	// Convert hyphens to spaces and title case
	title = strings.ReplaceAll(title, "-", " ")
	words := strings.Fields(title)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}

	return strings.Join(words, " ")
}

func (e *Engine) sanitizeQueryTitle(query string) string {
	// Truncate and sanitize query for use as title
	title := query
	if len(title) > 50 {
		title = title[:50]
	}

	// Replace spaces with hyphens
	title = strings.ReplaceAll(title, " ", "-")

	// Remove special characters
	var result strings.Builder
	for _, r := range title {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	title = result.String()

	// Remove multiple consecutive hyphens
	for strings.Contains(title, "--") {
		title = strings.ReplaceAll(title, "--", "-")
	}

	// Trim hyphens
	title = strings.Trim(title, "-")

	// Add timestamp to make unique
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("query-%s-%s", timestamp, title)
}

func (e *Engine) truncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
