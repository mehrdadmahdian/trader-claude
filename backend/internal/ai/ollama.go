package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const ollamaDefaultModel = "llama3.2"

// OllamaProvider implements AIProvider using a local Ollama instance.
type OllamaProvider struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOllamaProvider creates an Ollama provider.
func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://ollama:11434"
	}
	if model == "" {
		model = ollamaDefaultModel
	}
	return &OllamaProvider{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (o *OllamaProvider) Name() string { return "ollama" }

func (o *OllamaProvider) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	type ollamaMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := make([]ollamaMsg, len(messages))
	for i, m := range messages {
		msgs[i] = ollamaMsg{Role: string(m.Role), Content: m.Content}
	}

	reqBody := map[string]interface{}{
		"model":    o.model,
		"messages": msgs,
		"stream":   false,
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ollama: decode: %w", err)
	}

	reply, suggestions := parseSuggestions(result.Message.Content)
	return &ChatResponse{
		Reply:              reply,
		SuggestedQuestions: suggestions,
	}, nil
}

func (o *OllamaProvider) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("ollama: create request: %w", err)
	}
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama: unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama: status %d", resp.StatusCode)
	}
	return nil
}
