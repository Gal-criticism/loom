/**
 * Claude Code Runtime 实现
 * 封装 Claude Code CLI，提供统一的 Runtime 接口
 */

package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ClaudeRuntime Claude Code 运行时实现
type ClaudeRuntime struct {
	config  Config
	cliPath string
	tools   []Tool
}

// NewClaudeRuntime 创建 Claude Runtime 实例
func NewClaudeRuntime(config Config) (*ClaudeRuntime, error) {
	// 查找 Claude CLI
	cliPath := config.CLIPath
	if cliPath == "" {
		var err error
		cliPath, err = exec.LookPath("claude")
		if err != nil {
			return nil, fmt.Errorf("claude CLI not found in PATH: %w", err)
		}
	}

	rt := &ClaudeRuntime{
		config:  config,
		cliPath: cliPath,
		tools:   defaultClaudeTools(),
	}

	return rt, nil
}

// Name 返回运行时名称
func (r *ClaudeRuntime) Name() string {
	return "claude-code"
}

// ListTools 列出可用工具
func (r *ClaudeRuntime) ListTools() []Tool {
	return r.tools
}

// Chat 发送聊天请求，接收流式响应
func (r *ClaudeRuntime) Chat(ctx context.Context, req ChatRequest, onEvent func(StreamEvent)) error {
	// 构建 Claude CLI 参数
	args := []string{"--print"}

	// 添加系统提示词
	args = append(args, "--system-prompt", r.buildSystemPrompt(req.SessionID))

	// 添加 MCP 服务器配置
	if len(r.config.MCPServers) > 0 {
		mcpConfig := map[string]interface{}{
			"mcpServers": r.config.MCPServers,
		}
		mcpJSON, _ := json.Marshal(mcpConfig)
		args = append(args, "--mcp-config", string(mcpJSON))
	}

	// 添加允许的工具
	if len(r.config.AllowedTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(r.config.AllowedTools, ","))
	}

	// 添加权限模式
	if r.config.PermissionMode == "yolo" {
		args = append(args, "--dangerously-skip-permissions")
	}

	// 构建消息历史
	prompt := r.buildPrompt(req.Messages)
	args = append(args, prompt)

	// 启动 Claude 进程
	cmd := exec.CommandContext(ctx, r.cliPath, args...)
	cmd.Dir = r.config.WorkingDir

	// 设置环境变量
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
		return fmt.Errorf("failed to start claude: %w", err)
	}

	// 使用 scanner 读取流式输出
	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)

	// 创建错误通道
	errChan := make(chan error, 1)
	doneChan := make(chan bool, 1)

	// 解析 stdout
	go func() {
		defer stdout.Close()

		for scanner.Scan() {
			line := scanner.Text()

			// 解析工具调用
			if strings.HasPrefix(line, "$ mcp__") {
				toolCall, err := r.parseToolCall(line)
				if err == nil {
					onEvent(StreamEvent{
						Type:     "tool_call",
						ToolCall: toolCall,
					})

					// 执行工具调用
					result, err := r.ExecuteTool(ctx, toolCall.Name, toolCall.Arguments)

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
				}
			} else {
				// 普通文本输出
				onEvent(StreamEvent{
					Type: "text",
					Text: line,
				})
			}
		}

		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("scanner error: %w", err)
		}
	}()

	// 监控 stderr 获取思考状态
	go func() {
		scanner := bufio.NewScanner(stderr)
		thinking := false

		for scanner.Scan() {
			line := scanner.Text()

			// Claude 在 stderr 输出思考状态
			// 这是一个简化的检测逻辑
			if strings.Contains(line, "Thinking") || strings.Contains(line, "Fetching") {
				if !thinking {
					thinking = true
					onEvent(StreamEvent{
						Type:     "thinking",
						Thinking: true,
					})
				}
			} else if thinking {
				// 思考结束
				thinking = false
				onEvent(StreamEvent{
					Type:     "thinking",
					Thinking: false,
				})
			}
		}
	}()

	// 等待进程结束
	go func() {
		if err := cmd.Wait(); err != nil {
			if ctx.Err() == context.Canceled {
				doneChan <- true
				return
			}
			errChan <- fmt.Errorf("claude process error: %w", err)
			return
		}
		doneChan <- true
	}()

	// 等待完成或错误
	select {
	case <-doneChan:
		onEvent(StreamEvent{Type: "done", Done: true})
		return nil
	case err := <-errChan:
		onEvent(StreamEvent{
			Type:  "error",
			Error: err.Error(),
		})
		return err
	}
}

