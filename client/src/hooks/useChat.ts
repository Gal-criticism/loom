import { useState, useCallback, useEffect } from "react";
import { centrifugo, ChatMessage } from "~/lib/centrifugo";

interface Message {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  metadata?: {
    thinking?: boolean;
    toolCall?: {
      name: string;
      input: Record<string, unknown>;
    };
    error?: string;
  };
}

interface UseChatOptions {
  sessionId?: string;
  onMessage?: (message: Message) => void;
}

export function useChat(options: UseChatOptions = {}) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(
    options.sessionId || null
  );

  // 处理 incoming WebSocket 消息
  useEffect(() => {
    const unsubscribe = centrifugo.onMessage((chatMessage: ChatMessage) => {
      handleIncomingMessage(chatMessage);
    });

    return () => unsubscribe();
  }, []);

  const handleIncomingMessage = (chatMessage: ChatMessage) => {
    switch (chatMessage.type) {
      case "chat:thinking": {
        // Update last assistant message to show thinking state
        setMessages((prev) => {
          const lastMsg = prev[prev.length - 1];
          if (lastMsg?.role === "assistant") {
            return [
              ...prev.slice(0, -1),
              {
                ...lastMsg,
                metadata: { ...lastMsg.metadata, thinking: true },
              },
            ];
          }
          // Add new thinking message placeholder
          const newMessage: Message = {
            id: `thinking-${Date.now()}`,
            role: "assistant",
            content: "",
            metadata: { thinking: true },
          };
          return [...prev, newMessage];
        });
        break;
      }

      case "chat:response": {
        // Append content to assistant message
        setMessages((prev) => {
          const lastMsg = prev[prev.length - 1];
          if (lastMsg?.role === "assistant" && !lastMsg.metadata?.toolCall) {
            return [
              ...prev.slice(0, -1),
              {
                ...lastMsg,
                content: lastMsg.content + (chatMessage.data.content || ""),
                metadata: { ...lastMsg.metadata, thinking: false },
              },
            ];
          }
          // Add new assistant message
          const newMessage: Message = {
            id: chatMessage.data.message_id || `msg-${Date.now()}`,
            role: "assistant",
            content: chatMessage.data.content || "",
            metadata: { thinking: false },
          };
          return [...prev, newMessage];
        });
        setIsLoading(false);
        break;
      }

      case "chat:tool_call": {
        // Add tool call system message
        const toolMessage: Message = {
          id: `tool-${Date.now()}`,
          role: "system",
          content: `Tool: ${chatMessage.data.tool_name}`,
          metadata: {
            toolCall: {
              name: chatMessage.data.tool_name || "",
              input: chatMessage.data.tool_input || {},
            },
          },
        };
        setMessages((prev) => [...prev, toolMessage]);
        break;
      }

      case "chat:error": {
        const errorMessage: Message = {
          id: `error-${Date.now()}`,
          role: "system",
          content: chatMessage.data.error || "Unknown error",
          metadata: { error: chatMessage.data.error },
        };
        setMessages((prev) => [...prev, errorMessage]);
        setError(chatMessage.data.error || "Unknown error");
        setIsLoading(false);
        break;
      }

      case "chat:done": {
        setIsLoading(false);
        break;
      }
    }

    // Call optional callback
    if (options.onMessage) {
      const lastMessage = messages[messages.length - 1];
      if (lastMessage) {
        options.onMessage(lastMessage);
      }
    }
  };

  const sendMessage = useCallback(
    async (content: string) => {
      if (!content.trim()) return;

      setError(null);

      // Add user message
      const userMessage: Message = {
        id: `user-${Date.now()}`,
        role: "user",
        content: content.trim(),
      };
      setMessages((prev) => [...prev, userMessage]);
      setIsLoading(true);

      try {
        // Get or create session
        let sessionId = currentSessionId;
        if (!sessionId) {
          sessionId = await createSession();
          setCurrentSessionId(sessionId);
        }

        // Send via HTTP API
        await centrifugo.sendHttpMessage(sessionId, content.trim());
      } catch (err) {
        const errorMsg = err instanceof Error ? err.message : "Failed to send message";
        setError(errorMsg);
        setMessages((prev) => [
          ...prev,
          {
            id: `error-${Date.now()}`,
            role: "system",
            content: errorMsg,
            metadata: { error: errorMsg },
          },
        ]);
        setIsLoading(false);
      }
    },
    [currentSessionId, options.onMessage]
  );

  const createSession = async (): Promise<string> => {
    const apiUrl = import.meta.env.VITE_API_URL || "http://localhost:3000";
    const deviceId = centrifugo.getDeviceId();

    const response = await fetch(`${apiUrl}/api/sessions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "x-device-id": deviceId || "",
      },
      body: JSON.stringify({ title: "New Chat Session" }),
    });

    if (!response.ok) {
      throw new Error(`Failed to create session: ${response.statusText}`);
    }

    const data = await response.json();
    return data.data.session.id;
  };

  const clearMessages = useCallback(() => {
    setMessages([]);
    setCurrentSessionId(null);
    setError(null);
  }, []);

  const setSession = useCallback((sessionId: string) => {
    setCurrentSessionId(sessionId);
    setMessages([]);
  }, []);

  return {
    messages,
    isLoading,
    error,
    sessionId: currentSessionId,
    sendMessage,
    clearMessages,
    setSession,
  };
}
