/**
 * Runtime 接口定义
 * 定义 AI 运行时（Claude Code / OpenCode）的统一接口
 */

package runtime

import "context"

// Message 表示聊天消息
type Message struct {
	Role    string `json:"role"`    // user | assistant | system
	Content string `json:"content"` // 消息内容
}

// ToolCall 表示工具调用
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolResult 表示工具执行结果
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type string `json:"type"` // text | tool_call | tool_result | thinking | done | error

	// Type=text 时使用
	Text string `json:"text,omitempty"`

	// Type=tool_call 时使用
	ToolCall *ToolCall `json:"tool_call,omitempty"`

	// Type=tool_result 时使用
	ToolResult *ToolResult `json:"tool_result,omitempty"`

	// Type=thinking 时使用
	Thinking bool `json:"thinking,omitempty"`

	// Type=done 时使用
	Done bool `json:"done,omitempty"`

	// Type=error 时使用
	Error string `json:"error,omitempty"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	SessionID string    `json:"session_id"`
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
	Stream    bool      `json:"stream"`
}

// Tool 工具定义
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema Schema `json:"input_schema"`
}

// Schema JSON Schema 定义
type Schema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required"`
}

// Config 运行时配置
type Config struct {
	// CLI 路径，默认使用系统 PATH
	CLIPath string

	// 工作目录
	WorkingDir string

	// 环境变量
	EnvVars map[string]string

	// MCP 服务器配置
	MCPServers map[string]MCPServer

	// 允许的工具列表
	AllowedTools []string

	// 权限模式
	PermissionMode string // default | autoEdit | yolo
}

// MCPServer MCP 服务器配置
type MCPServer struct {
	Type    string `json:"type"` // http | stdio
	URL     string `json:"url,omitempty"`
	Command string `json:"command,omitempty"`
}

// Runtime AI 运行时接口
type Runtime interface {
	// Name 返回运行时名称
	Name() string

	// Chat 发送聊天请求，通过回调接收流式响应
	Chat(ctx context.Context, req ChatRequest, onEvent func(StreamEvent)) error

	// ListTools 列出可用工具
	ListTools() []Tool

	// ExecuteTool 执行特定工具
	ExecuteTool(ctx context.Context, name string, input map[string]interface{}) (string, error)

	// HealthCheck 健康检查
	HealthCheck() error
}

// Factory 创建 Runtime 的工厂函数
func Factory(runtimeType string, config Config) (Runtime, error) {
	switch runtimeType {
	case "claude", "claude-code":
		return NewClaudeRuntime(config)
	case "opencode", "open-code":
		return NewOpenCodeRuntime(config)
	default:
		return nil, &UnsupportedRuntimeError{Type: runtimeType}
	}
}

// UnsupportedRuntimeError 不支持的运行时类型错误
type UnsupportedRuntimeError struct {
	Type string
}

func (e *UnsupportedRuntimeError) Error() string {
	return "unsupported runtime type: " + e.Type
}