// ExecuteTool 执行特定工具
func (r *ClaudeRuntime) ExecuteTool(ctx context.Context, name string, input map[string]interface{}) (string, error) {
	// 导入工具包执行
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
func (r *ClaudeRuntime) HealthCheck() error {
	cmd := exec.Command(r.cliPath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude health check failed: %w", err)
	}
	return nil
}

// 辅助方法

func (r *ClaudeRuntime) buildSystemPrompt(sessionID string) string {
	return fmt.Sprintf(`You are Claude Code, an AI assistant integrated into Loom.
Current session: %s
Use tools when appropriate. Always confirm destructive actions.`, sessionID)
}

func (r *ClaudeRuntime) buildPrompt(messages []Message) string {
	var parts []string
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			parts = append(parts, fmt.Sprintf("User: %s", msg.Content))
		case "assistant":
			parts = append(parts, fmt.Sprintf("Assistant: %s", msg.Content))
		case "system":
			parts = append(parts, fmt.Sprintf("System: %s", msg.Content))
		}
	}
	return strings.Join(parts, "\n\n")
}

func (r *ClaudeRuntime) buildEnv() []string {
	// 基础环境变量
	env := []string{}

	// 添加自定义环境变量
	for k, v := range r.config.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

func (r *ClaudeRuntime) parseToolCall(line string) (*ToolCall, error) {
	// 解析 $ mcp__<server>__<tool> <json>
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid tool call format")
	}

	toolParts := strings.Split(parts[1], "__")
	if len(toolParts) < 2 {
		return nil, fmt.Errorf("invalid tool name format")
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(parts[2]), &args); err != nil {
		return nil, err
	}

	return &ToolCall{
		ID:        generateToolCallID(),
		Name:      toolParts[len(toolParts)-1],
		Arguments: args,
	}, nil
}

// 工具执行实现

func (r *ClaudeRuntime) executeBashTool(ctx context.Context, input map[string]interface{}) (string, error) {
	command, ok := input["command"].(string)
	if !ok {
		return "", fmt.Errorf("missing command argument")
	}

	timeout := 60 // 默认 60 秒
	if t, ok := input["timeout"].(float64); ok {
		timeout = int(t)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = r.config.WorkingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

func (r *ClaudeRuntime) executeReadTool(ctx context.Context, input map[string]interface{}) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("missing file_path argument")
	}

	// 安全检查：确保文件在工作目录内
	// TODO: 实现路径安全检查

	cmd := exec.CommandContext(ctx, "cat", filePath)
	cmd.Dir = r.config.WorkingDir

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(output), nil
}

func (r *ClaudeRuntime) executeWriteTool(ctx context.Context, input map[string]interface{}) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("missing file_path argument")
	}

	content, ok := input["content"].(string)
	if !ok {
		return "", fmt.Errorf("missing content argument")
	}

	// 使用 echo 写入文件
	cmd := exec.CommandContext(ctx, "sh", "-c",
		fmt.Sprintf("echo '%s' > %s", escapeSingleQuotes(content), filePath))
	cmd.Dir = r.config.WorkingDir

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote to %s", filePath), nil
}

func (r *ClaudeRuntime) executeEditTool(ctx context.Context, input map[string]interface{}) (string, error) {
	// 简化实现：使用 sed 替换
	filePath, _ := input["file_path"].(string)
	oldString, _ := input["old_string"].(string)
	newString, _ := input["new_string"].(string)

	if filePath == "" || oldString == "" {
		return "", fmt.Errorf("missing required arguments")
	}

	// 使用 sed 进行替换
	cmd := exec.CommandContext(ctx, "sed", "-i",
		fmt.Sprintf("s/%s/%s/g", escapeRegex(oldString), escapeRegex(newString)),
		filePath)
	cmd.Dir = r.config.WorkingDir

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to edit file: %w", err)
	}

	return fmt.Sprintf("Successfully edited %s", filePath), nil
}

