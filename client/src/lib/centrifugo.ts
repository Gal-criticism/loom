/**
 * Centrifugo Client
 * 通过 Centrifugo 与后端实时通信
 */

import { Centrifuge, Subscription } from "centrifuge";

// 消息类型定义
export interface ChatMessage {
  type: "chat:response" | "chat:thinking" | "chat:error" | "chat:tool_call" | "chat:done";
  data: {
    session_id: string;
    message_id?: string;
    content?: string;
    thinking?: boolean;
    error?: string;
    tool_name?: string;
    tool_input?: Record<string, unknown>;
    tool_call_id?: string;
    done?: boolean;
  };
}

export interface UserMessage {
  session_id: string;
  content: string;
}

// 连接状态
type ConnectionState = "disconnected" | "connecting" | "connected" | "reconnecting";

// 事件处理器
type MessageHandler = (message: ChatMessage) => void;
type StateChangeHandler = (state: ConnectionState) => void;

class CentrifugoClient {
  private centrifuge: Centrifuge | null = null;
  private subscription: Subscription | null = null;
  private state: ConnectionState = "disconnected";
  private deviceId: string | null = null;
  private token: string | null = null;

  // 事件处理器
  private messageHandlers: MessageHandler[] = [];
  private stateHandlers: StateChangeHandler[] = [];

  // 重连配置
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;

  // 单例模式
  private static instance: CentrifugoClient | null = null;

  static getInstance(): CentrifugoClient {
    if (!CentrifugoClient.instance) {
      CentrifugoClient.instance = new CentrifugoClient();
    }
    return CentrifugoClient.instance;
  }

