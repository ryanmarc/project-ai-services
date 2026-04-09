package types

import "time"

// WikiConfig holds the configuration for the wiki service
type WikiConfig struct {
	DataDir          string
	MaxPagesPerQuery int
	IndexSearchLimit int
	LogLevel         string
}

// LLMConfig holds the configuration for the LLM client
type LLMConfig struct {
	Endpoint    string
	Model       string
	MaxTokens   int
	Temperature float64
}

// IndexEntry represents an entry in the wiki index
type IndexEntry struct {
	Path       string    `json:"path"`
	Title      string    `json:"title"`
	Category   string    `json:"category"`
	Summary    string    `json:"summary"`
	References int       `json:"references"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// LogEntry represents an entry in the wiki log
type LogEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // "ingest", "query", "lint"
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Details     string    `json:"details"`
}

// IngestRequest represents a request to ingest a document
type IngestRequest struct {
	SourcePath  string `json:"source_path"`
	SourceType  string `json:"source_type"` // "pdf", "text", "image"
	Interactive bool   `json:"interactive"`
}

// IngestResponse represents the response from an ingest operation
type IngestResponse struct {
	PagesCreated  []string `json:"pages_created"`
	PagesUpdated  []string `json:"pages_updated"`
	EntitiesFound []string `json:"entities_found"`
	ConceptsFound []string `json:"concepts_found"`
	Summary       string   `json:"summary"`
	LogEntry      string   `json:"log_entry"`
}

// QueryRequest represents a request to query the wiki
type QueryRequest struct {
	Query        string `json:"query"`
	MaxPages     int    `json:"max_pages"`
	SaveAsPage   bool   `json:"save_as_page"`
	OutputFormat string `json:"output_format"` // "markdown", "table", "json"
}

// QueryResponse represents the response from a query operation
type QueryResponse struct {
	Answer         string     `json:"answer"`
	Citations      []Citation `json:"citations"`
	PagesRead      []string   `json:"pages_read"`
	NavigationPath []string   `json:"navigation_path"`
	Suggestions    []string   `json:"suggestions"`
	SavedPagePath  string     `json:"saved_page_path,omitempty"`
}

// Citation represents a citation in a query response
type Citation struct {
	PagePath  string  `json:"page_path"`
	PageTitle string  `json:"page_title"`
	Excerpt   string  `json:"excerpt"`
	Relevance float64 `json:"relevance"`
}

// LintCheck represents a lint check result
type LintCheck struct {
	Type          string   `json:"type"`     // "contradiction", "orphan", "missing_link", "stale"
	Severity      string   `json:"severity"` // "high", "medium", "low"
	Description   string   `json:"description"`
	AffectedPages []string `json:"affected_pages"`
	Suggestion    string   `json:"suggestion"`
}

// LintReport represents a lint report
type LintReport struct {
	Timestamp   time.Time   `json:"timestamp"`
	TotalChecks int         `json:"total_checks"`
	Issues      []LintCheck `json:"issues"`
	Suggestions []string    `json:"suggestions"`
}

// BrokenLink represents a broken link in the wiki
type BrokenLink struct {
	SourcePage string `json:"source_page"`
	TargetPath string `json:"target_path"`
	LineNumber int    `json:"line_number"`
}

// LinkSuggestion represents a suggested link
type LinkSuggestion struct {
	SourcePage string  `json:"source_page"`
	TargetPage string  `json:"target_page"`
	Context    string  `json:"context"`
	Confidence float64 `json:"confidence"`
}

// Entity represents an entity extracted from a document
type Entity struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "person", "organization", "concept", "technology"
	Description string `json:"description"`
}

// Concept represents a concept extracted from a document
type Concept struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Connection represents a connection between wiki pages
type Connection struct {
	ExistingPage string `json:"existing_page"`
	Relationship string `json:"relationship"`
}

// SuggestedPage represents a suggested new page
type SuggestedPage struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Reason   string `json:"reason"`
}

// IngestAnalysis represents the LLM's analysis of a document
type IngestAnalysis struct {
	Summary        string          `json:"summary"`
	Entities       []Entity        `json:"entities"`
	Concepts       []Concept       `json:"concepts"`
	Connections    []Connection    `json:"connections"`
	SuggestedPages []SuggestedPage `json:"suggested_pages"`
}

// WikiStats represents statistics about the wiki
type WikiStats struct {
	TotalSources  int       `json:"total_sources"`
	TotalPages    int       `json:"total_pages"`
	TotalEntities int       `json:"total_entities"`
	TotalConcepts int       `json:"total_concepts"`
	TotalQueries  int       `json:"total_queries"`
	LastUpdated   time.Time `json:"last_updated"`
}
