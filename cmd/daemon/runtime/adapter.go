package runtime

import "context"

// Message represents a chat message
type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

// Tool represents a tool call
type Tool struct {
    Name      string                 `json:"name"`
    Input     map[string]interface{} `json:"input"`
}

// ChatRequest represents a chat request
type ChatRequest struct {
    Messages []Message `json:"messages"`
    Tools    []Tool    `json:"tools,omitempty"`
    Stream   bool      `json:"stream"`
}

// ChatResponse represents a chat response
type ChatResponse struct {
    Content  string `json:"content"`
    ToolCall *Tool  `json:"tool_call,omitempty"`
    Done     bool   `json:"done"`
}

// Runtime is the interface for AI runtimes
type Runtime interface {
    Chat(ctx context.Context, req ChatRequest, onResponse func(ChatResponse)) error
    ExecuteTool(ctx context.Context, name string, input map[string]interface{}) (string, error)
    ListCapabilities() ([]string, []string)
    Name() string
}

// NewRuntime creates a new runtime based on the type
func NewRuntime(runtimeType string) (Runtime, error) {
    switch runtimeType {
    case "claude-code", "claude":
        return NewClaudeRuntime(), nil
    case "opencode", "open-code":
        return NewOpenCodeRuntime(), nil
    default:
        return nil, fmt.Errorf("unknown runtime: %s", runtimeType)
    }
}
