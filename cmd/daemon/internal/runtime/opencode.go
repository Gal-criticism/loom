/**
 * OpenCode Runtime 实现
 * 封装 OpenCode CLI，提供统一的 Runtime 接口
 */

package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// OpenCodeRuntime OpenCode 运行时实现
type OpenCodeRuntime struct {
	config  Config
	cliPath string
	tools   []Tool
}

// NewOpenCodeRuntime 创建 OpenCode Runtime 实例
func NewOpenCodeRuntime(config Config) (*OpenCodeRuntime, error) {
	// 查找 OpenCode CLI
	cliPath := config.CLIPath
	if cliPath == "" {
		var err error
		cliPath, err = exec.LookPath("opencode")
		if err != nil {
			// 尝试备选路径
			cliPath, err = exec.LookPath("open-code")
			if err != nil {
				return nil, fmt.Errorf("opencode CLI not found in PATH: %w", err)
			}
		}
	}

	rt := &OpenCodeRuntime{
		config:  config,
		cliPath: cliPath,
		tools:   defaultOpenCodeTools(),
	}

	return rt, nil
}

// Name 返回运行时名称
func (r *OpenCodeRuntime) Name() string {
	return "opencode"
}

// ListTools 列出可用工具
func (r *OpenCodeRuntime) ListTools() []Tool {
	return r.tools
}

// Chat 发送聊天请求，接收流式响应
func (r *OpenCodeRuntime) Chat(ctx context.Context, req ChatRequest, onEvent func(StreamEvent)) error {
	// OpenCode 使用不同的参数格式
	// 这里是一个适配层，根据实际 OpenCode CLI 调整

	args := []string{"chat", "--stream"}

	// 添加工作目录
	if r.config.WorkingDir != "" {
		args = append(args, "--cwd", r.config.WorkingDir)
	}

	// 构建消息历史为 JSON
	messagesJSON, err := json.Marshal(req.Messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}
	args = append(args, "--messages", string(messagesJSON))

	// 如果有工具，添加工具配置
	if len(req.Tools) > 0 {
		toolsJSON, _ := json.Marshal(req.Tools)
		args = append(args, "--tools", string(toolsJSON))
	}

	// 启动 OpenCode 进程
	cmd := exec.CommandContext(ctx, r.cliPath, args...)
	cmd.Dir = r.config.WorkingDir
	cmd.Env = r.buildEnv()

	// 获取 stdout 和 stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start opencode: %w", err)
	}

	// 解析流式输出
	// OpenCode 的流式格式假设为 JSON Lines:
	// {"type": "text", "content": "..."}
	// {"type": "tool_call", "tool": "...", "args": {...}}

	go func() {
		defer stdout.Close()
		scanner := bufio.NewScanner(stdout)

		for scanner.Scan() {
			line := scanner.Text()

			var event map[string]interface{}
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				// 非 JSON 行，当作纯文本
				onEvent(StreamEvent{
					Type: "text",
					Text: line,
				})
				continue
			}

			eventType, _ := event["type"].(string)

			switch eventType {
			case "text":
				content, _ := event["content"].(string)
				onEvent(StreamEvent{
					Type: "text",
					Text: content,
				})

			case "tool_call":
				toolName, _ := event["tool"].(string)
				toolArgs, _ := event["args"].(map[string]interface{})

				toolCall := &ToolCall{
					ID:        generateToolCallID(),
					Name:      toolName,
					Arguments: toolArgs,
				}

				onEvent(StreamEvent{
					Type:     "tool_call",
					ToolCall: toolCall,
				})

				// 执行工具
				result, err := r.ExecuteTool(ctx, toolName, toolArgs)
				var toolResult ToolResult
				if err != nil {
					toolResult = ToolResult{
						ToolCallID: toolCall.ID,
						Error:      err.Error(),
					}
				} else {
					toolResult = ToolResult{
						ToolCallID: toolCall.ID,
						Output:     result,
					}
				}

				onEvent(StreamEvent{
					Type:       "tool_result",
					ToolResult: &toolResult,
				})

			case "thinking":
				thinking, _ := event["thinking"].(bool)
				onEvent(StreamEvent{
					Type:     "thinking",
					Thinking: thinking,
				})

			case "error":
				errMsg, _ := event["error"].(string)
				onEvent(StreamEvent{
					Type:  "error",
					Error: errMsg,
				})
			}
		}
	}()

	// 监控 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// OpenCode 的 stderr 输出可以记录或忽略
			// 这里简单忽略
		}
	}()

	// 等待进程结束
	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.Canceled {
			onEvent(StreamEvent{Type: "done", Done: true})
			return nil
		}
		onEvent(StreamEvent{
			Type:  "error",
			Error: err.Error(),
		})
		return fmt.Errorf("opencode process error: %w", err)
	}

	onEvent(StreamEvent{Type: "done", Done: true})
	return nil
}

