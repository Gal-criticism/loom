package runtime

import (
    "context"
)

// OpenCodeRuntime implements Runtime for OpenCode
type OpenCodeRuntime struct{}

func NewOpenCodeRuntime() Runtime {
    return &OpenCodeRuntime{}
}

func (r *OpenCodeRuntime) Name() string {
    return "opencode"
}

func (r *OpenCodeRuntime) ListCapabilities() ([]string, []string) {
    // OpenCode 能力列表
    return []string{"Bash", "Read", "Edit", "Write", "Glob", "Grep"}, nil
}

func (r *OpenCodeRuntime) Chat(ctx context.Context, req ChatRequest, onResponse func(ChatResponse)) error {
    // OpenCode 实现
    onResponse(ChatResponse{
        Content: "OpenCode runtime not fully implemented yet",
        Done:    true,
    })
    return nil
}

func (r *OpenCodeRuntime) ExecuteTool(ctx context.Context, name string, input map[string]interface{}) (string, error) {
    // TODO: 实现 OpenCode 工具执行
    return "", nil
}