  // 获取或创建设备 ID
  private getDeviceId(): string {
    if (this.deviceId) return this.deviceId;

    // 从 localStorage 读取
    let deviceId = localStorage.getItem("loom_device_id");
    if (!deviceId) {
      // 生成新的 device ID
      deviceId = `device_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
      localStorage.setItem("loom_device_id", deviceId);
    }
    this.deviceId = deviceId;
    return deviceId;
  }

  // 设备认证，获取 Centrifugo token
  private async authenticate(): Promise<{ token: string; userId: string } | null> {
    try {
      const deviceId = this.getDeviceId();
      const apiUrl = import.meta.env.VITE_API_URL || "http://localhost:3000";

      const response = await fetch(`${apiUrl}/api/auth/device`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ device_id: deviceId }),
      });

      if (!response.ok) {
        throw new Error(`Auth failed: ${response.status}`);
      }

      const data = await response.json();

      // 保存 device ID 和用户信息
      if (data.data?.user?.device_id) {
        localStorage.setItem("loom_device_id", data.data.user.device_id);
        this.deviceId = data.data.user.device_id;
      }

      // TODO: 后端应该返回 Centrifugo JWT token
      // 暂时使用 device_id 作为 token（需要后端配合）
      return {
        token: data.data.user.device_id,
        userId: data.data.user.id,
      };
    } catch (error) {
      console.error("[Centrifugo] Authentication failed:", error);
      return null;
    }
  }

  // 连接 Centrifugo
  async connect(): Promise<void> {
    if (this.state === "connected" || this.state === "connecting") {
      return;
    }

    this.setState("connecting");

    // 认证获取 token
    const auth = await this.authenticate();
    if (!auth) {
      this.setState("disconnected");
      throw new Error("Authentication failed");
    }

    this.token = auth.token;

    const wsUrl = import.meta.env.VITE_WS_URL || "ws://localhost:8000";

    // 创建 Centrifuge 客户端
    this.centrifuge = new Centrifuge(`${wsUrl}/connection/websocket`, {
      token: this.token,
      name: "loom-client",
      version: "0.1.0",
    });

    // 设置事件处理器
    this.centrifuge.on("connected", (ctx) => {
      console.log("[Centrifugo] Connected:", ctx.client);
      this.setState("connected");
      this.reconnectAttempts = 0;

      // 订阅用户频道
      this.subscribeToUserChannel();
    });

    this.centrifuge.on("disconnected", (ctx) => {
      console.log("[Centrifugo] Disconnected:", ctx.reason);
      this.setState("reconnecting");
      this.scheduleReconnect();
    });

    this.centrifuge.on("error", (ctx) => {
      console.error("[Centrifugo] Error:", ctx.error);
    });

    // 连接
    this.centrifuge.connect();
  }

  // 订阅用户频道接收消息
  private subscribeToUserChannel(): void {
    if (!this.centrifuge || !this.deviceId) return;

    const channel = `user:${this.deviceId}`;

    this.subscription = this.centrifuge.newSubscription(channel);

    this.subscription.on("publication", (ctx) => {
      console.log("[Centrifugo] Received message:", ctx.data);
      this.handleMessage(ctx.data as ChatMessage);
    });

    this.subscription.on("subscribed", () => {
      console.log("[Centrifugo] Subscribed to", channel);
    });

    this.subscription.on("unsubscribed", () => {
      console.log("[Centrifugo] Unsubscribed from", channel);
    });

    this.subscription.on("error", (ctx) => {
      console.error("[Centrifugo] Subscription error:", ctx.error);
    });

    this.subscription.subscribe();
  }

  // 断开连接
  disconnect(): void {
    if (this.subscription) {
      this.subscription.unsubscribe();
      this.subscription = null;
    }

    if (this.centrifuge) {
      this.centrifuge.disconnect();
      this.centrifuge = null;
    }

    this.setState("disconnected");
  }

  // 发送消息到后端
  async sendMessage(message: UserMessage): Promise<void> {
    if (!this.centrifuge || this.state !== "connected") {
      throw new Error("Not connected to Centrifugo");
    }

    const channel = `daemon:${this.deviceId}`;

    const payload = {
      type: "chat:message",
      data: {
        session_id: message.session_id,
        content: message.content,
        timestamp: new Date().toISOString(),
      },
    };

    try {
      await this.centrifuge.publish(channel, payload);
      console.log("[Centrifugo] Message sent:", payload);
    } catch (error) {
      console.error("[Centrifugo] Failed to send message:", error);
      throw error;
    }
  }

  // 发送 HTTP API 请求（用于非实时操作）
  async sendHttpMessage(sessionId: string, content: string): Promise<void> {
    const apiUrl = import.meta.env.VITE_API_URL || "http://localhost:3000";
    const deviceId = this.getDeviceId();

    const response = await fetch(`${apiUrl}/api/messages`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "x-device-id": deviceId,
      },
      body: JSON.stringify({
        session_id: sessionId,
        content,
      }),
    });

    if (!response.ok) {
      throw new Error(`HTTP error: ${response.status}`);
    }
  }

  // 注册消息处理器
  onMessage(handler: MessageHandler): () => void {
    this.messageHandlers.push(handler);
    return () => {
      this.messageHandlers = this.messageHandlers.filter((h) => h !== handler);
    };
  }

  // 注册状态变化处理器
  onStateChange(handler: StateChangeHandler): () => void {
    this.stateHandlers.push(handler);
    return () => {
      this.stateHandlers = this.stateHandlers.filter((h) => h !== handler);
    };
  }

  // 获取当前状态
  getState(): ConnectionState {
    return this.state;
  }

  // 获取设备 ID
  getDeviceId(): string | null {
    return this.deviceId;
  }

  // 内部：设置状态
  private setState(state: ConnectionState): void {
    this.state = state;
    this.stateHandlers.forEach((handler) => handler(state));
  }

  // 内部：处理消息
  private handleMessage(message: ChatMessage): void {
    this.messageHandlers.forEach((handler) => handler(message));
  }

  // 内部：调度重连
  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error("[Centrifugo] Max reconnection attempts reached");
      this.setState("disconnected");
      return;
    }

    this.reconnectAttempts++;
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts - 1), 30000);

    console.log(`[Centrifugo] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);

    setTimeout(() => {
      if (this.state === "reconnecting") {
        this.connect().catch((err) => {
          console.error("[Centrifugo] Reconnect failed:", err);
        });
      }
    }, delay);
  }
}

// 导出单例
export const centrifugo = CentrifugoClient.getInstance();

// React Hook
import { useEffect, useState, useCallback } from "react";

export function useCentrifugo() {
  const [state, setState] = useState<ConnectionState>("disconnected");
  const [messages, setMessages] = useState<ChatMessage[]>([]);

  useEffect(() => {
    // 监听状态变化
    const unsubscribeState = centrifugo.onStateChange((newState) => {
      setState(newState);
    });

    // 监听消息
    const unsubscribeMessage = centrifugo.onMessage((message) => {
      setMessages((prev) => [...prev, message]);
    });

    // 自动连接
    if (state === "disconnected") {
      centrifugo.connect().catch(console.error);
    }

    return () => {
      unsubscribeState();
      unsubscribeMessage();
    };
  }, []);

  const sendMessage = useCallback(async (sessionId: string, content: string) => {
    await centrifugo.sendHttpMessage(sessionId, content);
  }, []);

  const connect = useCallback(() => {
    return centrifugo.connect();
  }, []);

  const disconnect = useCallback(() => {
    centrifugo.disconnect();
  }, []);

  return {
    state,
    messages,
    sendMessage,
    connect,
    disconnect,
    deviceId: centrifugo.getDeviceId(),
  };
}
