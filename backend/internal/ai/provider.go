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
