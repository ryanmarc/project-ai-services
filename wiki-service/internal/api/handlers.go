package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/IBM/project-ai-services/wiki-service/internal/errors"
	"github.com/IBM/project-ai-services/wiki-service/internal/ingest"
	"github.com/IBM/project-ai-services/wiki-service/internal/llm"
	"github.com/IBM/project-ai-services/wiki-service/internal/logger"
	"github.com/IBM/project-ai-services/wiki-service/internal/query"
	"github.com/IBM/project-ai-services/wiki-service/internal/wiki"
	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
	"github.com/gorilla/mux"
)

// Server holds the API server dependencies
type Server struct {
	WikiManager  *wiki.Manager
	LLMClient    *llm.Client
	IngestEngine *ingest.Engine
	QueryEngine  *query.Engine
	WikiConfig   types.WikiConfig

	// Query navigation tracking
	queryNavigations map[string]*QueryNavigation
	navMutex         sync.RWMutex
}

// QueryNavigation tracks the navigation path for a query
type QueryNavigation struct {
	QueryID        string    `json:"query_id"`
	Query          string    `json:"query"`
	NavigationPath []string  `json:"navigation_path"`
	PagesRead      []string  `json:"pages_read"`
	Timestamp      time.Time `json:"timestamp"`
}

// NewServer creates a new API server
func NewServer(wikiManager *wiki.Manager, llmClient *llm.Client, wikiConfig types.WikiConfig) *Server {
	return &Server{
		WikiManager:      wikiManager,
		LLMClient:        llmClient,
		IngestEngine:     ingest.NewEngine(wikiManager, llmClient),
		QueryEngine:      query.NewEngine(wikiManager, llmClient, wikiConfig.MaxPagesPerQuery),
		WikiConfig:       wikiConfig,
		queryNavigations: make(map[string]*QueryNavigation),
	}
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message"`
	Code    int                    `json:"code"`
	Context map[string]interface{} `json:"context,omitempty"`
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error("Failed to encode JSON response: %v", err)
	}
}

// respondError sends an error response
func respondError(w http.ResponseWriter, status int, message string) {
	logger.Warn("API Error: status=%d message=%s", status, message)
	respondJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
	})
}

// respondAppError sends an AppError response
func respondAppError(w http.ResponseWriter, err *errors.AppError) {
	logger.Error("API Error: code=%s message=%s", err.Code, err.Message)
	respondJSON(w, err.HTTPStatus, ErrorResponse{
		Error:   string(err.Code),
		Message: err.Message,
		Code:    err.HTTPStatus,
		Context: err.Context,
	})
}

// HandleIngest handles POST /v1/wiki/ingest
func (s *Server) HandleIngest(w http.ResponseWriter, r *http.Request) {
	logger.Info("Ingest request received")

	var req types.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr := errors.NewInvalidRequest(fmt.Sprintf("Invalid request body: %v", err))
		respondAppError(w, appErr)
		return
	}

	// Validate request
	if req.SourcePath == "" {
		appErr := errors.NewInvalidRequest("source_path is required")
		respondAppError(w, appErr)
		return
	}

	logger.Info("Ingesting document: path=%s type=%s", req.SourcePath, req.SourceType)

	// Perform ingestion
	resp, err := s.IngestEngine.Ingest(req)
	if err != nil {
		appErr := errors.NewIngestError(err, "Ingestion failed")
		appErr.WithContext("source_path", req.SourcePath)
		respondAppError(w, appErr)
		return
	}

	logger.Info("Ingest completed: pages_created=%d entities=%d concepts=%d",
		len(resp.PagesCreated), len(resp.EntitiesFound), len(resp.ConceptsFound))
	respondJSON(w, http.StatusOK, resp)
}

