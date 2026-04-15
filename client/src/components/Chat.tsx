import { useState } from "react";
import { useChat } from "~/hooks/useChat";

export function Chat() {
  const [input, setInput] = useState("");
  const { messages, isLoading, sendMessage } = useChat();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isLoading) return;

    const content = input;
    setInput("");
    await sendMessage(content);
  };

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "12px",
      }}
    >
      {/* 消息列表 */}
      <div
        style={{
          maxHeight: "300px",
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
              padding: "8px 12px",
              borderRadius: "8px",
              background: msg.role === "user" ? "#e94560" : "#2a2a4e",
              alignSelf: msg.role === "user" ? "flex-end" : "flex-start",
              maxWidth: "80%",
              fontSize: "14px",
            }}
          >
            {msg.content}
          </div>
        ))}

        {isLoading && (
          <div style={{ opacity: 0.5, fontSize: "14px" }}>
            正在输入...
          </div>
        )}
      </div>

      {/* 输入框 */}
      <form onSubmit={handleSubmit}>
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="发送消息..."
          disabled={isLoading}
          style={{
            width: "100%",
            padding: "12px 16px",
            borderRadius: "24px",
            border: "none",
            background: "rgba(0,0,0,0.5)",
            color: "#fff",
            fontSize: "14px",
            outline: "none",
          }}
        />
      </form>
    </div>
  );
}
