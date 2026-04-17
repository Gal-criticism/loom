/**
 * Message Formatter
 * 消息格式转换器，在 Daemon 和 Backend 之间转换消息格式
 */

package messaging

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/loom/daemon/internal/runtime"
)

// Formatter 消息格式转换器
type Formatter struct{}

// NewFormatter 创建格式转换器
func NewFormatter() *Formatter {
	return &Formatter{}
}

// ToBackendMessage 将 Runtime 事件转换为 Backend 消息格式
func (f *Formatter) ToBackendMessage(sessionID string, event runtime.StreamEvent) ([]byte, error) {
	msg := BackendMessage{
		Type:      f.mapEventType(event.Type),
		SessionID: sessionID,
		Timestamp: time.Now().Unix(),
	}

	// 根据事件类型设置数据
	switch event.Type {
	case "text":
		msg.Data = TextMessageData{Content: event.Text}

	case "thinking":
		msg.Data = ThinkingMessageData{Thinking: event.Thinking}

	case "tool_call":
		if event.ToolCall != nil {
			msg.Data = ToolCallMessageData{
				ToolName:  event.ToolCall.Name,
				ToolInput: event.ToolCall.Arguments,
			}
		}

	case "tool_result":
		if event.ToolResult != nil {
			msg.Data = ToolResultMessageData{
				Output: event.ToolResult.Output,
				Error:  event.ToolResult.Error,
			}
		}

	case "error":
		msg.Data = ErrorMessageData{Error: event.Error}

	case "done":
		msg.Data = DoneMessageData{Done: true}
	}

	return json.Marshal(msg)
}

// FromBackendMessage 将 Backend 消息转换为 Runtime 请求
func (f *Formatter) FromBackendMessage(data []byte) (*BackendMessage, error) {
	var msg BackendMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backend message: %w", err)
	}
	return &msg, nil
}

// mapEventType 映射事件类型
func (f *Formatter) mapEventType(eventType string) string {
	switch eventType {
	case "text":
		return "chat:response"
	case "thinking":
		return "chat:thinking"
	case "tool_call":
		return "chat:tool_call"
	case "tool_result":
		return "chat:tool_result"
	case "error":
		return "chat:error"
	case "done":
		return "chat:done"
	default:
		return "chat:unknown"
	}
}

// BackendMessage Backend 消息格式
type BackendMessage struct {
	Type      string      `json:"type"`
	SessionID string      `json:"session_id"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// TextMessageData 文本消息数据
type TextMessageData struct {
	Content string `json:"content"`
}

// ThinkingMessageData 思考状态数据
type ThinkingMessageData struct {
	Thinking bool `json:"thinking"`
}

// ToolCallMessageData 工具调用数据
type ToolCallMessageData struct {
	ToolName  string                 `json:"tool_name"`
	ToolInput map[string]interface{} `json:"tool_input"`
}

// ToolResultMessageData 工具结果数据
type ToolResultMessageData struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// ErrorMessageData 错误数据
type ErrorMessageData struct {
	Error string `json:"error"`
}

// DoneMessageData 完成数据
type DoneMessageData struct {
	Done bool `json:"done"`
}