// ExecuteTool 执行特定工具
func (r *OpenCodeRuntime) ExecuteTool(ctx context.Context, name string, input map[string]interface{}) (string, error) {
	// OpenCode 的工具执行与 Claude 相同
	// 复用相同的工具实现

	switch name {
	case "Bash":
		return r.executeBashTool(ctx, input)
	case "Read":
		return r.executeReadTool(ctx, input)
	case "Write":
		return r.executeWriteTool(ctx, input)
	case "Edit":
		return r.executeEditTool(ctx, input)
	case "Glob":
		return r.executeGlobTool(ctx, input)
	case "Grep":
		return r.executeGrepTool(ctx, input)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// HealthCheck 健康检查
func (r *OpenCodeRuntime) HealthCheck() error {
	cmd := exec.Command(r.cliPath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("opencode health check failed: %w", err)
	}
	return nil
}

// 辅助方法

func (r *OpenCodeRuntime) buildEnv() []string {
	env := []string{}

	for k, v := range r.config.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

// 工具执行实现（与 Claude 相同）

func (r *OpenCodeRuntime) executeBashTool(ctx context.Context, input map[string]interface{}) (string, error) {
	command, ok := input["command"].(string)
	if !ok {
		return "", fmt.Errorf("missing command argument")
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = r.config.WorkingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

func (r *OpenCodeRuntime) executeReadTool(ctx context.Context, input map[string]interface{}) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("missing file_path argument")
	}

	cmd := exec.CommandContext(ctx, "cat", filePath)
	cmd.Dir = r.config.WorkingDir

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(output), nil
}

func (r *OpenCodeRuntime) executeWriteTool(ctx context.Context, input map[string]interface{}) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("missing file_path argument")
	}

	content, ok := input["content"].(string)
	if !ok {
		return "", fmt.Errorf("missing content argument")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c",
		fmt.Sprintf("echo '%s' > %s", escapeSingleQuotes(content), filePath))
	cmd.Dir = r.config.WorkingDir

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote to %s", filePath), nil
}

func (r *OpenCodeRuntime) executeEditTool(ctx context.Context, input map[string]interface{}) (string, error) {
	filePath, _ := input["file_path"].(string)
	oldString, _ := input["old_string"].(string)
	newString, _ := input["new_string"].(string)

	if filePath == "" || oldString == "" {
		return "", fmt.Errorf("missing required arguments")
	}

	cmd := exec.CommandContext(ctx, "sed", "-i",
		fmt.Sprintf("s/%s/%s/g", escapeRegex(oldString), escapeRegex(newString)),
		filePath)
	cmd.Dir = r.config.WorkingDir

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to edit file: %w", err)
	}

	return fmt.Sprintf("Successfully edited %s", filePath), nil
}

func (r *OpenCodeRuntime) executeGlobTool(ctx context.Context, input map[string]interface{}) (string, error) {
	pattern, ok := input["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("missing pattern argument")
	}

	cmd := exec.CommandContext(ctx, "find", ".", "-name", pattern, "-type", "f")
	cmd.Dir = r.config.WorkingDir

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("glob failed: %w", err)
	}

	return string(output), nil
}

func (r *OpenCodeRuntime) executeGrepTool(ctx context.Context, input map[string]interface{}) (string, error) {
	pattern, ok := input["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("missing pattern argument")
	}

	path, _ := input["path"].(string)
	if path == "" {
		path = "."
	}

	cmd := exec.CommandContext(ctx, "grep", "-r", "-n", pattern, path)
	cmd.Dir = r.config.WorkingDir

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "No matches found", nil
		}
		return "", fmt.Errorf("grep failed: %w", err)
	}

	return string(output), nil
}

func defaultOpenCodeTools() []Tool {
	// OpenCode 通常使用与 Claude 相同的工具集
	return defaultClaudeTools()
}
