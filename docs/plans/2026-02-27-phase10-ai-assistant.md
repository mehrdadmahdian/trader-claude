# Phase 10 — AI Assistant Chatbot — Atomic Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a context-aware AI chatbot that answers questions about the user's current view (chart analysis, backtest interpretation, portfolio advice). Supports OpenAI and Ollama (local) providers. Surfaces as a floating panel on every page with 3 suggested follow-up questions per reply.

**Architecture:** `internal/ai/` package with pluggable `AIProvider` interface. A dynamic system prompt is built from `page_context` sent by the frontend. The chat panel captures context from Zustand stores and sends it with each request. No server-side chat history — client passes full `messages[]` each time.

**Tech Stack:** Go 1.24 (Fiber v2, GORM), React 18 (React Query v5, Zustand, react-markdown, Tailwind, lucide-react). New npm dependency: `react-markdown`.

**Design doc:** `docs/plans/2026-02-27-phase10-ai-assistant-design.md`

---

## BACKEND TASKS

---

### Task B1: Create internal/ai/provider.go — interfaces and types

**Read first:** Nothing — standalone.

**Files to create:**
- `backend/internal/ai/provider.go`

---

**Step 1: Create the file**

```go
package ai

import "context"

// Role represents the role of a message in a chat.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ChatMessage is a single message in the conversation.
type ChatMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatResponse is the backend's response to a chat request.
type ChatResponse struct {
	Reply              string   `json:"reply"`
	SuggestedQuestions []string `json:"suggested_questions"`
}

// AIProvider is the interface that all AI providers must implement.
type AIProvider interface {
	Name() string
	Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error)
	TestConnection(ctx context.Context) error
}
```

---

**Step 2: Verify compile**

```bash
docker compose exec backend go build ./internal/ai/...
```

---

**Step 3: Commit**

```bash
git add backend/internal/ai/
git commit -m "feat(phase10): add AIProvider interface and ChatMessage types"
```

---

### Task B2: Create internal/ai/openai.go + tests

**Read first:**
- `backend/internal/ai/provider.go`

**Files to create:**
- `backend/internal/ai/openai.go`
- `backend/internal/ai/openai_test.go`

---

**Step 1: Create `openai.go`**

```go
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
		httpClient: &http.Client{},
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
	b, _ := json.Marshal(reqBody)

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
		return err
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
```

---

**Step 2: Create `openai_test.go`**

