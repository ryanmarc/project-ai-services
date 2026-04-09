package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

// SetupRoutes configures all API routes
func (s *Server) SetupRoutes() *mux.Router {
	r := mux.NewRouter()

	// Apply middleware
	r.Use(LoggingMiddleware)
	r.Use(CORSMiddleware)
	r.Use(RecoveryMiddleware)

	// API v1 routes
	api := r.PathPrefix("/v1/wiki").Subrouter()

	// Ingest endpoint
	api.HandleFunc("/ingest", s.HandleIngest).Methods("POST", "OPTIONS")

	// Query endpoint
	api.HandleFunc("/query", s.HandleQuery).Methods("POST", "OPTIONS")

	// Pages endpoints
	api.HandleFunc("/pages", s.HandleGetPages).Methods("GET", "OPTIONS")
	api.HandleFunc("/pages/{path:.*}", s.HandleGetPage).Methods("GET", "OPTIONS")

	// Index endpoint
	api.HandleFunc("/index", s.HandleGetIndex).Methods("GET", "OPTIONS")

	// Log endpoint
	api.HandleFunc("/log", s.HandleGetLog).Methods("GET", "OPTIONS")

	// Stats endpoint
	api.HandleFunc("/stats", s.HandleGetStats).Methods("GET", "OPTIONS")

	// Navigation endpoint
	api.HandleFunc("/navigation/{query_id}", s.HandleGetNavigation).Methods("GET", "OPTIONS")

	// Health check endpoint
	r.HandleFunc("/health", s.HandleHealth).Methods("GET", "OPTIONS")

	// Root endpoint
	r.HandleFunc("/", s.HandleRoot).Methods("GET")

	return r
}

// HandleRoot handles GET /
func (s *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	type RootResponse struct {
		Service   string   `json:"service"`
		Version   string   `json:"version"`
		Endpoints []string `json:"endpoints"`
	}

	respondJSON(w, http.StatusOK, RootResponse{
		Service: "Wiki Service API",
		Version: "v1",
		Endpoints: []string{
			"POST /v1/wiki/ingest",
			"POST /v1/wiki/query",
			"GET /v1/wiki/pages",
			"GET /v1/wiki/pages/{path}",
			"GET /v1/wiki/index",
			"GET /v1/wiki/log",
			"GET /v1/wiki/stats",
			"GET /v1/wiki/navigation/{query_id}",
			"GET /health",
		},
	})
}
