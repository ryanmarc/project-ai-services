package query

import (
	"encoding/json"
	"fmt"
	"strings"
)

// QueryAnalysisResult represents the LLM's analysis of a query
type QueryAnalysisResult struct {
	PagesToRead       []string   `json:"pages_to_read"`
	Answer            string     `json:"answer"`
	Citations         []Citation `json:"citations"`
	Contradictions    []string   `json:"contradictions"`
	Gaps              []string   `json:"gaps"`
	FollowUpQuestions []string   `json:"follow_up_questions"`
}

// Citation represents a citation in the query response
type Citation struct {
	Page      string  `json:"page"`
	Excerpt   string  `json:"excerpt"`
	Relevance float64 `json:"relevance"`
}

// NavigationStep represents a step in the LLM's navigation
type NavigationStep struct {
	Action       string `json:"action"` // "read_page", "list_links", "synthesize"
	Target       string `json:"target,omitempty"`
	Reasoning    string `json:"reasoning"`
	NeedMoreInfo bool   `json:"need_more_info"`
}

// QueryNavigationPrompt generates the initial prompt for LLM navigation
func QueryNavigationPrompt(query, indexContent string) string {
	return fmt.Sprintf(`You are answering a question using a personal knowledge wiki. You will navigate the wiki by reading pages and following links to gather information.

Question: %s

You have access to the wiki index showing all available pages:

%s

Available tools:
1. read_page(path): Read a specific wiki page to get detailed information
2. list_links(path): Get all links from a page to discover related content

Your task:
1. Analyze the question and the index to identify the most relevant pages
2. Use read_page() to read those pages
3. Follow links within pages using list_links() and read_page() to find related information
4. Continue reading until you have sufficient information or reach the page limit
5. Synthesize the information from all pages you read
6. Generate a comprehensive answer with citations

Navigation strategy:
- Start with the most directly relevant pages from the index
- Follow cross-references to related concepts
- Stop when you have sufficient information or hit the page limit
- Track which pages you've read to avoid duplicates

Respond with your next action in JSON format:
{
  "action": "read_page" | "list_links" | "synthesize",
  "target": "path/to/page.md",
  "reasoning": "why you're taking this action",
  "need_more_info": true | false
}

When you have enough information, use action "synthesize" to generate the final answer.`, query, indexContent)
}

// SynthesisPrompt generates the prompt for final answer synthesis
func SynthesisPrompt(query string, pagesRead []string, pageContents map[string]string) string {
	var contentBuilder strings.Builder
	contentBuilder.WriteString("Pages you have read:\n\n")

	for _, path := range pagesRead {
		content, ok := pageContents[path]
		if !ok {
			continue
		}
		contentBuilder.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", path, content))
	}

	return fmt.Sprintf(`You have finished navigating the wiki and gathering information. Now synthesize a comprehensive answer.

Question: %s

%s

Your task:
1. Synthesize information from all pages you read
2. Generate a clear, comprehensive answer
3. Cite specific pages for each claim
4. Note any contradictions or gaps you found
5. Suggest follow-up questions

Respond in JSON format:
{
  "answer": "comprehensive answer with inline citations like [page.md]",
  "citations": [
    {
      "page": "path/to/page.md",
      "excerpt": "relevant excerpt from the page",
      "relevance": 0.0-1.0
    }
  ],
  "contradictions": ["any contradictions found"],
  "gaps": ["information gaps or missing data"],
  "follow_up_questions": ["suggested follow-up questions"]
}`, query, contentBuilder.String())
}

// SimpleQueryPrompt generates a simpler prompt for direct query (fallback)
func SimpleQueryPrompt(query, indexContent string, relevantPages []string) string {
	var pagesSection string
	if len(relevantPages) > 0 {
		pagesSection = fmt.Sprintf("\nMost relevant pages based on index search:\n- %s\n",
			strings.Join(relevantPages, "\n- "))
	}

	return fmt.Sprintf(`You are answering a question using a personal knowledge wiki.

Question: %s

Wiki Index:
%s
%s
Based on the index, identify which pages would be most helpful to answer this question.

Respond in JSON format:
{
  "pages_to_read": ["path1.md", "path2.md", ...],
  "reasoning": "why these pages are relevant"
}`, query, indexContent, pagesSection)
}

// QueryPageContentPrompt generates content for a saved query page
func QueryPageContentPrompt(query, answer string, citations []Citation, pagesRead []string) string {
	citationsText := ""
	for _, cit := range citations {
		citationsText += fmt.Sprintf("- [%s](%s): %s\n", cit.Page, cit.Page, cit.Excerpt)
	}

	pagesReadText := strings.Join(pagesRead, ", ")

	return fmt.Sprintf(`Generate a well-structured markdown page for this query and answer.

Query: %s

Answer: %s

Citations:
%s

Pages consulted: %s

Create a markdown page with:
1. A clear title (# Query: ...)
2. The question
3. The answer with inline citations
4. A "Sources" section listing all cited pages
5. A "Related Pages" section with links to pages consulted
6. Metadata (date, pages read count)

Return only the markdown content, no JSON.`, query, answer, citationsText, pagesReadText)
}

// FormatNavigationPath formats the navigation path for display
func FormatNavigationPath(steps []string) string {
	if len(steps) == 0 {
		return "No pages read"
	}
	return fmt.Sprintf("Navigation path (%d pages):\n%s", len(steps), strings.Join(steps, " → "))
}

// FormatCitations formats citations for display
func FormatCitations(citations []Citation) string {
	if len(citations) == 0 {
		return "No citations"
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Citations (%d):\n", len(citations)))
	for i, cit := range citations {
		builder.WriteString(fmt.Sprintf("%d. [%s](%s) (relevance: %.2f)\n",
			i+1, cit.Page, cit.Page, cit.Relevance))
		builder.WriteString(fmt.Sprintf("   \"%s\"\n", cit.Excerpt))
	}
	return builder.String()
}

// ParseNavigationStep parses a navigation step from JSON
func ParseNavigationStep(jsonStr string) (*NavigationStep, error) {
	var step NavigationStep
	if err := json.Unmarshal([]byte(jsonStr), &step); err != nil {
		return nil, fmt.Errorf("failed to parse navigation step: %w", err)
	}
	return &step, nil
}

// ParseQueryAnalysis parses query analysis from JSON
func ParseQueryAnalysis(jsonStr string) (*QueryAnalysisResult, error) {
	var result QueryAnalysisResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse query analysis: %w", err)
	}
	return &result, nil
}
