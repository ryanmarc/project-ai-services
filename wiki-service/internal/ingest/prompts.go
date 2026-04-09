package ingest

import (
	"fmt"
	"strings"
)

// Entity represents an extracted entity
type Entity struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Concept represents an extracted concept
type Concept struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Connection represents a connection to an existing wiki page
type Connection struct {
	ExistingPage string `json:"existing_page"`
	Relationship string `json:"relationship"`
}

// SuggestedUpdate represents a suggested update to an existing page
type SuggestedUpdate struct {
	PagePath string `json:"page_path"`
	Reason   string `json:"reason"`
}

// IngestAnalysisResult represents the result of document analysis
type IngestAnalysisResult struct {
	Summary          string            `json:"summary"`
	Entities         []Entity          `json:"entities"`
	Concepts         []Concept         `json:"concepts"`
	Connections      []Connection      `json:"connections"`
	SuggestedUpdates []SuggestedUpdate `json:"suggested_updates"`
}

// IngestAnalysisPrompt generates a prompt for analyzing a document during ingest
func IngestAnalysisPrompt(documentContent, indexContent string) string {
	return fmt.Sprintf(`You are maintaining a personal knowledge wiki. A new source document has been added.

Your task:
1. Read and analyze the source document
2. Extract key entities (people, organizations, concepts, technologies, places)
3. Identify main themes and topics
4. Generate a comprehensive summary (2-3 paragraphs)
5. Identify connections to existing wiki pages
6. Suggest new pages to create or existing pages to update

Source document:
%s

Existing wiki index:
%s

Respond in JSON format with this exact structure:
{
  "summary": "A comprehensive 2-3 paragraph summary of the document",
  "entities": [
    {
      "name": "Entity Name",
      "type": "person|organization|technology|place|other",
      "description": "Brief description of the entity and its role in the document"
    }
  ],
  "concepts": [
    {
      "name": "Concept Name",
      "description": "Brief description of the concept and why it's important"
    }
  ],
  "connections": [
    {
      "existing_page": "path/to/existing/page.md",
      "relationship": "How this document relates to the existing page"
    }
  ],
  "suggested_updates": [
    {
      "page_path": "path/to/page.md",
      "reason": "Why this page should be updated with information from this document"
    }
  ]
}`, documentContent, indexContent)
}

// EntityPageContentPrompt generates a prompt for creating an entity page
func EntityPageContentPrompt(entityName, entityType, description, relatedInfo string) string {
	return fmt.Sprintf(`Create a wiki page for the following entity:

Entity Name: %s
Entity Type: %s
Description: %s

Related Information:
%s

Generate a well-structured markdown page with:
1. A clear title (# Entity Name)
2. An overview section describing the entity
3. Key facts or attributes
4. Related concepts or entities (as markdown links where appropriate)
5. References section

The page should be informative, well-organized, and use proper markdown formatting.
Include placeholder links like [Concept Name](../concepts/concept-name.md) for related items.`, entityName, entityType, description, relatedInfo)
}

// ConceptPageContentPrompt generates a prompt for creating a concept page
func ConceptPageContentPrompt(conceptName, description, relatedInfo string) string {
	return fmt.Sprintf(`Create a wiki page for the following concept:

Concept Name: %s
Description: %s

Related Information:
%s

Generate a well-structured markdown page with:
1. A clear title (# Concept Name)
2. An overview section explaining the concept
3. Key characteristics or principles
4. Applications or examples
5. Related concepts or entities (as markdown links where appropriate)
6. References section

The page should be educational, well-organized, and use proper markdown formatting.
Include placeholder links like [Related Concept](../concepts/related-concept.md) or [Entity Name](../entities/entity-name.md) for related items.`, conceptName, description, relatedInfo)
}

// SourceSummaryPrompt generates a prompt for creating a source summary page
func SourceSummaryPrompt(sourceTitle, summary, entities, concepts string) string {
	return fmt.Sprintf(`Create a wiki summary page for the following source document:

Source Title: %s
Summary: %s

Entities Found:
%s

Concepts Found:
%s

Generate a well-structured markdown page with:
1. A clear title (# Source Title)
2. An overview section with the summary
3. Key Entities section (with links to entity pages)
4. Key Concepts section (with links to concept pages)
5. Main Takeaways section (bullet points)
6. Metadata section (date processed, etc.)

Use proper markdown formatting and create links like:
- [Entity Name](../entities/entity-name.md)
- [Concept Name](../concepts/concept-name.md)`, sourceTitle, summary, entities, concepts)
}

// UpdateExistingPagePrompt generates a prompt for updating an existing page
func UpdateExistingPagePrompt(pagePath, currentContent, newInformation string) string {
	return fmt.Sprintf(`You need to update an existing wiki page with new information.

Page Path: %s

Current Content:
%s

New Information to Integrate:
%s

Your task:
1. Read the current page content
2. Identify where the new information fits
3. Integrate the new information seamlessly
4. Maintain the existing structure and style
5. Add cross-references where appropriate
6. Update any relevant sections

Return the complete updated page content in markdown format.
Preserve all existing links and add new ones where appropriate.`, pagePath, currentContent, newInformation)
}

// FormatEntityList formats a list of entities for display
func FormatEntityList(entities []Entity) string {
	if len(entities) == 0 {
		return "None"
	}

	var parts []string
	for _, e := range entities {
		parts = append(parts, fmt.Sprintf("- %s (%s): %s", e.Name, e.Type, e.Description))
	}
	return strings.Join(parts, "\n")
}

// FormatConceptList formats a list of concepts for display
func FormatConceptList(concepts []Concept) string {
	if len(concepts) == 0 {
		return "None"
	}

	var parts []string
	for _, c := range concepts {
		parts = append(parts, fmt.Sprintf("- %s: %s", c.Name, c.Description))
	}
	return strings.Join(parts, "\n")
}
