package ws

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

// Message represents a WebSocket message
type Message struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

// Handler is a callback for handling messages
type Handler func(payload json.RawMessage) error

// Client represents a WebSocket client
type Client struct {
    conn     *websocket.Conn
    url      string
    deviceID string
    mu       sync.Mutex
    handlers map[string]Handler
    ctx      context.Context
    cancel   context.CancelFunc
}

// NewClient creates a new WebSocket client
func NewClient(url, deviceID string) *Client {
    return &Client{
        url:      url,
        deviceID: deviceID,
        handlers: make(map[string]Handler),
    }
}

// Connect connects to the WebSocket server
func (c *Client) Connect(ctx context.Context) error {
    c.ctx, c.cancel = context.WithCancel(ctx)

    conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, nil)
    if err != nil {
        return fmt.Errorf("failed to connect to backend: %w", err)
    }

    c.conn = conn

    // 发送注册消息
    registerMsg := Message{
        Type: "register",
        Payload: mustMarshal(map[string]string{
            "device_id": c.deviceID,
            "runtime":   "claude-code",
        }),
    }

    if err := c.conn.WriteJSON(registerMsg); err != nil {
        return err
    }

    go c.readLoop()
    return nil
}

func (c *Client) readLoop() {
    for {
        select {
        case <-c.ctx.Done():
            c.conn.Close()
            return
        default:
            _, data, err := c.conn.ReadMessage()
            if err != nil {
                log.Printf("Error reading message: %v", err)
                continue
            }

            var msg Message
            if err := json.Unmarshal(data, &msg); err != nil {
                log.Printf("Error unmarshaling message: %v", err)
                continue
            }

            if handler, ok := c.handlers[msg.Type]; ok {
                if err := handler(msg.Payload); err != nil {
                    log.Printf("Error handling message: %v", err)
                }
            }
        }
    }
}

// On registers a handler for a message type
func (c *Client) On(event string, handler Handler) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.handlers[event] = handler
}

// Send sends a message to the server
func (c *Client) Send(typ string, payload interface{}) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    data, err := json.Marshal(Message{
        Type:    typ,
        Payload: mustMarshal(payload),
    })
    if err != nil {
        return err
    }

    return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Close closes the connection
func (c *Client) Close() error {
    if c.cancel != nil {
        c.cancel()
    }
    if c.conn != nil {
        return c.conn.Close()
    }
    return nil
}

func mustMarshal(v interface{}) json.RawMessage {
    data, _ := json.Marshal(v)
    return data
}