func (r *ClaudeRuntime) executeGlobTool(ctx context.Context, input map[string]interface{}) (string, error) {
	pattern, ok := input["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("missing pattern argument")
	}

	// 使用 find 命令
	cmd := exec.CommandContext(ctx, "find", ".", "-name", pattern, "-type", "f")
	cmd.Dir = r.config.WorkingDir

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("glob failed: %w", err)
	}

	return string(output), nil
}

func (r *ClaudeRuntime) executeGrepTool(ctx context.Context, input map[string]interface{}) (string, error) {
	pattern, ok := input["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("missing pattern argument")
	}

	path, _ := input["path"].(string)
	if path == "" {
		path = "."
	}

	// 使用 grep 命令
	cmd := exec.CommandContext(ctx, "grep", "-r", "-n", pattern, path)
	cmd.Dir = r.config.WorkingDir

	output, err := cmd.Output()
	if err != nil {
		// grep 没有找到匹配时返回 exit code 1，这不是错误
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "No matches found", nil
		}
		return "", fmt.Errorf("grep failed: %w", err)
	}

	return string(output), nil
}

// 辅助函数

func defaultClaudeTools() []Tool {
	return []Tool{
		{
			Name:        "Bash",
			Description: "Execute bash commands in the working directory",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]interface{}{
					"command": map[string]string{"type": "string", "description": "The bash command to execute"},
					"timeout": map[string]string{"type": "number", "description": "Timeout in seconds (default: 60)"},
				},
				Required: []string{"command"},
			},
		},
		{
			Name:        "Read",
			Description: "Read the contents of a file",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]interface{}{
					"file_path": map[string]string{"type": "string", "description": "Path to the file"},
					"offset":    map[string]string{"type": "number", "description": "Line offset to start reading"},
					"limit":     map[string]string{"type": "number", "description": "Number of lines to read"},
				},
				Required: []string{"file_path"},
			},
		},
		{
			Name:        "Write",
			Description: "Write content to a file",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]interface{}{
					"file_path": map[string]string{"type": "string", "description": "Path to the file"},
					"content":   map[string]string{"type": "string", "description": "Content to write"},
				},
				Required: []string{"file_path", "content"},
			},
		},
		{
			Name:        "Edit",
			Description: "Edit a file by replacing text",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]interface{}{
					"file_path":  map[string]string{"type": "string", "description": "Path to the file"},
					"old_string": map[string]string{"type": "string", "description": "Text to replace"},
					"new_string": map[string]string{"type": "string", "description": "Replacement text"},
				},
				Required: []string{"file_path", "old_string", "new_string"},
			},
		},
		{
			Name:        "Glob",
			Description: "Find files matching a pattern",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]interface{}{
					"pattern": map[string]string{"type": "string", "description": "File pattern (e.g., '*.go')"},
					"path":    map[string]string{"type": "string", "description": "Directory to search"},
				},
				Required: []string{"pattern"},
			},
		},
		{
			Name:        "Grep",
			Description: "Search for patterns in files",
			InputSchema: Schema{
				Type: "object",
				Properties: map[string]interface{}{
					"pattern": map[string]string{"type": "string", "description": "Search pattern"},
					"path":    map[string]string{"type": "string", "description": "Directory to search"},
				},
				Required: []string{"pattern"},
			},
		},
	}
}

func generateToolCallID() string {
	return fmt.Sprintf("call_%d", time.Now().UnixNano())
}

func escapeSingleQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "'\"'\"'")
}

func escapeRegex(s string) string {
	// 简化的正则转义
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"/", "\\/",
		"&", "\\&",
	)
	return replacer.Replace(s)
}