// HandleQuery handles POST /v1/wiki/query
func (s *Server) HandleQuery(w http.ResponseWriter, r *http.Request) {
	logger.Info("Query request received")

	var req types.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr := errors.NewInvalidRequest(fmt.Sprintf("Invalid request body: %v", err))
		respondAppError(w, appErr)
		return
	}

	// Validate request
	if req.Query == "" {
		appErr := errors.NewInvalidRequest("query is required")
		respondAppError(w, appErr)
		return
	}

	// Set defaults
	if req.MaxPages == 0 {
		req.MaxPages = s.WikiConfig.MaxPagesPerQuery
	}
	if req.OutputFormat == "" {
		req.OutputFormat = "markdown"
	}

	logger.Info("Executing query: query=%s max_pages=%d", req.Query, req.MaxPages)

	// Perform query
	resp, err := s.QueryEngine.Query(req)
	if err != nil {
		appErr := errors.NewQueryError(err, "Query execution failed")
		appErr.WithContext("query", req.Query)
		respondAppError(w, appErr)
		return
	}

	// Store navigation for tracking
	queryID := fmt.Sprintf("query-%d", time.Now().Unix())
	s.navMutex.Lock()
	s.queryNavigations[queryID] = &QueryNavigation{
		QueryID:        queryID,
		Query:          req.Query,
		NavigationPath: resp.NavigationPath,
		PagesRead:      resp.PagesRead,
		Timestamp:      time.Now(),
	}
	s.navMutex.Unlock()

	logger.Info("Query completed: query_id=%s pages_read=%d citations=%d",
		queryID, len(resp.PagesRead), len(resp.Citations))

	// Add query ID to response
	type QueryResponseWithID struct {
		types.QueryResponse
		QueryID string `json:"query_id"`
	}

	respWithID := QueryResponseWithID{
		QueryResponse: *resp,
		QueryID:       queryID,
	}

	respondJSON(w, http.StatusOK, respWithID)
}

// HandleGetPages handles GET /v1/wiki/pages
func (s *Server) HandleGetPages(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	category := r.URL.Query().Get("category")

	// Default to "all" if no category specified
	if category == "" {
		category = "all"
	}

	// List pages
	pages, err := s.WikiManager.ListPages(category)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list pages: %v", err))
		return
	}

	type PagesResponse struct {
		Pages    []string `json:"pages"`
		Count    int      `json:"count"`
		Category string   `json:"category,omitempty"`
	}

	respondJSON(w, http.StatusOK, PagesResponse{
		Pages:    pages,
		Count:    len(pages),
		Category: category,
	})
}

// HandleGetPage handles GET /v1/wiki/pages/{path}
func (s *Server) HandleGetPage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pagePath := vars["path"]

	if pagePath == "" {
		respondError(w, http.StatusBadRequest, "page path is required")
		return
	}

	// Read page
	content, err := s.WikiManager.ReadPage(pagePath)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, fmt.Sprintf("Page not found: %s", pagePath))
		} else {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read page: %v", err))
		}
		return
	}

	type PageResponse struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	respondJSON(w, http.StatusOK, PageResponse{
		Path:    pagePath,
		Content: content,
	})
}

// HandleGetIndex handles GET /v1/wiki/index
func (s *Server) HandleGetIndex(w http.ResponseWriter, r *http.Request) {
	content, err := s.WikiManager.ReadIndex()
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read index: %v", err))
		return
	}

	type IndexResponse struct {
		Content string `json:"content"`
	}

	respondJSON(w, http.StatusOK, IndexResponse{
		Content: content,
	})
}

// HandleGetLog handles GET /v1/wiki/log
func (s *Server) HandleGetLog(w http.ResponseWriter, r *http.Request) {
	// Get query parameter for number of recent entries
	limitStr := r.URL.Query().Get("limit")
	limit := 0
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	var content string
	var err error

	if limit > 0 {
		// Get recent logs
		logs, err := s.WikiManager.GetRecentLogs(limit)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read log: %v", err))
			return
		}
		// Convert to string
		var sb strings.Builder
		for _, log := range logs {
			sb.WriteString(fmt.Sprintf("## [%s] %s | %s\n",
				log.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
				log.Type,
				log.Title))
			sb.WriteString(log.Description)
			sb.WriteString("\n")
			if log.Details != "" {
				sb.WriteString(log.Details)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
		content = sb.String()
	} else {
		// Get full log
		content, err = s.WikiManager.ReadLog()
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to read log: %v", err))
			return
		}
	}

	type LogResponse struct {
		Content string `json:"content"`
		Limit   int    `json:"limit,omitempty"`
	}

	respondJSON(w, http.StatusOK, LogResponse{
		Content: content,
		Limit:   limit,
	})
}

