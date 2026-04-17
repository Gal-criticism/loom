/**
 * WebSocket Client
 * Centrifugo WebSocket 客户端实现 (使用 centrifuge-go)
 */

package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/centrifugal/centrifuge-go"
	"github.com/loom/daemon/internal/runtime"
	"github.com/loom/daemon/internal/session"
)

// MessageHandler 消息处理函数类型
type MessageHandler func(payload []byte) error

// Client WebSocket 客户端
type Client struct {
	centrifugoURL  string
	deviceID       string
	apiKey         string
	sessionManager *session.Manager

	client    *centrifuge.Client
	connected bool
	mu        sync.RWMutex

	// 订阅的频道
	subscriptions map[string]*centrifuge.Subscription

	// 消息处理器
	handlers map[string]MessageHandler

	// 重连配置
	maxRetries    int
	retryInterval time.Duration
	reconnecting  bool

	// 停止信号
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewClient 创建 WebSocket 客户端
func NewClient(centrifugoURL, deviceID string, sessionManager *session.Manager) *Client {
	return &Client{
		centrifugoURL:  centrifugoURL,
		deviceID:       deviceID,
		sessionManager: sessionManager,
		handlers:       make(map[string]MessageHandler),
		subscriptions:  make(map[string]*centrifuge.Subscription),
		maxRetries:     10,
		retryInterval:  5 * time.Second,
		stopChan:       make(chan struct{}),
	}
}

// RegisterHandler 注册消息处理器
func (c *Client) RegisterHandler(messageType string, handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[messageType] = handler
}

// Connect 连接到 Centrifugo
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	// 创建 Centrifugo 客户端配置
	config := centrifuge.DefaultConfig()
	config.Name = "loom-daemon"
	config.Version = "0.1.0"

	// 创建客户端
	client := centrifuge.New(c.centrifugoURL, config)

	// 设置事件处理器
	client.OnConnecting(func(e centrifuge.ConnectingEvent) {
		log.Printf("[WS] Connecting: %d, %s", e.Code, e.Reason)
	})

	client.OnConnected(func(e centrifuge.ConnectedEvent) {
		log.Printf("[WS] Connected with client ID: %s", e.ClientID)
		c.mu.Lock()
		c.connected = true
		c.mu.Unlock()
	})

	client.OnDisconnected(func(e centrifuge.DisconnectedEvent) {
		log.Printf("[WS] Disconnected: %d, %s", e.Code, e.Reason)
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()

		// 触发重连
		go c.reconnect()
	})

	client.OnError(func(e centrifuge.ErrorEvent) {
		log.Printf("[WS] Error: %v", e.Error)
	})

	client.OnMessage(func(e centrifuge.MessageEvent) {
		c.handleMessage(e.Data)
	})

	// 连接到服务器
	err := client.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to Centrifugo: %w", err)
	}

	c.client = client

	// 订阅 daemon 频道 (接收后端命令)
	daemonChannel := fmt.Sprintf("daemon:%s", c.deviceID)
	if err := c.subscribe(daemonChannel); err != nil {
		log.Printf("[WS] Warning: failed to subscribe to %s: %v", daemonChannel, err)
	}

	// 订阅用户响应频道
	userChannel := fmt.Sprintf("user:%s", c.deviceID)
	if err := c.subscribe(userChannel); err != nil {
		log.Printf("[WS] Warning: failed to subscribe to %s: %v", userChannel, err)
	}

	log.Printf("[WS] Connected to Centrifugo at %s", c.centrifugoURL)

	// 启动心跳
	c.wg.Add(1)
	go c.heartbeatLoop()

	return nil
}

