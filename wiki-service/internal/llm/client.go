package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/IBM/project-ai-services/wiki-service/pkg/types"
)

// Client handles communication with the LLM
type Client struct {
	config     types.LLMConfig
	httpClient *http.Client
}

// NewClient creates a new LLM client
func NewClient(config types.LLMConfig) *Client {
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// CompletionRequest represents a request to the LLM
type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionResponse represents a response from the LLM
type CompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Complete sends a completion request to the LLM
func (c *Client) Complete(systemPrompt, userPrompt string) (string, error) {
	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	return c.CompleteWithMessages(messages)
}

// CompleteWithMessages sends a completion request with custom messages
func (c *Client) CompleteWithMessages(messages []Message) (string, error) {
	req := CompletionRequest{
		Model:       c.config.Model,
		Messages:    messages,
		MaxTokens:   c.config.MaxTokens,
		Temperature: c.config.Temperature,
		Stream:      false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", c.config.Endpoint+"/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var completionResp CompletionResponse
	if err := json.Unmarshal(body, &completionResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(completionResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return completionResp.Choices[0].Message.Content, nil
}

// CompleteJSON sends a completion request and expects a JSON response
func (c *Client) CompleteJSON(systemPrompt, userPrompt string, result interface{}) error {
	content, err := c.Complete(systemPrompt, userPrompt)
	if err != nil {
		return err
	}

	// Try to extract JSON from markdown code blocks if present
	content = extractJSON(content)

	if err := json.Unmarshal([]byte(content), result); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w\nContent: %s", err, content)
	}

	return nil
}

// extractJSON extracts JSON from markdown code blocks
func extractJSON(content string) string {
	// Remove markdown code blocks if present
	if len(content) > 7 && content[:3] == "```" {
		// Find the end of the opening code block marker
		start := 0
		for i := 3; i < len(content); i++ {
			if content[i] == '\n' {
				start = i + 1
				break
			}
		}

		// Find the closing code block marker
		end := len(content)
		for i := len(content) - 1; i >= 3; i-- {
			if content[i-2:i+1] == "```" {
				end = i - 2
				break
			}
		}

		if start < end {
			content = content[start:end]
		}
	}

	return content
}

// Health checks if the LLM endpoint is healthy
func (c *Client) Health() error {
	resp, err := c.httpClient.Get(c.config.Endpoint + "/health")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// GetConfig returns the client configuration
func (c *Client) GetConfig() types.LLMConfig {
	return c.config
}
