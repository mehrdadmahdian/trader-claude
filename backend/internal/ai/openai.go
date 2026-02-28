package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const openaiDefaultModel = "gpt-4o-mini"

var openaiBaseURL = "https://api.openai.com/v1"

// OpenAIProvider implements AIProvider using OpenAI's Chat Completions API.
type OpenAIProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIProvider creates an OpenAI provider.
func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	if model == "" {
		model = openaiDefaultModel
	}
	return &OpenAIProvider{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *OpenAIProvider) Name() string { return "openai" }

func (o *OpenAIProvider) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	type oaiMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	oaiMessages := make([]oaiMsg, len(messages))
	for i, m := range messages {
		oaiMessages[i] = oaiMsg{Role: string(m.Role), Content: m.Content}
	}

	reqBody := map[string]interface{}{
		"model":       o.model,
		"messages":    oaiMessages,
		"temperature": 0.7,
		"max_tokens":  1024,
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiBaseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("openai: decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("openai: no choices returned")
	}

	content := result.Choices[0].Message.Content
	reply, suggestions := parseSuggestions(content)

	return &ChatResponse{
		Reply:              reply,
		SuggestedQuestions: suggestions,
	}, nil
}

func (o *OpenAIProvider) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openaiBaseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("openai: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("openai: test connection: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("openai: test failed with status %d", resp.StatusCode)
	}
	return nil
}

// parseSuggestions extracts suggested questions from the reply.
// The system prompt instructs the model to end with a JSON block:
//
//	```suggestions
//	["question1", "question2", "question3"]
//	```
func parseSuggestions(content string) (string, []string) {
	marker := "```suggestions"
	idx := strings.Index(content, marker)
	if idx == -1 {
		return content, defaultSuggestions()
	}

	reply := strings.TrimSpace(content[:idx])
	jsonPart := content[idx+len(marker):]
	endIdx := strings.Index(jsonPart, "```")
	if endIdx == -1 {
		return reply, defaultSuggestions()
	}
	jsonPart = strings.TrimSpace(jsonPart[:endIdx])

	var suggestions []string
	if err := json.Unmarshal([]byte(jsonPart), &suggestions); err != nil {
		return reply, defaultSuggestions()
	}
	if len(suggestions) > 3 {
		suggestions = suggestions[:3]
	}
	return reply, suggestions
}

func defaultSuggestions() []string {
	return []string{
		"Can you explain more?",
		"What are the risks?",
		"What should I do next?",
	}
}
