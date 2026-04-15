import { useState, useCallback, useEffect } from "react";
import { ws } from "~/lib/websocket";

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
}

export function useChat() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  // 监听 AI 响应
  useEffect(() => {
    const handleResponse = (payload: any) => {
      const assistantMessage: Message = {
        id: Date.now().toString(),
        role: "assistant",
        content: payload.content,
      };
      setMessages((prev) => [...prev, assistantMessage]);
      setIsLoading(false);
    };

    ws.on("chat_response", handleResponse);

    return () => {
      ws.off("chat_response", handleResponse);
    };
  }, []);

  const sendMessage = useCallback(async (content: string) => {
    const userMessage: Message = {
      id: Date.now().toString(),
      role: "user",
      content,
    };

    setMessages((prev) => [...prev, userMessage]);
    setIsLoading(true);

    // 通过 WebSocket 发送
    ws.send("chat_request", {
      messages: [...messages, userMessage],
    });
  }, [messages]);

  const clearMessages = useCallback(() => {
    setMessages([]);
  }, []);

  return { messages, isLoading, sendMessage, clearMessages };
}
