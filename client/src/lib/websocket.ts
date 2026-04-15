type MessageHandler = (data: any) => void;

interface WSConnection {
  connect(url: string, token?: string): void;
  disconnect(): void;
  send(type: string, payload: any): void;
  on(type: string, handler: MessageHandler): void;
  off(type: string, handler: MessageHandler): void;
}

export function createWSConnection(): WSConnection {
  let ws: WebSocket | null = null;
  let handlers: Map<string, MessageHandler[]> = new Map();
  let reconnectAttempts = 0;
  const maxReconnectAttempts = 5;

  const connect = (url: string, token?: string) => {
    ws = new WebSocket(url);

    ws.onopen = () => {
      console.log("WebSocket connected");
      reconnectAttempts = 0;

      // 发送认证
      if (token) {
        ws?.send(JSON.stringify({ type: "auth", payload: { token } }));
      }
    };

    ws.onmessage = (event) => {
      try {
        const { type, payload } = JSON.parse(event.data);
        const typeHandlers = handlers.get(type) || [];
        typeHandlers.forEach((handler) => handler(payload));
      } catch (error) {
        console.error("WS message error:", error);
      }
    };

    ws.onclose = () => {
      console.log("WebSocket closed");
      // 自动重连
      if (reconnectAttempts < maxReconnectAttempts) {
        reconnectAttempts++;
        setTimeout(() => connect(url, token), 1000 * reconnectAttempts);
      }
    };

    ws.onerror = (error) => {
      console.error("WebSocket error:", error);
    };
  };

  const disconnect = () => {
    if (ws) {
      ws.close();
      ws = null;
    }
  };

  const send = (type: string, payload: any) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type, payload }));
    }
  };

  const on = (type: string, handler: MessageHandler) => {
    const typeHandlers = handlers.get(type) || [];
    typeHandlers.push(handler);
    handlers.set(type, typeHandlers);
  };

  const off = (type: string, handler: MessageHandler) => {
    const typeHandlers = handlers.get(type) || [];
    const filtered = typeHandlers.filter((h) => h !== handler);
    handlers.set(type, filtered);
  };

  return { connect, disconnect, send, on, off };
}

export const ws = createWSConnection();
