package wiki

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
)

// Manager handles all wiki operations
type Manager struct {
	dataDir string
	wikiDir string
}

// NewManager creates a new wiki manager
func NewManager(dataDir string) (*Manager, error) {
	wikiDir := filepath.Join(dataDir, "wiki")

	m := &Manager{
		dataDir: dataDir,
		wikiDir: wikiDir,
	}

	// Initialize directory structure
	if err := m.initializeStructure(); err != nil {
		return nil, fmt.Errorf("failed to initialize wiki structure: %w", err)
	}

	return m, nil
}

// initializeStructure creates the wiki directory structure
func (m *Manager) initializeStructure() error {
	dirs := []string{
		m.wikiDir,
		filepath.Join(m.wikiDir, "sources"),
		filepath.Join(m.wikiDir, "entities"),
		filepath.Join(m.wikiDir, "concepts"),
		filepath.Join(m.wikiDir, "queries"),
		filepath.Join(m.dataDir, "sources", "documents"),
		filepath.Join(m.dataDir, "schema"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Initialize index.md if it doesn't exist
	indexPath := filepath.Join(m.wikiDir, "index.md")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		if err := m.initializeIndex(); err != nil {
			return fmt.Errorf("failed to initialize index: %w", err)
		}
	}

	// Initialize log.md if it doesn't exist
	logPath := filepath.Join(m.wikiDir, "log.md")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		if err := m.initializeLog(); err != nil {
			return fmt.Errorf("failed to initialize log: %w", err)
		}
	}

	return nil
}

// initializeIndex creates an empty index file
func (m *Manager) initializeIndex() error {
	indexPath := filepath.Join(m.wikiDir, "index.md")
	content := fmt.Sprintf(`# Wiki Index

Last updated: %s

## Statistics
- Total sources: 0
- Total pages: 0
- Entities: 0
- Concepts: 0
- Queries: 0

## Sources (0)

## Entities (0)

## Concepts (0)

## Queries (0)
`, time.Now().Format(time.RFC3339))

	return os.WriteFile(indexPath, []byte(content), 0644)
}

// initializeLog creates an empty log file
func (m *Manager) initializeLog() error {
	logPath := filepath.Join(m.wikiDir, "log.md")
	content := fmt.Sprintf(`# Wiki Activity Log

## [%s] init | Wiki Initialized
- Created wiki directory structure
- Initialized index and log files
`, time.Now().Format(time.RFC3339))

	return os.WriteFile(logPath, []byte(content), 0644)
}

// CreatePage creates a new wiki page
func (m *Manager) CreatePage(category, title, content string) (string, error) {
	// Sanitize title for filename
	filename := m.sanitizeFilename(title) + ".md"

	// Determine directory based on category
	var dir string
	switch category {
	case "source", "sources":
		dir = filepath.Join(m.wikiDir, "sources")
	case "entity", "entities":
		dir = filepath.Join(m.wikiDir, "entities")
	case "concept", "concepts":
		dir = filepath.Join(m.wikiDir, "concepts")
	case "query", "queries":
		dir = filepath.Join(m.wikiDir, "queries")
	default:
		return "", fmt.Errorf("invalid category: %s", category)
	}

	pagePath := filepath.Join(dir, filename)

	// Check if page already exists
	if _, err := os.Stat(pagePath); err == nil {
		return "", fmt.Errorf("page already exists: %s", pagePath)
	}

	// Write page content
	if err := os.WriteFile(pagePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write page: %w", err)
	}

	// Return relative path from wiki directory
	relPath, err := filepath.Rel(m.wikiDir, pagePath)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}

	return relPath, nil
}

// UpdatePage updates an existing wiki page
func (m *Manager) UpdatePage(path, content string) error {
	fullPath := filepath.Join(m.wikiDir, path)

	// Check if page exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("page does not exist: %s", path)
	}

	// Write updated content
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to update page: %w", err)
	}

	return nil
}

// ReadPage reads a wiki page
func (m *Manager) ReadPage(path string) (string, error) {
	fullPath := filepath.Join(m.wikiDir, path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read page: %w", err)
	}

	return string(content), nil
}