// subscribe 订阅频道
func (c *Client) subscribe(channel string) error {
	sub := c.client.NewSubscription(channel)

	sub.OnSubscribing(func(e centrifuge.SubscribingEvent) {
		log.Printf("[WS] Subscribing to %s: %d", channel, e.Code)
	})

	sub.OnSubscribed(func(e centrifuge.SubscribedEvent) {
		log.Printf("[WS] Subscribed to %s", channel)
	})

	sub.OnUnsubscribed(func(e centrifuge.UnsubscribedEvent) {
		log.Printf("[WS] Unsubscribed from %s: %d", channel, e.Code)
	})

	sub.OnError(func(e centrifuge.SubscriptionErrorEvent) {
		log.Printf("[WS] Subscription error for %s: %v", channel, e.Error)
	})

	sub.OnPublication(func(e centrifuge.PublicationEvent) {
		c.handlePublication(channel, e.Data)
	})

	if err := sub.Subscribe(); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", channel, err)
	}

	c.subscriptions[channel] = sub
	return nil
}

// handlePublication 处理频道消息
func (c *Client) handlePublication(channel string, data []byte) {
	log.Printf("[WS] Received message on channel %s", channel)

	// 解析消息类型
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("[WS] Failed to unmarshal message: %v", err)
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		log.Printf("[WS] Message without type field")
		return
	}

	// 查找处理器
	c.mu.RLock()
	handler, exists := c.handlers[msgType]
	c.mu.RUnlock()

	if !exists {
		log.Printf("[WS] No handler for message type: %s", msgType)
		return
	}

	// 调用处理器
	if err := handler(data); err != nil {
		log.Printf("[WS] Handler error for %s: %v", msgType, err)
	}
}

// handleMessage 处理直接消息
func (c *Client) handleMessage(data []byte) {
	log.Printf("[WS] Received direct message")
	c.handlePublication("direct", data)
}

// Disconnect 断开连接
func (c *Client) Disconnect() {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return
	}
	c.connected = false
	c.mu.Unlock()

	// 发送停止信号
	close(c.stopChan)

	// 取消所有订阅
	for _, sub := range c.subscriptions {
		sub.Unsubscribe()
	}

	// 断开连接
	if c.client != nil {
		c.client.Disconnect()
	}

	// 等待所有 goroutine 退出
	c.wg.Wait()

	log.Println("[WS] Disconnected from Centrifugo")
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetDeviceID 获取设备 ID
func (c *Client) GetDeviceID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.deviceID
}

// SendToBackend 发送消息到 Backend
func (c *Client) SendToBackend(channel string, message interface{}) error {
	c.mu.RLock()
	client := c.client
	connected := c.connected
	c.mu.RUnlock()

	if !connected || client == nil {
		return fmt.Errorf("not connected to Centrifugo")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 使用 Publish 发送消息到频道
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.Publish(ctx, channel, data)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// SendError 发送错误响应
func (c *Client) SendError(channel string, err error) {
	msg := map[string]interface{}{
		"type":      "error",
		"error":     err.Error(),
		"timestamp": time.Now().Unix(),
	}
	if err := c.SendToBackend(channel, msg); err != nil {
		log.Printf("[WS] Failed to send error: %v", err)
	}
}

// heartbeatLoop 心跳循环
func (c *Client) heartbeatLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			if c.IsConnected() {
				c.sendHeartbeat()
			}
		}
	}
}

// sendHeartbeat 发送心跳
func (c *Client) sendHeartbeat() {
	heartbeat := map[string]interface{}{
		"type":      "heartbeat",
		"device_id": c.deviceID,
		"timestamp": time.Now().Unix(),
	}

	channel := fmt.Sprintf("daemon:%s", c.deviceID)
	if err := c.SendToBackend(channel, heartbeat); err != nil {
		log.Printf("[WS] Heartbeat failed: %v", err)
	}
}

// reconnect 重新连接
func (c *Client) reconnect() {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()

	for i := 0; i < c.maxRetries; i++ {
		select {
		case <-c.stopChan:
			return
		default:
		}

		log.Printf("[WS] Reconnecting... attempt %d/%d", i+1, c.maxRetries)

		if err := c.Connect(); err == nil {
			log.Println("[WS] Reconnected successfully")
			return
		}

		time.Sleep(c.retryInterval)
	}

	log.Println("[WS] Max reconnection attempts reached")
}