```go
package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIChat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("expected Authorization header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices":[{
				"message":{"content":"RSI divergence means...\n\n` + "```suggestions\n" + `[\"What about MACD?\",\"Best timeframe?\",\"How to confirm?\"]` + "\n```" + `"}
			}]
		}`))
	}))
	defer server.Close()

	old := openaiBaseURL
	openaiBaseURL = server.URL
	defer func() { openaiBaseURL = old }()

	provider := NewOpenAIProvider("test-key", "gpt-4o-mini")
	resp, err := provider.Chat(context.Background(), []ChatMessage{
		{Role: RoleUser, Content: "What is RSI divergence?"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Reply == "" {
		t.Error("expected non-empty reply")
	}
	if len(resp.SuggestedQuestions) != 3 {
		t.Errorf("expected 3 suggestions, got %d", len(resp.SuggestedQuestions))
	}
}

func TestOpenAIChat_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer server.Close()

	old := openaiBaseURL
	openaiBaseURL = server.URL
	defer func() { openaiBaseURL = old }()

	provider := NewOpenAIProvider("bad-key", "")
	_, err := provider.Chat(context.Background(), []ChatMessage{
		{Role: RoleUser, Content: "test"},
	})
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestParseSuggestions_WithBlock(t *testing.T) {
	content := "Here is my answer.\n\n```suggestions\n[\"Q1\",\"Q2\",\"Q3\"]\n```"
	reply, suggestions := parseSuggestions(content)
	if reply != "Here is my answer." {
		t.Errorf("unexpected reply: %s", reply)
	}
	if len(suggestions) != 3 {
		t.Errorf("expected 3, got %d", len(suggestions))
	}
}

func TestParseSuggestions_WithoutBlock(t *testing.T) {
	content := "Here is my answer without suggestions."
	reply, suggestions := parseSuggestions(content)
	if reply != content {
		t.Errorf("unexpected reply: %s", reply)
	}
	if len(suggestions) != 3 {
		t.Errorf("expected 3 defaults, got %d", len(suggestions))
	}
}

func TestOpenAITestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	old := openaiBaseURL
	openaiBaseURL = server.URL
	defer func() { openaiBaseURL = old }()

	provider := NewOpenAIProvider("test-key", "")
	err := provider.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

---

**Step 3: Run tests**

```bash
docker compose exec backend go test ./internal/ai/... -v
```

Expected: all tests PASS.

---

**Step 4: Commit**

```bash
git add backend/internal/ai/openai.go backend/internal/ai/openai_test.go
git commit -m "feat(phase10): add OpenAI provider with chat + test connection"
```

---

### Task B3: Create internal/ai/ollama.go + tests

**Read first:**
- `backend/internal/ai/provider.go`

**Files to create:**
- `backend/internal/ai/ollama.go`
- `backend/internal/ai/ollama_test.go`

---

**Step 1: Create `ollama.go`**

```go
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		httpClient: &http.Client{},
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
	b, _ := json.Marshal(reqBody)

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

	body, _ := io.ReadAll(resp.Body)
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
		return err
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
```

---

**Step 2: Create `ollama_test.go`**

```go
package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaChat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":{"content":"Ollama says hello"}}`))
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "llama3.2")
	resp, err := provider.Chat(context.Background(), []ChatMessage{
		{Role: RoleUser, Content: "Hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Reply != "Ollama says hello" {
		t.Errorf("unexpected reply: %s", resp.Reply)
	}
}

func TestOllamaTestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.URL, "")
	err := provider.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOllamaTestConnection_Failure(t *testing.T) {
	provider := NewOllamaProvider("http://localhost:1", "")
	err := provider.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
```

---

**Step 3: Run tests**

```bash
docker compose exec backend go test ./internal/ai/... -v
```

---

**Step 4: Commit**

```bash
git add backend/internal/ai/ollama.go backend/internal/ai/ollama_test.go
git commit -m "feat(phase10): add Ollama provider with chat + test connection"
```

---

### Task B4: Create internal/ai/prompt.go + tests

**Read first:** Nothing — standalone pure function.

**Files to create:**
- `backend/internal/ai/prompt.go`
- `backend/internal/ai/prompt_test.go`

---

**Step 1: Create `prompt.go`**

```go
package ai

import (
	"fmt"
	"strings"
)

// PageContext describes the user's current view for system prompt injection.
type PageContext struct {
	Page       string                 `json:"page"`
	Symbol     string                 `json:"symbol,omitempty"`
	Timeframe  string                 `json:"timeframe,omitempty"`
	Indicators []string               `json:"indicators,omitempty"`
	Metrics    map[string]interface{} `json:"metrics,omitempty"`
	Positions  []PositionSummary      `json:"positions,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

// PositionSummary is a lightweight position description for the AI context.
type PositionSummary struct {
	Symbol string  `json:"symbol"`
	PnLPct float64 `json:"pnl_pct"`
}

const baseSystemPrompt = `You are an AI trading assistant for the trader-claude platform. You help users understand their market data, backtest results, portfolio performance, and trading strategies. Be concise, accurate, and actionable. Always end your response with exactly 3 suggested follow-up questions formatted as:

` + "```suggestions" + `
["question 1", "question 2", "question 3"]
` + "```"

// BuildSystemPrompt creates a context-specific system prompt from the user's current page state.
func BuildSystemPrompt(ctx PageContext) string {
	var sb strings.Builder
	sb.WriteString(baseSystemPrompt)
	sb.WriteString("\n\n")

	switch ctx.Page {
	case "chart":
		sb.WriteString(fmt.Sprintf("The user is viewing a %s chart for %s.", ctx.Timeframe, ctx.Symbol))
		if len(ctx.Indicators) > 0 {
			sb.WriteString(fmt.Sprintf(" Active indicators: %s.", strings.Join(ctx.Indicators, ", ")))
		}
	case "backtest":
		sb.WriteString("The user is viewing backtest results.")
		if len(ctx.Metrics) > 0 {
			sb.WriteString(" Metrics:")
			for k, v := range ctx.Metrics {
				sb.WriteString(fmt.Sprintf(" %s=%v", k, v))
			}
			sb.WriteString(".")
		}
	case "portfolio":
		sb.WriteString("The user is viewing their portfolio.")
		if len(ctx.Positions) > 0 {
			sb.WriteString(fmt.Sprintf(" They have %d positions.", len(ctx.Positions)))
			for _, p := range ctx.Positions {
				sb.WriteString(fmt.Sprintf(" %s: %+.1f%%", p.Symbol, p.PnLPct))
			}
		}
	case "monitor":
		sb.WriteString("The user is viewing live market monitors.")
	case "alerts":
		sb.WriteString("The user is viewing price alerts.")
	case "news":
		sb.WriteString(fmt.Sprintf("The user is reading news for %s.", ctx.Symbol))
	default:
		sb.WriteString("The user is on the general dashboard.")
	}

	return sb.String()
}
```

---

**Step 2: Create `prompt_test.go`**

```go
package ai

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_Chart(t *testing.T) {
	prompt := BuildSystemPrompt(PageContext{
		Page:       "chart",
		Symbol:     "BTCUSDT",
		Timeframe:  "1h",
		Indicators: []string{"RSI(14)", "EMA(21)"},
	})
	if !strings.Contains(prompt, "BTCUSDT") {
		t.Error("expected symbol in prompt")
	}
	if !strings.Contains(prompt, "RSI(14)") {
		t.Error("expected indicator in prompt")
	}
}

func TestBuildSystemPrompt_Backtest(t *testing.T) {
	prompt := BuildSystemPrompt(PageContext{
		Page:    "backtest",
		Metrics: map[string]interface{}{"sharpe": 1.87, "return": "34.2%"},
	})
	if !strings.Contains(prompt, "backtest") {
		t.Error("expected backtest mention")
	}
	if !strings.Contains(prompt, "sharpe") {
		t.Error("expected metrics in prompt")
	}
}

func TestBuildSystemPrompt_Portfolio(t *testing.T) {
	prompt := BuildSystemPrompt(PageContext{
		Page: "portfolio",
		Positions: []PositionSummary{
			{Symbol: "BTC", PnLPct: 12.5},
			{Symbol: "ETH", PnLPct: -3.2},
		},
	})
	if !strings.Contains(prompt, "2 positions") {
		t.Error("expected position count")
	}
}

func TestBuildSystemPrompt_Default(t *testing.T) {
	prompt := BuildSystemPrompt(PageContext{Page: "unknown"})
	if !strings.Contains(prompt, "general dashboard") {
		t.Error("expected default fallback")
	}
}

func TestBuildSystemPrompt_AlwaysHasSuggestionInstruction(t *testing.T) {
	pages := []string{"chart", "backtest", "portfolio", "monitor", "alerts", "news", "unknown"}
	for _, page := range pages {
		prompt := BuildSystemPrompt(PageContext{Page: page})
		if !strings.Contains(prompt, "suggested follow-up questions") {
			t.Errorf("page %q: expected suggestion instruction in prompt", page)
		}
	}
}
```

---

**Step 3: Run tests**

```bash
docker compose exec backend go test ./internal/ai/... -v -run TestBuildSystemPrompt
```

---

**Step 4: Commit**

```bash
git add backend/internal/ai/prompt.go backend/internal/ai/prompt_test.go
git commit -m "feat(phase10): add dynamic system prompt builder for all page contexts"
```

---

### Task B5: Create api/ai_handler.go + wire routes

**Read first:**
- `backend/internal/ai/provider.go`
- `backend/internal/ai/prompt.go`
- `backend/internal/api/routes.go` (to see where to add routes)

**Files to create:**
- `backend/internal/api/ai_handler.go`

**Files to modify:**
- `backend/internal/api/routes.go`

---

**Step 1: Create `ai_handler.go`**

Handler struct holds `db *gorm.DB` and creates providers on-the-fly from settings.

Endpoints:
- `POST /api/v1/ai/chat` — main chat endpoint
- `GET /api/v1/settings/ai` — load AI settings
- `POST /api/v1/settings/ai` — save AI settings
- `POST /api/v1/settings/ai/test` — test provider connection

Chat handler logic:
1. Parse request body: `{messages, page_context, provider}`
2. Load AI settings from DB (api_key, model, ollama_url)
3. Build system prompt from page_context
4. Create provider instance based on request or default
5. Prepend system message to messages
6. Call provider.Chat()
7. Return ChatResponse

---

**Step 2: Wire routes**

Add to `routes.go`:

```go
// --- AI ---
aih := newAIHandler(db)
v1.Post("/ai/chat", aih.chat)
v1.Get("/settings/ai", aih.getAISettings)
v1.Post("/settings/ai", aih.saveAISettings)
v1.Post("/settings/ai/test", aih.testAIConnection)
```

---

**Step 3: Verify compile + run tests**

```bash
make backend-fmt
docker compose exec backend go build ./...
make backend-test
```

---

**Step 4: Commit**

```bash
git add backend/internal/api/ai_handler.go backend/internal/api/routes.go
git commit -m "feat(phase10): add AI chat endpoint and settings API"
```

---

## FRONTEND TASKS

---

### Task F1: Add AI types + api/ai.ts

**Files to modify/create:**
- `frontend/src/types/index.ts` — append AI types
- `frontend/src/api/ai.ts` — create API client

---

### Task F2: Create hooks/useAI.ts

React Query hooks:
- `useSendChat()` — mutation for POST /ai/chat
- `useAISettings()` — query for GET /settings/ai
- `useSaveAISettings()` — mutation
- `useTestAIConnection()` — mutation

---

### Task F3: Create components/ai/ChatPanel.tsx + AIButton.tsx

**ChatPanel.tsx:**
- Slide-up panel (~50% viewport height)
- Header: "AI Assistant" + context chip + close button
- Message list: user right-aligned, assistant left-aligned
- `react-markdown` for rendering assistant messages
- 3 suggested question chips (clickable → fills input and sends)
- Text input + Send button (Enter to send, Shift+Enter newline)
- Typing indicator (bouncing dots while loading)
- Context captured from Zustand stores on panel open

**AIButton.tsx:**
- Fixed position bottom-right (above SignalToast)
- Sparkle icon (from lucide-react)
- Click toggles ChatPanel open/close

---

### Task F4: Wire AIButton into Layout.tsx

**Files to modify:**
- `frontend/src/components/layout/Layout.tsx`

Add `<AIButton />` below `<SignalToast />`.

---

### Task F5: Add AI section to Settings page

**Files to modify:**
- `frontend/src/pages/Settings.tsx`

Add "AI Assistant" section with:
- Provider selector (OpenAI / Ollama dropdown)
- API key input (password type, with indicator)
- Ollama URL input (shown only when Ollama selected)
- Model input
- "Test Connection" button + result toast

---

**Step: Add `react-markdown` dependency**

```bash
docker compose exec frontend npm install react-markdown
```

---

**Final Verification:**

```bash
make backend-test
make frontend-lint
make frontend-test
```

---

## Summary

| Task | Files | Description |
|---|---|---|
| B1 | ai/provider.go | AIProvider interface + types |
| B2 | ai/openai.go | OpenAI Chat Completions provider + tests |
| B3 | ai/ollama.go | Ollama local provider + tests |
| B4 | ai/prompt.go | Dynamic system prompt builder + tests |
| B5 | api/ai_handler.go, routes.go | Chat endpoint + AI settings API |
| F1 | types/index.ts, api/ai.ts | AI types + API client |
| F2 | hooks/useAI.ts | React Query hooks |
| F3 | components/ai/ChatPanel.tsx, AIButton.tsx | Chat UI + floating button |
| F4 | layout/Layout.tsx | Wire AI button into layout |
| F5 | pages/Settings.tsx | AI configuration section |