// DeletePage deletes a wiki page
func (m *Manager) DeletePage(path string) error {
	fullPath := filepath.Join(m.wikiDir, path)

	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete page: %w", err)
	}

	return nil
}

// UpdateIndex updates the wiki index with a new entry
func (m *Manager) UpdateIndex(entry types.IndexEntry) error {
	indexPath := filepath.Join(m.wikiDir, "index.md")

	// Read current index
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("failed to read index: %w", err)
	}

	indexContent := string(content)

	// Parse current statistics
	stats := m.parseIndexStats(indexContent)

	// Update statistics based on category
	switch entry.Category {
	case "sources":
		stats["sources"]++
	case "entities":
		stats["entities"]++
	case "concepts":
		stats["concepts"]++
	case "queries":
		stats["queries"]++
	}
	stats["pages"]++

	// Build new index content
	newIndex := m.buildIndexContent(stats, indexContent, entry)

	// Write updated index
	if err := os.WriteFile(indexPath, []byte(newIndex), 0644); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	return nil
}

// SearchIndex searches the index for relevant pages
func (m *Manager) SearchIndex(query string) ([]types.IndexEntry, error) {
	indexPath := filepath.Join(m.wikiDir, "index.md")

	content, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	// Simple keyword-based search
	queryLower := strings.ToLower(query)
	keywords := strings.Fields(queryLower)

	var results []types.IndexEntry
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "- [") {
			lineLower := strings.ToLower(line)
			score := 0

			for _, keyword := range keywords {
				if strings.Contains(lineLower, keyword) {
					score++
				}
			}

			if score > 0 {
				entry := m.parseIndexLine(line)
				if entry != nil {
					results = append(results, *entry)
				}
			}
		}
	}

	// Sort by relevance (simple: number of keyword matches)
	sort.Slice(results, func(i, j int) bool {
		return results[i].References > results[j].References
	})

	return results, nil
}

// AppendLog appends an entry to the wiki log
func (m *Manager) AppendLog(entry types.LogEntry) error {
	logPath := filepath.Join(m.wikiDir, "log.md")

	// Format log entry
	logEntry := fmt.Sprintf("\n## [%s] %s | %s\n%s\n",
		entry.Timestamp.Format(time.RFC3339),
		entry.Type,
		entry.Title,
		entry.Details,
	)

	// Append to log file
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	return nil
}

// GetRecentLogs retrieves the n most recent log entries
func (m *Manager) GetRecentLogs(n int) ([]types.LogEntry, error) {
	logPath := filepath.Join(m.wikiDir, "log.md")

	content, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read log: %w", err)
	}

	// Parse log entries
	entries := m.parseLogEntries(string(content))

	// Return last n entries
	if len(entries) <= n {
		return entries, nil
	}

	return entries[len(entries)-n:], nil
}

// ExtractLinks extracts markdown links from content
func (m *Manager) ExtractLinks(content string) []string {
	// Regex to match markdown links: [text](path)
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := linkRegex.FindAllStringSubmatch(content, -1)

	var links []string
	for _, match := range matches {
		if len(match) > 2 {
			links = append(links, match[2])
		}
	}

	return links
}

