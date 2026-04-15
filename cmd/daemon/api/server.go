package api

import (
    "encoding/json"
    "log"
    "net/http"

    "github.com/loom/daemon/runtime"
)

// Server represents an HTTP API server
type Server struct {
    runtime runtime.Runtime
    addr    string
}

// ChatRequest represents a chat request
type ChatRequest struct {
    Messages []runtime.Message `json:"messages"`
    Tools    []runtime.Tool    `json:"tools"`
    Stream   bool              `json:"stream"`
}

// ChatResponse represents a chat response
type ChatResponse struct {
    Content  string        `json:"content"`
    ToolCall *runtime.Tool `json:"tool_call,omitempty"`
    Done     bool          `json:"done"`
}

// ToolRequest represents a tool execution request
type ToolRequest struct {
    Name  string                 `json:"name"`
    Input map[string]interface{} `json:"input"`
}

// ToolResponse represents a tool execution response
type ToolResponse struct {
    Output string `json:"output"`
    Error  string `json:"error,omitempty"`
}

// CapabilitiesResponse represents capabilities response
type CapabilitiesResponse struct {
    Tools []string `json:"tools"`
    Skills []string `json:"skills"`
}

// ToolsResponse represents tools list response
type ToolsResponse struct {
    Tools []string `json:"tools"`
}

// NewServer creates a new API server
func NewServer(rt runtime.Runtime, addr string) *Server {
    return &Server{
        runtime: rt,
        addr:    addr,
    }
}

// Start starts the API server
func (s *Server) Start() error {
    http.HandleFunc("/api/chat", s.handleChat)
    http.HandleFunc("/api/tools", s.handleTools)
    http.HandleFunc("/api/tools/execute", s.handleToolExecute)
    http.HandleFunc("/api/capabilities", s.handleCapabilities)
    http.HandleFunc("/health", s.handleHealth)

    log.Printf("Starting API server on %s", s.addr)
    return http.ListenAndServe(s.addr, nil)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req ChatRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    encoder := json.NewEncoder(w)

    runtimeReq := runtime.ChatRequest{
        Messages: req.Messages,
        Tools:    req.Tools,
        Stream:   req.Stream,
    }

    err := s.runtime.Chat(r.Context(), runtimeReq, func(resp runtime.ChatResponse) {
        encoder.Encode(ChatResponse{
            Content:  resp.Content,
            ToolCall:  resp.ToolCall,
            Done:      resp.Done,
        })
    })

    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    tools, _ := s.runtime.ListCapabilities()
    json.NewEncoder(w).Encode(ToolsResponse{Tools: tools})
}

func (s *Server) handleToolExecute(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req ToolRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    output, err := s.runtime.ExecuteTool(r.Context(), req.Name, req.Input)
    if err != nil {
        json.NewEncoder(w).Encode(ToolResponse{Error: err.Error()})
        return
    }

    json.NewEncoder(w).Encode(ToolResponse{Output: output})
}

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
    tools, skills := s.runtime.ListCapabilities()
    json.NewEncoder(w).Encode(CapabilitiesResponse{
        Tools:  tools,
        Skills: skills,
    })
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
