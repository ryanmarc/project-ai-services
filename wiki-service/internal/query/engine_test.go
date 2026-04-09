package query

import (
	"strings"
	"testing"
)

// Test helper functions

func TestSanitizeQueryTitle(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "simple query",
			query:    "What is machine learning",
			expected: "What-is-machine-learning",
		},
		{
			name:     "query with special characters",
			query:    "How does X compare to Y?",
			expected: "How-does-X-compare-to-Y",
		},
		{
			name:     "long query gets truncated",
			query:    "This is a very long query that should be truncated to fifty characters maximum",
			expected: "This-is-a-very-long-query-that-should-be-trun",
		},
		{
			name:     "query with multiple spaces",
			query:    "What  is   the   difference",
			expected: "What-is-the-difference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Engine{}
			result := e.sanitizeQueryTitle(tt.query)

			// Check that result starts with "query-" and timestamp
			if !strings.HasPrefix(result, "query-") {
				t.Errorf("expected result to start with 'query-', got: %s", result)
			}

			// Check that the sanitized query part is present
			if !strings.Contains(result, tt.expected) {
				t.Errorf("expected result to contain '%s', got: %s", tt.expected, result)
			}
		})
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "concept path",
			path:     "concepts/machine-learning.md",
			expected: "Machine Learning",
		},
		{
			name:     "entity path",
			path:     "entities/john-doe.md",
			expected: "John Doe",
		},
		{
			name:     "source path",
			path:     "sources/technical-spec.md",
			expected: "Technical Spec",
		},
		{
			name:     "simple filename",
			path:     "overview.md",
			expected: "Overview",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Engine{}
			result := e.extractTitle(tt.path)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLen   int
		expected string
	}{
		{
			name:     "text shorter than max",
			text:     "Short text",
			maxLen:   20,
			expected: "Short text",
		},
		{
			name:     "text longer than max",
			text:     "This is a long text that needs truncation",
			maxLen:   20,
			expected: "This is a long text ...",
		},
		{
			name:     "text exactly at max",
			text:     "Exactly twenty chars",
			maxLen:   20,
			expected: "Exactly twenty chars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Engine{}
			result := e.truncate(tt.text, tt.maxLen)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestConvertCitations(t *testing.T) {
	e := &Engine{}

	citations := []Citation{
		{
			Page:      "concepts/machine-learning.md",
			Excerpt:   "Machine learning is a subset of AI",
			Relevance: 0.95,
		},
		{
			Page:      "entities/john-doe.md",
			Excerpt:   "John Doe is a researcher",
			Relevance: 0.80,
		},
	}

	result := e.convertCitations(citations)

	if len(result) != 2 {
		t.Errorf("expected 2 citations, got %d", len(result))
	}

	if result[0].PagePath != "concepts/machine-learning.md" {
		t.Errorf("expected page path 'concepts/machine-learning.md', got '%s'", result[0].PagePath)
	}

	if result[0].PageTitle != "Machine Learning" {
		t.Errorf("expected page title 'Machine Learning', got '%s'", result[0].PageTitle)
	}

	if result[0].Relevance != 0.95 {
		t.Errorf("expected relevance 0.95, got %f", result[0].Relevance)
	}
}

// Test prompt functions

func TestQueryNavigationPrompt(t *testing.T) {
	query := "What is machine learning?"
	indexContent := "# Wiki Index\n\n## Concepts\n- Machine Learning\n- Neural Networks"

	prompt := QueryNavigationPrompt(query, indexContent)

	// Check that prompt contains key elements
	if !strings.Contains(prompt, query) {
		t.Error("prompt should contain the query")
	}

	if !strings.Contains(prompt, indexContent) {
		t.Error("prompt should contain the index content")
	}

	if !strings.Contains(prompt, "read_page") {
		t.Error("prompt should mention read_page tool")
	}

	if !strings.Contains(prompt, "list_links") {
		t.Error("prompt should mention list_links tool")
	}

	if !strings.Contains(prompt, "JSON") {
		t.Error("prompt should request JSON response")
	}
}

func TestSynthesisPrompt(t *testing.T) {
	query := "What is machine learning?"
	pagesRead := []string{"concepts/machine-learning.md", "concepts/neural-networks.md"}
	pageContents := map[string]string{
		"concepts/machine-learning.md": "Machine learning is a subset of AI...",
		"concepts/neural-networks.md":  "Neural networks are computing systems...",
	}

	prompt := SynthesisPrompt(query, pagesRead, pageContents)

	// Check that prompt contains key elements
	if !strings.Contains(prompt, query) {
		t.Error("prompt should contain the query")
	}

	for _, page := range pagesRead {
		if !strings.Contains(prompt, page) {
			t.Errorf("prompt should contain page: %s", page)
		}
	}

	if !strings.Contains(prompt, "citations") {
		t.Error("prompt should request citations")
	}

	if !strings.Contains(prompt, "JSON") {
		t.Error("prompt should request JSON response")
	}
}

func TestSimpleQueryPrompt(t *testing.T) {
	query := "What is machine learning?"
	indexContent := "# Wiki Index\n\n## Concepts\n- Machine Learning"
	relevantPages := []string{"concepts/machine-learning.md"}

	prompt := SimpleQueryPrompt(query, indexContent, relevantPages)

	if !strings.Contains(prompt, query) {
		t.Error("prompt should contain the query")
	}

	if !strings.Contains(prompt, indexContent) {
		t.Error("prompt should contain the index content")
	}

	if !strings.Contains(prompt, relevantPages[0]) {
		t.Error("prompt should contain relevant pages")
	}
}

func TestQueryPageContentPrompt(t *testing.T) {
	query := "What is machine learning?"
	answer := "Machine learning is a subset of artificial intelligence..."
	citations := []Citation{
		{
			Page:      "concepts/machine-learning.md",
			Excerpt:   "ML is a subset of AI",
			Relevance: 0.95,
		},
	}
	pagesRead := []string{"concepts/machine-learning.md"}

	prompt := QueryPageContentPrompt(query, answer, citations, pagesRead)

	if !strings.Contains(prompt, query) {
		t.Error("prompt should contain the query")
	}

	if !strings.Contains(prompt, answer) {
		t.Error("prompt should contain the answer")
	}

	if !strings.Contains(prompt, "markdown") {
		t.Error("prompt should request markdown format")
	}
}

func TestFormatNavigationPath(t *testing.T) {
	tests := []struct {
		name     string
		steps    []string
		expected string
	}{
		{
			name:     "empty path",
			steps:    []string{},
			expected: "No pages read",
		},
		{
			name:     "single page",
			steps:    []string{"concepts/machine-learning.md"},
			expected: "Navigation path (1 pages):\nconcepts/machine-learning.md",
		},
		{
			name:     "multiple pages",
			steps:    []string{"concepts/machine-learning.md", "concepts/neural-networks.md"},
			expected: "Navigation path (2 pages):\nconcepts/machine-learning.md → concepts/neural-networks.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatNavigationPath(tt.steps)
			if result != tt.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestFormatCitations(t *testing.T) {
	tests := []struct {
		name      string
		citations []Citation
		contains  []string
	}{
		{
			name:      "empty citations",
			citations: []Citation{},
			contains:  []string{"No citations"},
		},
		{
			name: "single citation",
			citations: []Citation{
				{
					Page:      "concepts/machine-learning.md",
					Excerpt:   "ML is a subset of AI",
					Relevance: 0.95,
				},
			},
			contains: []string{"Citations (1)", "machine-learning.md", "ML is a subset of AI", "0.95"},
		},
		{
			name: "multiple citations",
			citations: []Citation{
				{
					Page:      "concepts/machine-learning.md",
					Excerpt:   "ML is a subset of AI",
					Relevance: 0.95,
				},
				{
					Page:      "concepts/neural-networks.md",
					Excerpt:   "Neural networks are computing systems",
					Relevance: 0.85,
				},
			},
			contains: []string{"Citations (2)", "machine-learning.md", "neural-networks.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCitations(tt.citations)
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected result to contain '%s', got:\n%s", expected, result)
				}
			}
		})
	}
}

func TestParseNavigationStep(t *testing.T) {
	tests := []struct {
		name      string
		jsonStr   string
		expectErr bool
		expected  *NavigationStep
	}{
		{
			name:      "valid read_page action",
			jsonStr:   `{"action":"read_page","target":"concepts/ml.md","reasoning":"Need ML info","need_more_info":true}`,
			expectErr: false,
			expected: &NavigationStep{
				Action:       "read_page",
				Target:       "concepts/ml.md",
				Reasoning:    "Need ML info",
				NeedMoreInfo: true,
			},
		},
		{
			name:      "valid synthesize action",
			jsonStr:   `{"action":"synthesize","reasoning":"Have enough info","need_more_info":false}`,
			expectErr: false,
			expected: &NavigationStep{
				Action:       "synthesize",
				Reasoning:    "Have enough info",
				NeedMoreInfo: false,
			},
		},
		{
			name:      "invalid JSON",
			jsonStr:   `{invalid json}`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseNavigationStep(tt.jsonStr)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Action != tt.expected.Action {
				t.Errorf("expected action '%s', got '%s'", tt.expected.Action, result.Action)
			}

			if result.Target != tt.expected.Target {
				t.Errorf("expected target '%s', got '%s'", tt.expected.Target, result.Target)
			}

			if result.NeedMoreInfo != tt.expected.NeedMoreInfo {
				t.Errorf("expected need_more_info %v, got %v", tt.expected.NeedMoreInfo, result.NeedMoreInfo)
			}
		})
	}
}

func TestParseQueryAnalysis(t *testing.T) {
	tests := []struct {
		name      string
		jsonStr   string
		expectErr bool
	}{
		{
			name: "valid analysis",
			jsonStr: `{
				"pages_to_read": ["concepts/ml.md"],
				"answer": "Machine learning is...",
				"citations": [{"page":"concepts/ml.md","excerpt":"ML is...","relevance":0.9}],
				"contradictions": [],
				"gaps": [],
				"follow_up_questions": ["What about deep learning?"]
			}`,
			expectErr: false,
		},
		{
			name:      "invalid JSON",
			jsonStr:   `{invalid}`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseQueryAnalysis(tt.jsonStr)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

// Test tool executor

func TestToolExecutorGetAvailableTools(t *testing.T) {
	te := &ToolExecutor{
		pageCache: make(map[string]string),
	}

	tools := te.GetAvailableTools()

	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	if !toolNames["read_page"] {
		t.Error("expected read_page tool")
	}

	if !toolNames["list_links"] {
		t.Error("expected list_links tool")
	}
}

func TestToolExecutorClearCache(t *testing.T) {
	te := &ToolExecutor{
		pageCache: map[string]string{
			"page1.md": "content1",
			"page2.md": "content2",
		},
	}

	if len(te.pageCache) != 2 {
		t.Errorf("expected 2 cached pages, got %d", len(te.pageCache))
	}

	te.ClearCache()

	if len(te.pageCache) != 0 {
		t.Errorf("expected 0 cached pages after clear, got %d", len(te.pageCache))
	}
}

func TestToolExecutorGetCachedPages(t *testing.T) {
	te := &ToolExecutor{
		pageCache: map[string]string{
			"page1.md": "content1",
			"page2.md": "content2",
		},
	}

	cached := te.GetCachedPages()

	if len(cached) != 2 {
		t.Errorf("expected 2 cached pages, got %d", len(cached))
	}

	// Check that both pages are in the list
	found := make(map[string]bool)
	for _, page := range cached {
		found[page] = true
	}

	if !found["page1.md"] || !found["page2.md"] {
		t.Error("expected both page1.md and page2.md in cached pages")
	}
}