// ValidateLinks checks for broken links in the wiki
func (m *Manager) ValidateLinks() ([]types.BrokenLink, error) {
	var brokenLinks []types.BrokenLink

	// Walk through all wiki pages
	err := filepath.Walk(m.wikiDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			links := m.ExtractLinks(string(content))
			relPath, _ := filepath.Rel(m.wikiDir, path)

			for _, link := range links {
				// Skip external links
				if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
					continue
				}

				// Check if target exists
				targetPath := filepath.Join(m.wikiDir, link)
				if _, err := os.Stat(targetPath); os.IsNotExist(err) {
					brokenLinks = append(brokenLinks, types.BrokenLink{
						SourcePage: relPath,
						TargetPath: link,
					})
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to validate links: %w", err)
	}

	return brokenLinks, nil
}

// GetStats returns statistics about the wiki
func (m *Manager) GetStats() (*types.WikiStats, error) {
	indexPath := filepath.Join(m.wikiDir, "index.md")

	content, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	stats := m.parseIndexStats(string(content))

	return &types.WikiStats{
		TotalSources:  stats["sources"],
		TotalPages:    stats["pages"],
		TotalEntities: stats["entities"],
		TotalConcepts: stats["concepts"],
		TotalQueries:  stats["queries"],
		LastUpdated:   time.Now(),
	}, nil
}

// Helper functions

func (m *Manager) sanitizeFilename(title string) string {
	// Convert to lowercase
	filename := strings.ToLower(title)

	// Replace spaces with hyphens
	filename = strings.ReplaceAll(filename, " ", "-")

	// Remove special characters
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	filename = reg.ReplaceAllString(filename, "")

	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	filename = reg.ReplaceAllString(filename, "-")

	// Trim hyphens from start and end
	filename = strings.Trim(filename, "-")

	return filename
}

func (m *Manager) parseIndexStats(content string) map[string]int {
	stats := map[string]int{
		"sources":  0,
		"pages":    0,
		"entities": 0,
		"concepts": 0,
		"queries":  0,
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		var val int
		if strings.HasPrefix(line, "- Total sources:") {
			fmt.Sscanf(line, "- Total sources: %d", &val)
			stats["sources"] = val
		} else if strings.HasPrefix(line, "- Total pages:") {
			fmt.Sscanf(line, "- Total pages: %d", &val)
			stats["pages"] = val
		} else if strings.HasPrefix(line, "- Entities:") {
			fmt.Sscanf(line, "- Entities: %d", &val)
			stats["entities"] = val
		} else if strings.HasPrefix(line, "- Concepts:") {
			fmt.Sscanf(line, "- Concepts: %d", &val)
			stats["concepts"] = val
		} else if strings.HasPrefix(line, "- Queries:") {
			fmt.Sscanf(line, "- Queries: %d", &val)
			stats["queries"] = val
		}
	}

	return stats
}

func (m *Manager) buildIndexContent(stats map[string]int, oldContent string, newEntry types.IndexEntry) string {
	var builder strings.Builder

	// Header
	builder.WriteString("# Wiki Index\n\n")
	builder.WriteString(fmt.Sprintf("Last updated: %s\n\n", time.Now().Format(time.RFC3339)))

	// Statistics
	builder.WriteString("## Statistics\n")
	builder.WriteString(fmt.Sprintf("- Total sources: %d\n", stats["sources"]))
	builder.WriteString(fmt.Sprintf("- Total pages: %d\n", stats["pages"]))
	builder.WriteString(fmt.Sprintf("- Entities: %d\n", stats["entities"]))
	builder.WriteString(fmt.Sprintf("- Concepts: %d\n", stats["concepts"]))
	builder.WriteString(fmt.Sprintf("- Queries: %d\n\n", stats["queries"]))

	// Parse existing entries by category
	entries := m.parseIndexEntries(oldContent)

	// Add new entry
	entries[newEntry.Category] = append(entries[newEntry.Category], newEntry)

	// Build sections
	categories := []string{"sources", "entities", "concepts", "queries"}
	categoryTitles := map[string]string{
		"sources":  "Sources",
		"entities": "Entities",
		"concepts": "Concepts",
		"queries":  "Queries",
	}

	for _, cat := range categories {
		count := len(entries[cat])
		builder.WriteString(fmt.Sprintf("## %s (%d)\n", categoryTitles[cat], count))

		for _, entry := range entries[cat] {
			builder.WriteString(fmt.Sprintf("- [%s](%s) - %s\n",
				entry.Title, entry.Path, entry.Summary))
		}

		builder.WriteString("\n")
	}

	return builder.String()
}

func (m *Manager) parseIndexEntries(content string) map[string][]types.IndexEntry {
	entries := make(map[string][]types.IndexEntry)
	entries["sources"] = []types.IndexEntry{}
	entries["entities"] = []types.IndexEntry{}
	entries["concepts"] = []types.IndexEntry{}
	entries["queries"] = []types.IndexEntry{}

	lines := strings.Split(content, "\n")
	currentCategory := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "## Sources") {
			currentCategory = "sources"
		} else if strings.HasPrefix(line, "## Entities") {
			currentCategory = "entities"
		} else if strings.HasPrefix(line, "## Concepts") {
			currentCategory = "concepts"
		} else if strings.HasPrefix(line, "## Queries") {
			currentCategory = "queries"
		} else if strings.HasPrefix(line, "- [") && currentCategory != "" {
			entry := m.parseIndexLine(line)
			if entry != nil {
				entry.Category = currentCategory
				entries[currentCategory] = append(entries[currentCategory], *entry)
			}
		}
	}

	return entries
}

