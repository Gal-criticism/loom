/**
 * API Types
 * 共享的 API 类型定义
 */

package api

import "time"

// Message 聊天消息
type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"` // user | assistant | system | tool
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Session 会话信息
type Session struct {
	ID           string            `json:"id"`
	RuntimeType  string            `json:"runtime_type"`
	Status       string            `json:"status"`
	WorkingDir   string            `json:"working_dir"`
	StartedAt    time.Time         `json:"started_at"`
	LastActivity time.Time         `json:"last_activity"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// DaemonStatus Daemon 状态
type DaemonStatus struct {
	Version   string    `json:"version"`
	StartedAt time.Time `json:"started_at"`
	Uptime    string    `json:"uptime"`
	Sessions  int       `json:"sessions"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Sessions  int       `json:"sessions"`
}
