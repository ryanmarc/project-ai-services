package ingest

// ExtractedContent represents content extracted from a document
type ExtractedContent struct {
	Text   string
	Tables []Table
	Images []Image
}

// Table represents an extracted table
type Table struct {
	Headers []string
	Rows    [][]string
	Caption string
}

// Image represents an extracted image
type Image struct {
	Path        string
	Caption     string
	Description string
}