func (m *Manager) parseIndexLine(line string) *types.IndexEntry {
	// Parse line format: - [Title](path) - Summary
	linkRegex := regexp.MustCompile(`- \[([^\]]+)\]\(([^)]+)\) - (.+)`)
	matches := linkRegex.FindStringSubmatch(line)

	if len(matches) < 4 {
		return nil
	}

	return &types.IndexEntry{
		Title:   matches[1],
		Path:    matches[2],
		Summary: matches[3],
	}
}

func (m *Manager) parseLogEntries(content string) []types.LogEntry {
	var entries []types.LogEntry

	lines := strings.Split(content, "\n")
	var currentEntry *types.LogEntry

	for _, line := range lines {
		if strings.HasPrefix(line, "## [") {
			// Save previous entry
			if currentEntry != nil {
				entries = append(entries, *currentEntry)
			}

			// Parse new entry header
			headerRegex := regexp.MustCompile(`## \[([^\]]+)\] ([^|]+) \| (.+)`)
			matches := headerRegex.FindStringSubmatch(line)

			if len(matches) >= 4 {
				timestamp, _ := time.Parse(time.RFC3339, matches[1])
				currentEntry = &types.LogEntry{
					Timestamp: timestamp,
					Type:      strings.TrimSpace(matches[2]),
					Title:     strings.TrimSpace(matches[3]),
					Details:   "",
				}
			}
		} else if currentEntry != nil && strings.HasPrefix(line, "- ") {
			currentEntry.Details += line + "\n"
		}
	}

	// Save last entry
	if currentEntry != nil {
		entries = append(entries, *currentEntry)
	}

	return entries
}

// ReadIndex returns the full index content
func (m *Manager) ReadIndex() (string, error) {
	indexPath := filepath.Join(m.wikiDir, "index.md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return "", fmt.Errorf("failed to read index: %w", err)
	}
	return string(content), nil
}

// ReadLog returns the full log content
func (m *Manager) ReadLog() (string, error) {
	logPath := filepath.Join(m.wikiDir, "log.md")
	content, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to read log: %w", err)
	}
	return string(content), nil
}

// ListPages lists all pages in a category
func (m *Manager) ListPages(category string) ([]string, error) {
	var dir string
	switch category {
	case "sources":
		dir = filepath.Join(m.wikiDir, "sources")
	case "entities":
		dir = filepath.Join(m.wikiDir, "entities")
	case "concepts":
		dir = filepath.Join(m.wikiDir, "concepts")
	case "queries":
		dir = filepath.Join(m.wikiDir, "queries")
	case "all":
		// Return all pages
		var allPages []string
		for _, cat := range []string{"sources", "entities", "concepts", "queries"} {
			pages, err := m.ListPages(cat)
			if err != nil {
				return nil, err
			}
			allPages = append(allPages, pages...)
		}
		return allPages, nil
	default:
		return nil, fmt.Errorf("invalid category: %s", category)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var pages []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			relPath := filepath.Join(category, entry.Name())
			pages = append(pages, relPath)
		}
	}

	return pages, nil
}

// MarshalJSON for debugging
func (m *Manager) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"dataDir": m.dataDir,
		"wikiDir": m.wikiDir,
	})
}
