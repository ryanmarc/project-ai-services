package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	config := types.LLMConfig{
		Endpoint:    "http://localhost:8000",
		Model:       "test-model",
		MaxTokens:   1000,
		Temperature: 0.5,
	}

	client := NewClient(config)
	require.NotNil(t, client)
	assert.Equal(t, config, client.config)
	assert.NotNil(t, client.httpClient)
}

func TestComplete(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request
		var req CompletionRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "test-model", req.Model)
		assert.Len(t, req.Messages, 2)
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "user", req.Messages[1].Role)

		// Send response
		resp := CompletionResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "test-model",
			Choices: []Choice{
				{
					Index: 0,
					Message: Message{
						Role:    "assistant",
						Content: "This is a test response",
					},
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	config := types.LLMConfig{
		Endpoint:    server.URL,
		Model:       "test-model",
		MaxTokens:   1000,
		Temperature: 0.5,
	}
	client := NewClient(config)

	// Test completion
	result, err := client.Complete("System prompt", "User prompt")
	require.NoError(t, err)
	assert.Equal(t, "This is a test response", result)
}

func TestCompleteJSON(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := CompletionResponse{
			Choices: []Choice{
				{
					Message: Message{
						Content: `{"name": "test", "value": 42}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := types.LLMConfig{
		Endpoint:    server.URL,
		Model:       "test-model",
		MaxTokens:   1000,
		Temperature: 0.5,
	}
	client := NewClient(config)

	// Test JSON completion
	var result map[string]interface{}
	err := client.CompleteJSON("System", "User", &result)
	require.NoError(t, err)
	assert.Equal(t, "test", result["name"])
	assert.Equal(t, float64(42), result["value"])
}

func TestCompleteJSONWithCodeBlock(t *testing.T) {
	// Create mock server that returns JSON in markdown code block
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := CompletionResponse{
			Choices: []Choice{
				{
					Message: Message{
						Content: "```json\n{\"name\": \"test\", \"value\": 42}\n```",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := types.LLMConfig{
		Endpoint:    server.URL,
		Model:       "test-model",
		MaxTokens:   1000,
		Temperature: 0.5,
	}
	client := NewClient(config)

	// Test JSON completion with code block
	var result map[string]interface{}
	err := client.CompleteJSON("System", "User", &result)
	require.NoError(t, err)
	assert.Equal(t, "test", result["name"])
	assert.Equal(t, float64(42), result["value"])
}

func TestCompleteError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	config := types.LLMConfig{
		Endpoint:    server.URL,
		Model:       "test-model",
		MaxTokens:   1000,
		Temperature: 0.5,
	}
	client := NewClient(config)

	// Test error handling
	_, err := client.Complete("System", "User")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLM request failed")
}

func TestCompleteNoChoices(t *testing.T) {
	// Create mock server that returns no choices
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := CompletionResponse{
			Choices: []Choice{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := types.LLMConfig{
		Endpoint:    server.URL,
		Model:       "test-model",
		MaxTokens:   1000,
		Temperature: 0.5,
	}
	client := NewClient(config)

	// Test no choices error
	_, err := client.Complete("System", "User")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no choices")
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain JSON",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON in code block",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: "{\"key\": \"value\"}\n",
		},
		{
			name:     "JSON in code block without language",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: "{\"key\": \"value\"}\n",
		},
		{
			name:     "no code block",
			input:    "Some text {\"key\": \"value\"}",
			expected: "Some text {\"key\": \"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHealth(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/health", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := types.LLMConfig{
			Endpoint: server.URL,
		}
		client := NewClient(config)

		err := client.Health()
		assert.NoError(t, err)
	})

	t.Run("unhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		config := types.LLMConfig{
			Endpoint: server.URL,
		}
		client := NewClient(config)

		err := client.Health()
		assert.Error(t, err)
	})
}

func TestGetConfig(t *testing.T) {
	config := types.LLMConfig{
		Endpoint:    "http://localhost:8000",
		Model:       "test-model",
		MaxTokens:   1000,
		Temperature: 0.5,
	}

	client := NewClient(config)
	retrievedConfig := client.GetConfig()

	assert.Equal(t, config, retrievedConfig)
}
