import { useState, useEffect, useRef } from "react";
import { useCentrifugo, ChatMessage } from "~/lib/centrifugo";

interface Message {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  isThinking?: boolean;
  toolCall?: {
    name: string;
    input: Record<string, unknown>;
  };
  error?: string;
}

export function Chat() {
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const { state, messages: wsMessages, sendMessage, deviceId } = useCentrifugo();

  // 处理 WebSocket 消息
  useEffect(() => {
    if (wsMessages.length === 0) return;

    const latestMessage = wsMessages[wsMessages.length - 1];
    handleWebSocketMessage(latestMessage);
  }, [wsMessages]);

  // 自动滚动到底部
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleWebSocketMessage = (msg: ChatMessage) => {
    switch (msg.type) {
      case "chat:thinking": {
        // 添加或更新思考状态消息
        setMessages((prev) => {
          const lastMsg = prev[prev.length - 1];
          if (lastMsg?.role === "assistant" && lastMsg.isThinking) {
            // 更新现有思考消息
            return [
              ...prev.slice(0, -1),
              { ...lastMsg, isThinking: msg.data.thinking ?? true },
            ];
          }
          // 添加新思考消息
          return [
            ...prev,
            {
              id: `thinking-${Date.now()}`,
              role: "assistant",
              content: "",
              isThinking: true,
            },
          ];
        });
        break;
      }

      case "chat:response": {
        // 添加或追加 AI 响应
        setMessages((prev) => {
          const lastMsg = prev[prev.length - 1];
          if (lastMsg?.role === "assistant" && !lastMsg.toolCall && !lastMsg.error) {
            // 追加到现有 AI 消息
            return [
              ...prev.slice(0, -1),
              {
                ...lastMsg,
                content: lastMsg.content + (msg.data.content || ""),
                isThinking: false,
              },
            ];
          }
          // 添加新 AI 消息
          return [
            ...prev,
            {
              id: msg.data.message_id || `msg-${Date.now()}`,
              role: "assistant",
              content: msg.data.content || "",
              isThinking: false,
            },
          ];
        });
        setIsLoading(false);
        break;
      }

      case "chat:tool_call": {
        // 添加工具调用消息
        setMessages((prev) => [
          ...prev,
          {
            id: `tool-${Date.now()}`,
            role: "system",
            content: `Using tool: ${msg.data.tool_name}`,
            toolCall: {
              name: msg.data.tool_name || "",
              input: msg.data.tool_input || {},
            },
          },
        ]);
        break;
      }

      case "chat:error": {
        // 添加错误消息
        setMessages((prev) => [
          ...prev,
          {
            id: `error-${Date.now()}`,
            role: "system",
            content: msg.data.error || "An error occurred",
            error: msg.data.error,
          },
        ]);
        setIsLoading(false);
        break;
      }

      case "chat:done": {
        // 流式响应完成
        setIsLoading(false);
        break;
      }
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isLoading || state !== "connected") return;

    const content = input.trim();
    setInput("");

    // 添加用户消息
    const userMessage: Message = {
      id: `user-${Date.now()}`,
      role: "user",
      content,
    };
    setMessages((prev) => [...prev, userMessage]);
    setIsLoading(true);

    try {
      // 获取或创建会话
      let currentSessionId = sessionId;
      if (!currentSessionId) {
        currentSessionId = await createSession();
        setSessionId(currentSessionId);
      }

      // 发送消息
      await sendMessage(currentSessionId, content);
    } catch (error) {
      console.error("Failed to send message:", error);
      setMessages((prev) => [
        ...prev,
        {
          id: `error-${Date.now()}`,
          role: "system",
          content: "Failed to send message. Please try again.",
          error: "Network error",
        },
      ]);
      setIsLoading(false);
    }
  };

  const createSession = async (): Promise<string> => {
    const apiUrl = import.meta.env.VITE_API_URL || "http://localhost:3000";

    const response = await fetch(`${apiUrl}/api/sessions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "x-device-id": deviceId || "",
      },
      body: JSON.stringify({
        title: "New Chat",
      }),
    });

    if (!response.ok) {
      throw new Error(`Failed to create session: ${response.status}`);
    }

    const data = await response.json();
    return data.data.session.id;
  };

  const clearChat = () => {
    setMessages([]);
    setSessionId(null);
  };

  // 连接状态指示器
  const getConnectionStatus = () => {
    switch (state) {
      case "connected":
        return { text: "已连接", color: "#4ade80" };
      case "connecting":
        return { text: "连接中...", color: "#facc15" };
      case "reconnecting":
        return { text: "重连中...", color: "#f97316" };
      default:
        return { text: "未连接", color: "#ef4444" };
    }
  };

  const status = getConnectionStatus();

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "12px",
        height: "100%",
      }}
    >
      {/* 连接状态 */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "8px 12px",
          background: "rgba(0,0,0,0.3)",
          borderRadius: "8px",
          fontSize: "12px",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
          <span
            style={{
              width: "8px",
              height: "8px",
              borderRadius: "50%",
              background: status.color,
            }}
          />
          <span>{status.text}</span>
        </div>
        {messages.length > 0 && (
          <button
            onClick={clearChat}
            style={{
              background: "transparent",
              border: "none",
              color: "#e94560",
              cursor: "pointer",
              fontSize: "12px",
            }}
          >
            清空对话
          </button>
        )}
      </div>

      {/* 消息列表 */}
      <div
        style={{
          flex: 1,
          overflowY: "auto",
          display: "flex",
          flexDirection: "column",
          gap: "8px",
          padding: "12px",
          background: "rgba(0,0,0,0.3)",
          borderRadius: "12px",
          backdropFilter: "blur(8px)",
        }}
      >
        {messages.length === 0 && (
          <div style={{ textAlign: "center", opacity: 0.5, fontSize: "14px" }}>
            开始对话吧...
          </div>
        )}

        {messages.map((msg) => (
          <div
            key={msg.id}
            style={{
              padding: "10px 14px",
              borderRadius: "12px",
              background:
                msg.role === "user"
                  ? "#e94560"
                  : msg.error
                  ? "rgba(239,68,68,0.3)"
                  : msg.toolCall
                  ? "rgba(59,130,246,0.3)"
                  : "#2a2a4e",
              alignSelf: msg.role === "user" ? "flex-end" : "flex-start",
              maxWidth: "85%",
              fontSize: "14px",
              lineHeight: "1.5",
              wordBreak: "break-word",
            }}
          >
            {msg.isThinking ? (
              <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <span style={{ opacity: 0.7 }}>思考中</span>
                <span
                  style={{
                    display: "inline-block",
                    width: "12px",
                    height: "12px",
                    border: "2px solid rgba(255,255,255,0.3)",
                    borderTopColor: "#fff",
                    borderRadius: "50%",
                    animation: "spin 1s linear infinite",
                  }}
                />
              </div>
            ) : (
              <>
                {msg.toolCall && (
                  <div
                    style={{
                      fontSize: "12px",
                      opacity: 0.8,
                      marginBottom: "4px",
                    }}
                  >
                    🔧 {msg.toolCall.name}
                  </div>
                )}
                {msg.error && (
                  <div
                    style={{
                      fontSize: "12px",
                      color: "#fca5a5",
                      marginBottom: "4px",
                    }}
                  >
                    ⚠️ 错误
                  </div>
                )}
                <div style={{ whiteSpace: "pre-wrap" }}>{msg.content}</div>
              </>
            )}
          </div>
        ))}

        {isLoading && messages[messages.length - 1]?.role === "user" && (
          <div style={{ opacity: 0.5, fontSize: "14px" }}>等待响应...</div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* 输入框 */}
      <form onSubmit={handleSubmit}>
        <div style={{ display: "flex", gap: "8px" }}>
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={
              state === "connected" ? "发送消息..." : "等待连接..."
            }
            disabled={isLoading || state !== "connected"}
            style={{
              flex: 1,
              padding: "12px 16px",
              borderRadius: "24px",
              border: "none",
              background: "rgba(0,0,0,0.5)",
              color: "#fff",
              fontSize: "14px",
              outline: "none",
            }}
          />
          <button
            type="submit"
            disabled={isLoading || state !== "connected" || !input.trim()}
            style={{
              padding: "12px 20px",
              borderRadius: "24px",
              border: "none",
              background: "#e94560",
              color: "#fff",
              fontSize: "14px",
              cursor:
                isLoading || state !== "connected" ? "not-allowed" : "pointer",
              opacity: isLoading || state !== "connected" ? 0.6 : 1,
            }}
          >
            发送
          </button>
        </div>
      </form>

      {/* CSS 动画 */}
      <style>{`
        @keyframes spin {
          to {
            transform: rotate(360deg);
          }
        }
      `}</style>
    </div>
  );
}
