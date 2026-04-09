package api

import (
	"github.com/gorilla/mux"
)

// SetupRoutes configures all API routes
func (s *Server) SetupRoutes() *mux.Router {
	r := mux.NewRouter()

	// Apply middleware
	r.Use(RequestIDMiddleware)
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
