package llm

import (
	"fmt"
	"strings"
)

// PromptTemplate represents a prompt template
type PromptTemplate struct {
	System string
	User   string
}

// IngestPromptTemplate returns the prompt template for document ingestion
func IngestPromptTemplate() PromptTemplate {
	return PromptTemplate{
		System: `You are maintaining a personal knowledge wiki. A new source document has been added.

Your task is to analyze the document and extract structured information that will be used to create and update wiki pages.

You must respond in valid JSON format with the following structure:
{
  "summary": "A comprehensive summary of the document (2-3 paragraphs)",
  "entities": [
    {
      "name": "Entity name",
      "type": "person|organization|technology|location|other",
      "description": "Brief description of the entity"
    }
  ],
  "concepts": [
    {
      "name": "Concept name",
      "description": "Brief description of the concept"
    }
  ],
  "connections": [
    {
      "existing_page": "Path to existing wiki page (if known)",
      "relationship": "How this document relates to that page"
    }
  ],
  "suggested_pages": [
    {
      "title": "Suggested page title",
      "category": "entities|concepts",
      "reason": "Why this page should be created"
    }
  ]
}

Focus on:
- Key entities (people, organizations, technologies, locations)
- Main concepts and themes
- Relationships to existing knowledge
- Important facts and claims

Be thorough but concise. Extract only the most important information.`,
		User: `Source document:
{{DOCUMENT}}

{{INDEX}}

Analyze this document and respond with the JSON structure described in the system prompt.`,
	}
}

// QueryPromptTemplate returns the prompt template for querying the wiki
func QueryPromptTemplate() PromptTemplate {
	return PromptTemplate{
		System: `You are answering questions using a personal knowledge wiki.

You have access to a wiki index that lists all available pages. You can read specific pages to gather information.

Your task:
1. Analyze the question
2. Identify which wiki pages are most relevant
3. Read those pages (you will be given their content)
4. Synthesize information from multiple pages
5. Generate a comprehensive answer with citations

Respond in valid JSON format:
{
  "pages_to_read": ["path1", "path2", ...],
  "answer": "Your comprehensive answer (after reading pages)",
  "citations": [
    {
      "page": "path/to/page.md",
      "excerpt": "Relevant excerpt from the page",
      "relevance": 0.9
    }
  ],
  "contradictions": ["Any contradictions found between pages"],
  "gaps": ["Information gaps that couldn't be answered"],
  "follow_up_questions": ["Suggested follow-up questions"]
}

Important:
- First, list the pages you want to read in "pages_to_read"
- After reading, provide the answer with citations
- Be specific about which page each claim comes from
- Note any contradictions or gaps in knowledge`,
		User: `Question: {{QUERY}}

Wiki Index:
{{INDEX}}

{{PAGES_CONTENT}}

Respond with the JSON structure described in the system prompt.`,
	}
}

// LintPromptTemplate returns the prompt template for wiki health checks
func LintPromptTemplate() PromptTemplate {
	return PromptTemplate{
		System: `You are performing a health check on a personal knowledge wiki.

Your task is to identify issues and suggest improvements:
1. Contradictions between pages
2. Orphan pages with no inbound links
3. Missing cross-references
4. Frequently mentioned concepts that lack their own page
5. Potentially stale information

Respond in valid JSON format:
{
  "contradictions": [
    {
      "pages": ["path1", "path2"],
      "issue": "Description of the contradiction",
      "severity": "high|medium|low"
    }
  ],
  "orphans": ["path/to/orphan.md"],
  "missing_links": [
    {
      "from": "path/from.md",
      "to": "path/to.md",
      "reason": "Why these should be linked"
    }
  ],
  "suggested_pages": [
    {
      "title": "Suggested page title",
      "reason": "Why this page should exist",
      "mentions": 5
    }
  ],
  "stale_info": [
    {
      "page": "path/to/page.md",
      "reason": "Why this might be stale"
    }
  ]
}`,
		User: `Wiki pages to analyze:
{{PAGES}}

Wiki index:
{{INDEX}}

Perform a health check and respond with the JSON structure described in the system prompt.`,
	}
}

// EntityPageTemplate returns a template for entity pages
func EntityPageTemplate(entity, entityType, description, sources string) string {
	return fmt.Sprintf(`# %s

**Type:** %s

## Description

%s

## Mentioned In

%s

## Related Concepts

(To be filled as connections are discovered)

---
*Last updated: %s*
`, entity, entityType, description, sources, "{{TIMESTAMP}}")
}

// ConceptPageTemplate returns a template for concept pages
func ConceptPageTemplate(concept, description, sources string) string {
	return fmt.Sprintf(`# %s

## Overview

%s

## Key Points

(To be filled from source analysis)

## Related Entities

(To be filled as connections are discovered)

## Related Concepts

(To be filled as connections are discovered)

## Mentioned In

%s

---
*Last updated: %s*
`, concept, description, sources, "{{TIMESTAMP}}")
}

// SourceSummaryTemplate returns a template for source summary pages
func SourceSummaryTemplate(title, summary, entities, concepts string) string {
	return fmt.Sprintf(`# %s

## Summary

%s

## Key Entities

%s

## Key Concepts

%s

---
*Source processed: %s*
`, title, summary, entities, concepts, "{{TIMESTAMP}}")
}

// Fill replaces placeholders in a template
func Fill(template string, replacements map[string]string) string {
	result := template
	for key, value := range replacements {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// FormatEntityList formats a list of entities for display
func FormatEntityList(entities []string) string {
	if len(entities) == 0 {
		return "None"
	}

	var items []string
	for _, entity := range entities {
		items = append(items, fmt.Sprintf("- %s", entity))
	}
	return strings.Join(items, "\n")
}

// FormatConceptList formats a list of concepts for display
func FormatConceptList(concepts []string) string {
	if len(concepts) == 0 {
		return "None"
	}

	var items []string
	for _, concept := range concepts {
		items = append(items, fmt.Sprintf("- %s", concept))
	}
	return strings.Join(items, "\n")
}

// FormatSourceList formats a list of sources for display
func FormatSourceList(sources []string) string {
	if len(sources) == 0 {
		return "None"
	}

	var items []string
	for _, source := range sources {
		items = append(items, fmt.Sprintf("- [%s](%s)", source, source))
	}
	return strings.Join(items, "\n")
}