// HandleGetStats handles GET /v1/wiki/stats
func (s *Server) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.WikiManager.GetStats()
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get stats: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// HandleGetNavigation handles GET /v1/wiki/navigation/{query_id}
func (s *Server) HandleGetNavigation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queryID := vars["query_id"]

	if queryID == "" {
		respondError(w, http.StatusBadRequest, "query_id is required")
		return
	}

	s.navMutex.RLock()
	nav, exists := s.queryNavigations[queryID]
	s.navMutex.RUnlock()

	if !exists {
		respondError(w, http.StatusNotFound, fmt.Sprintf("Navigation not found for query_id: %s", queryID))
		return
	}

	respondJSON(w, http.StatusOK, nav)
}

// HandleHealth handles GET /health
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	// Check LLM health
	llmHealthy := true
	llmError := ""
	if err := s.LLMClient.Health(); err != nil {
		llmHealthy = false
		llmError = err.Error()
		logger.Warn("LLM health check failed: %v", err)
	}

	// Check wiki manager
	wikiHealthy := true
	wikiError := ""
	stats, err := s.WikiManager.GetStats()
	if err != nil {
		wikiHealthy = false
		wikiError = err.Error()
		logger.Warn("Wiki health check failed: %v", err)
	}

	status := "healthy"
	httpStatus := http.StatusOK
	if !llmHealthy || !wikiHealthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	type HealthCheck struct {
		Healthy bool   `json:"healthy"`
		Error   string `json:"error,omitempty"`
	}

	type HealthResponse struct {
		Status      string       `json:"status"`
		LLM         HealthCheck  `json:"llm"`
		Wiki        HealthCheck  `json:"wiki"`
		Timestamp   time.Time    `json:"timestamp"`
		Version     string       `json:"version"`
		Uptime      string       `json:"uptime,omitempty"`
		TotalPages  int          `json:"total_pages,omitempty"`
	}

	response := HealthResponse{
		Status: status,
		LLM: HealthCheck{
			Healthy: llmHealthy,
			Error:   llmError,
		},
		Wiki: HealthCheck{
			Healthy: wikiHealthy,
			Error:   wikiError,
		},
		Timestamp: time.Now(),
		Version:   "v0.1.0",
	}

	if wikiHealthy && stats != nil {
		response.TotalPages = stats.TotalPages
	}

	respondJSON(w, httpStatus, response)
}

// HandleRoot handles GET /
func (s *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	type EndpointInfo struct {
		Method      string `json:"method"`
		Path        string `json:"path"`
		Description string `json:"description"`
	}

	type RootResponse struct {
		Service     string         `json:"service"`
		Version     string         `json:"version"`
		Description string         `json:"description"`
		Endpoints   []EndpointInfo `json:"endpoints"`
	}

	response := RootResponse{
		Service:     "Wiki Service",
		Version:     "v0.1.0",
		Description: "LLM-powered knowledge base with persistent wiki maintenance",
		Endpoints: []EndpointInfo{
			{Method: "GET", Path: "/", Description: "Service information"},
			{Method: "GET", Path: "/health", Description: "Health check"},
			{Method: "POST", Path: "/v1/wiki/ingest", Description: "Ingest a document"},
			{Method: "POST", Path: "/v1/wiki/query", Description: "Query the wiki"},
			{Method: "GET", Path: "/v1/wiki/pages", Description: "List all pages"},
			{Method: "GET", Path: "/v1/wiki/pages/{path}", Description: "Get a specific page"},
			{Method: "GET", Path: "/v1/wiki/index", Description: "Get wiki index"},
			{Method: "GET", Path: "/v1/wiki/log", Description: "Get activity log"},
			{Method: "GET", Path: "/v1/wiki/stats", Description: "Get wiki statistics"},
			{Method: "GET", Path: "/v1/wiki/navigation/{query_id}", Description: "Get query navigation path"},
		},
	}

	respondJSON(w, http.StatusOK, response)
}
