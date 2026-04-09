package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents an application error code
type ErrorCode string

const (
	// General errors
	ErrInternal       ErrorCode = "INTERNAL_ERROR"
	ErrInvalidRequest ErrorCode = "INVALID_REQUEST"
	ErrNotFound       ErrorCode = "NOT_FOUND"
	ErrTimeout        ErrorCode = "TIMEOUT"

	// Wiki errors
	ErrWikiInit       ErrorCode = "WIKI_INIT_ERROR"
	ErrWikiRead       ErrorCode = "WIKI_READ_ERROR"
	ErrWikiWrite      ErrorCode = "WIKI_WRITE_ERROR"
	ErrWikiNotFound   ErrorCode = "WIKI_NOT_FOUND"
	ErrWikiValidation ErrorCode = "WIKI_VALIDATION_ERROR"

	// LLM errors
	ErrLLMConnection ErrorCode = "LLM_CONNECTION_ERROR"
	ErrLLMResponse   ErrorCode = "LLM_RESPONSE_ERROR"
	ErrLLMTimeout    ErrorCode = "LLM_TIMEOUT"
	ErrLLMParsing    ErrorCode = "LLM_PARSING_ERROR"

	// Ingest errors
	ErrIngestFile       ErrorCode = "INGEST_FILE_ERROR"
	ErrIngestProcessing ErrorCode = "INGEST_PROCESSING_ERROR"
	ErrIngestValidation ErrorCode = "INGEST_VALIDATION_ERROR"

	// Query errors
	ErrQueryExecution  ErrorCode = "QUERY_EXECUTION_ERROR"
	ErrQueryNavigation ErrorCode = "QUERY_NAVIGATION_ERROR"
	ErrQuerySave       ErrorCode = "QUERY_SAVE_ERROR"
)

// AppError represents an application error with context
type AppError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	HTTPStatus int                    `json:"-"`
	Err        error                  `json:"-"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithContext adds context to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// New creates a new AppError
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
		Context:    make(map[string]interface{}),
	}
}

// Wrap wraps an existing error with an AppError
func Wrap(err error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
		Err:        err,
		Context:    make(map[string]interface{}),
	}
}

// NewWithStatus creates a new AppError with a specific HTTP status
func NewWithStatus(code ErrorCode, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Context:    make(map[string]interface{}),
	}
}

// WrapWithStatus wraps an error with a specific HTTP status
func WrapWithStatus(err error, code ErrorCode, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Err:        err,
		Context:    make(map[string]interface{}),
	}
}

// Common error constructors

// NewInvalidRequest creates an invalid request error
func NewInvalidRequest(message string) *AppError {
	return NewWithStatus(ErrInvalidRequest, message, http.StatusBadRequest)
}

// NewNotFound creates a not found error
func NewNotFound(resource string) *AppError {
	return NewWithStatus(ErrNotFound, fmt.Sprintf("%s not found", resource), http.StatusNotFound)
}

// NewInternalError creates an internal error
func NewInternalError(message string) *AppError {
	return NewWithStatus(ErrInternal, message, http.StatusInternalServerError)
}

// WrapInternalError wraps an error as internal error
func WrapInternalError(err error, message string) *AppError {
	return WrapWithStatus(err, ErrInternal, message, http.StatusInternalServerError)
}

// NewLLMError creates an LLM error
func NewLLMError(err error, message string) *AppError {
	return WrapWithStatus(err, ErrLLMConnection, message, http.StatusServiceUnavailable)
}

// NewWikiError creates a wiki error
func NewWikiError(err error, message string) *AppError {
	return WrapWithStatus(err, ErrWikiRead, message, http.StatusInternalServerError)
}

// NewIngestError creates an ingest error
func NewIngestError(err error, message string) *AppError {
	return WrapWithStatus(err, ErrIngestProcessing, message, http.StatusInternalServerError)
}

// NewQueryError creates a query error
func NewQueryError(err error, message string) *AppError {
	return WrapWithStatus(err, ErrQueryExecution, message, http.StatusInternalServerError)
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError extracts an AppError from an error
func GetAppError(err error) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return WrapInternalError(err, "An unexpected error occurred")
}
