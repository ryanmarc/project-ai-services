package query

import (
	"fmt"

	"github.com/IBM/project-ai-services/wiki-service/internal/wiki"
)

// Tool represents a tool that the LLM can call
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall represents a call to a tool by the LLM
type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	ToolName string `json:"tool_name"`
	Success  bool   `json:"success"`
	Result   string `json:"result"`
	Error    string `json:"error,omitempty"`
}

// ToolExecutor handles tool execution
type ToolExecutor struct {
	wikiManager *wiki.Manager
	pageCache   map[string]string // Simple cache for read pages
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(wikiManager *wiki.Manager) *ToolExecutor {
	return &ToolExecutor{
		wikiManager: wikiManager,
		pageCache:   make(map[string]string),
	}
}

// GetAvailableTools returns the list of tools available to the LLM
func (te *ToolExecutor) GetAvailableTools() []Tool {
	return []Tool{
		{
			Name:        "read_page",
			Description: "Read the content of a specific wiki page. Use this to get detailed information from a page you've identified as relevant.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The relative path to the wiki page (e.g., 'concepts/machine-learning.md' or 'entities/john-doe.md')",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "list_links",
			Description: "Extract all markdown links from a wiki page. Use this to discover related pages you can navigate to.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The relative path to the wiki page",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

// ExecuteTool executes a tool call
func (te *ToolExecutor) ExecuteTool(toolCall ToolCall) ToolResult {
	switch toolCall.Name {
	case "read_page":
		return te.executeReadPage(toolCall)
	case "list_links":
		return te.executeListLinks(toolCall)
	default:
		return ToolResult{
			ToolName: toolCall.Name,
			Success:  false,
			Error:    fmt.Sprintf("unknown tool: %s", toolCall.Name),
		}
	}
}

// executeReadPage reads a wiki page
func (te *ToolExecutor) executeReadPage(toolCall ToolCall) ToolResult {
	path, ok := toolCall.Arguments["path"].(string)
	if !ok {
		return ToolResult{
			ToolName: "read_page",
			Success:  false,
			Error:    "path argument must be a string",
		}
	}

	// Check cache first
	if content, found := te.pageCache[path]; found {
		return ToolResult{
			ToolName: "read_page",
			Success:  true,
			Result:   content,
		}
	}

	// Read from wiki
	content, err := te.wikiManager.ReadPage(path)
	if err != nil {
		return ToolResult{
			ToolName: "read_page",
			Success:  false,
			Error:    fmt.Sprintf("failed to read page: %v", err),
		}
	}

	// Cache the content
	te.pageCache[path] = content

	return ToolResult{
		ToolName: "read_page",
		Success:  true,
		Result:   content,
	}
}

// executeListLinks lists all links in a page
func (te *ToolExecutor) executeListLinks(toolCall ToolCall) ToolResult {
	path, ok := toolCall.Arguments["path"].(string)
	if !ok {
		return ToolResult{
			ToolName: "list_links",
			Success:  false,
			Error:    "path argument must be a string",
		}
	}

	// Read page content (use cache if available)
	var content string
	if cached, found := te.pageCache[path]; found {
		content = cached
	} else {
		var err error
		content, err = te.wikiManager.ReadPage(path)
		if err != nil {
			return ToolResult{
				ToolName: "list_links",
				Success:  false,
				Error:    fmt.Sprintf("failed to read page: %v", err),
			}
		}
		te.pageCache[path] = content
	}

	// Extract links
	links := te.wikiManager.ExtractLinks(content)

	// Format links as a readable list
	result := fmt.Sprintf("Found %d links in %s:\n", len(links), path)
	for i, link := range links {
		result += fmt.Sprintf("%d. %s\n", i+1, link)
	}

	return ToolResult{
		ToolName: "list_links",
		Success:  true,
		Result:   result,
	}
}

// ClearCache clears the page cache
func (te *ToolExecutor) ClearCache() {
	te.pageCache = make(map[string]string)
}

// GetCachedPages returns the list of cached page paths
func (te *ToolExecutor) GetCachedPages() []string {
	paths := make([]string, 0, len(te.pageCache))
	for path := range te.pageCache {
		paths = append(paths, path)
	}
	return paths
}