// HandleChatRequest 处理聊天请求（从 Backend 接收）
func (c *Client) HandleChatRequest(sessionID string, messages []runtime.Message) error {
	log.Printf("[WS] Handling chat request for session %s", sessionID)

	// 获取或创建会话
	sess, err := c.sessionManager.Get(sessionID)
	if err != nil {
		// 创建新会话
		sess, err = c.sessionManager.Spawn(context.Background(), session.SpawnOptions{
			RuntimeType: "claude",
			WorkingDir:  ".",
			OnEvent: func(evt session.Event) {
				c.forwardEvent(evt)
			},
		})
		if err != nil {
			return fmt.Errorf("failed to spawn session: %w", err)
		}
		log.Printf("[WS] Created new session: %s", sess.ID)
	}

	// 发送聊天请求
	req := runtime.ChatRequest{
		SessionID: sessionID,
		Messages:  messages,
		Stream:    true,
	}

	onEvent := func(event runtime.StreamEvent) {
		c.forwardStreamEvent(sessionID, event)
	}

	// 异步处理聊天
	go func() {
		if err := c.sessionManager.Chat(context.Background(), sess.ID, req, onEvent); err != nil {
			log.Printf("[WS] Chat error: %v", err)
			// 发送错误到 Backend
			c.SendError(fmt.Sprintf("user:%s", c.deviceID), err)
		}
	}()

	return nil
}

// HandleIncomingMessage 处理来自 Backend 的消息
func (c *Client) HandleIncomingMessage(messageType string, payload []byte) error {
	c.mu.RLock()
	handler, exists := c.handlers[messageType]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no handler for message type: %s", messageType)
	}

	return handler(payload)
}

// forwardEvent 转发会话事件到 Backend
func (c *Client) forwardEvent(evt session.Event) {
	message := map[string]interface{}{
		"type":       evt.Type,
		"session_id": evt.SessionID,
		"data":       evt.Data,
		"timestamp":  evt.Timestamp.Unix(),
	}

	channel := fmt.Sprintf("user:%s", c.deviceID)
	if err := c.SendToBackend(channel, message); err != nil {
		log.Printf("[WS] Failed to forward event: %v", err)
	}
}

// forwardStreamEvent 转发流式事件到 Backend
func (c *Client) forwardStreamEvent(sessionID string, event runtime.StreamEvent) {
	// 映射到 Backend 的消息格式
	msgType := fmt.Sprintf("chat:%s", event.Type)

	message := map[string]interface{}{
		"type":       msgType,
		"session_id": sessionID,
		"timestamp":  time.Now().Unix(),
	}

	// 根据事件类型添加数据
	switch event.Type {
	case "text":
		message["data"] = map[string]interface{}{
			"content": event.Text,
		}
	case "thinking":
		message["data"] = map[string]interface{}{
			"thinking": event.Thinking,
		}
	case "tool_call":
		if event.ToolCall != nil {
			message["data"] = map[string]interface{}{
				"tool_name":  event.ToolCall.Name,
				"tool_input": event.ToolCall.Arguments,
			}
		}
	case "tool_result":
		if event.ToolResult != nil {
			message["data"] = map[string]interface{}{
				"output": event.ToolResult.Output,
				"error":  event.ToolResult.Error,
			}
		}
	case "error":
		message["data"] = map[string]interface{}{
			"error": event.Error,
		}
	case "done":
		message["data"] = map[string]interface{}{
			"done": true,
		}
	}

	channel := fmt.Sprintf("user:%s", c.deviceID)
	if err := c.SendToBackend(channel, message); err != nil {
		log.Printf("[WS] Failed to forward stream event: %v", err)
	}
}

// Stats 返回客户端统计信息
func (c *Client) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"connected":     c.connected,
		"device_id":     c.deviceID,
		"subscriptions": len(c.subscriptions),
		"handlers":      len(c.handlers),
		"reconnecting":  c.reconnecting,
	}
}
