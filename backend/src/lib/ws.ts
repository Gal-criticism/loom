import { Centrifuge, Subscription } from "centrifuge";
import { processDaemonMessage, ClientMessage } from "./messageHandler";

// WebSocket connection states
export enum ConnectionState {
  DISCONNECTED = "disconnected",
  CONNECTING = "connecting",
  CONNECTED = "connected",
  RECONNECTING = "reconnecting",
}

// Client connection metadata
interface ClientConnection {
  ws: WebSocket;
  deviceId: string;
  userId: string;
}

// Subscription metadata
interface SubscriptionMeta {
  subscription: Subscription;
  deviceId: string;
}

export interface WSServer {
  // Connection state
  getState(): ConnectionState;

  // Client connection management
  handleConnection(ws: WebSocket, deviceId: string, userId: string): void;
  removeConnection(ws: WebSocket): void;

  // Broadcasting
  broadcast(userId: string, message: ClientMessage): void;
  broadcastToSession(sessionId: string, message: ClientMessage): void;

  // Daemon communication
  sendToDaemon(deviceId: string, message: Record<string, unknown>): Promise<void>;

  // Lifecycle
  connect(): Promise<void>;
  disconnect(): void;
}

class WSServerImpl implements WSServer {
  private centrifuge: Centrifuge;
  private state: ConnectionState = ConnectionState.DISCONNECTED;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectDelay = 1000; // Start with 1 second
  private maxReconnectDelay = 30000; // Max 30 seconds
  private reconnectTimeoutId: ReturnType<typeof setTimeout> | null = null;

  // Client connections: Map<WebSocket, ClientConnection>
  private clientConnections = new Map<WebSocket, ClientConnection>();

  // User to connections mapping: Map<userId, Set<WebSocket>>
  private userConnections = new Map<string, Set<WebSocket>>();

  // Device to user mapping for subscriptions: Map<deviceId, userId>
  private deviceToUser = new Map<string, string>();

  // Active subscriptions to user channels from Daemon: Map<deviceId, SubscriptionMeta>
  private daemonSubscriptions = new Map<string, SubscriptionMeta>();

  // Pending subscription promises to prevent race conditions: Map<deviceId, Promise<void>>
  private pendingSubscriptions = new Map<string, Promise<void>>();

  // Centrifugo configuration
  private centrifugoUrl: string;
  private centrifugoToken: string;

  constructor() {
    this.centrifugoUrl = process.env.CENTRIFUGO_URL || "ws://localhost:8000/connection/websocket";
    this.centrifugoToken = process.env.CENTRIFUGO_TOKEN || "dev-token";

    this.centrifuge = new Centrifuge(this.centrifugoUrl, {
      token: this.centrifugoToken,
    });

    this.setupEventHandlers();
  }

  private setupEventHandlers(): void {
    // Connection established
    this.centrifuge.on("connected", (ctx) => {
      console.log("[WebSocket] Connected to Centrifugo", {
        clientId: ctx.client,
        transport: ctx.transport,
      });
      this.state = ConnectionState.CONNECTED;
      this.reconnectAttempts = 0;
      this.reconnectDelay = 1000;

      // Re-subscribe to all user channels after reconnection
      this.resubscribeAll();
    });

    // Connection disconnected
    this.centrifuge.on("disconnected", (ctx) => {
      console.log("[WebSocket] Disconnected from Centrifugo", {
        reason: ctx.reason,
      });
      // Always attempt to reconnect on disconnect
      this.state = ConnectionState.RECONNECTING;
      this.scheduleReconnect();
    });

    // Error handling
    this.centrifuge.on("error", (ctx) => {
      console.error("[WebSocket] Centrifuge error:", ctx.error);
    });
  }

  private async resubscribeAll(): Promise<void> {
    console.log("[WebSocket] Re-subscribing to all user channels...");

    // Re-subscribe to all device channels
    for (const [deviceId, userId] of this.deviceToUser.entries()) {
      try {
        await this.subscribeToUserChannel(deviceId, userId);
      } catch (error) {
        console.error(`[WebSocket] Failed to re-subscribe to user:${deviceId}:`, error);
      }
    }
  }

  getState(): ConnectionState {
    return this.state;
  }

  async connect(): Promise<void> {
    if (this.state === ConnectionState.CONNECTED) {
      return;
    }

    if (this.state === ConnectionState.CONNECTING) {
      return;
    }

    this.state = ConnectionState.CONNECTING;

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error("Connection timeout"));
      }, 10000);

      this.centrifuge.once("connected", () => {
        clearTimeout(timeout);
        resolve();
      });

      this.centrifuge.once("error", (ctx) => {
        clearTimeout(timeout);
        reject(ctx.error);
      });

      this.centrifuge.connect();
    });
  }

  disconnect(): void {
    // Clear any pending reconnect
    if (this.reconnectTimeoutId) {
      clearTimeout(this.reconnectTimeoutId);
      this.reconnectTimeoutId = null;
    }

    this.centrifuge.disconnect();
    this.state = ConnectionState.DISCONNECTED;

    // Clean up all subscriptions
    for (const [deviceId, meta] of this.daemonSubscriptions.entries()) {
      try {
        meta.subscription.unsubscribe();
      } catch (error) {
        console.error(`[WebSocket] Error unsubscribing from ${deviceId}:`, error);
      }
    }
    this.daemonSubscriptions.clear();
    this.pendingSubscriptions.clear();
  }

  /**
   * Schedule a reconnection attempt with exponential backoff
   */
  private scheduleReconnect(): void {
    if (this.reconnectTimeoutId) {
      return; // Already scheduled
    }

    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error("[WebSocket] Max reconnection attempts reached, giving up");
      this.state = ConnectionState.DISCONNECTED;
      return;
    }

    this.reconnectAttempts++;
    const delay = Math.min(
      this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1),
      this.maxReconnectDelay
    );

    console.log(`[WebSocket] Scheduling reconnect attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts} in ${delay}ms`);

    this.reconnectTimeoutId = setTimeout(() => {
      this.reconnectTimeoutId = null;
      if (this.state === ConnectionState.RECONNECTING) {
        console.log("[WebSocket] Attempting to reconnect...");
        this.centrifuge.connect();
      }
    }, delay);
  }

  /**
   * Subscribe to user:${deviceId} channel to receive messages from Daemon
   * Uses promise-based locking to prevent race conditions
   */
  private async subscribeToUserChannel(
    deviceId: string,
    userId: string
  ): Promise<void> {
    // Check if already subscribed
    if (this.daemonSubscriptions.has(deviceId)) {
      return;
    }

    // Check if there's a pending subscription to prevent race conditions
    const pending = this.pendingSubscriptions.get(deviceId);
    if (pending) {
      return pending;
    }

    // Create the subscription promise
    const subscriptionPromise = this.doSubscribeToUserChannel(deviceId, userId);
    this.pendingSubscriptions.set(deviceId, subscriptionPromise);

    try {
      await subscriptionPromise;
    } finally {
      // Clean up pending subscription
      this.pendingSubscriptions.delete(deviceId);
    }
  }

  /**
   * Actual subscription implementation
   */
  private async doSubscribeToUserChannel(
    deviceId: string,
    userId: string
  ): Promise<void> {
    // Double-check after acquiring lock
    if (this.daemonSubscriptions.has(deviceId)) {
      return;
    }

    const channel = `user:${deviceId}`;

    const subscription = this.centrifuge.newSubscription(channel);

    subscription.on("publication", async (ctx) => {
      console.log(`[WebSocket] Received message from Daemon on ${channel}:`, ctx.data);

      try {
        // Process the Daemon message
        const clientMessage = await processDaemonMessage(deviceId, ctx.data);

        if (clientMessage) {
          // Broadcast to all connected clients for this user
          this.broadcast(userId, clientMessage);
        }
      } catch (error) {
        console.error("[WebSocket] Error processing Daemon message:", error);
      }
    });

    subscription.on("subscribed", () => {
      console.log(`[WebSocket] Subscribed to channel: ${channel}`);
    });

    subscription.on("unsubscribed", () => {
      console.log(`[WebSocket] Unsubscribed from channel: ${channel}`);
    });

    subscription.on("error", (ctx) => {
      console.error(`[WebSocket] Subscription error for ${channel}:`, ctx.error);
    });

    // Subscribe to the channel
    await subscription.subscribe();

    // Store subscription
    this.daemonSubscriptions.set(deviceId, {
      subscription,
      deviceId,
    });
  }

  /**
   * Handle new client WebSocket connection
   */
  handleConnection(ws: WebSocket, deviceId: string, userId: string): void {
    console.log(`[WebSocket] New client connection: deviceId=${deviceId}, userId=${userId}`);

    // Store connection
    this.clientConnections.set(ws, { ws, deviceId, userId });

    // Add to user connections
    if (!this.userConnections.has(userId)) {
      this.userConnections.set(userId, new Set());
    }
    this.userConnections.get(userId)!.add(ws);

    // Map device to user
    this.deviceToUser.set(deviceId, userId);

    // Subscribe to user channel to receive Daemon messages
    this.subscribeToUserChannel(deviceId, userId).catch((error) => {
      console.error(`[WebSocket] Failed to subscribe to user channel for ${deviceId}:`, error);
      // Disconnect client to prevent memory leak and stale connections
      console.log(`[WebSocket] Disconnecting client ${deviceId} due to subscription failure`);
      ws.close(1011, "Subscription failed");
      this.removeConnection(ws);
    });

    // Handle client disconnect
    ws.addEventListener("close", () => {
      this.removeConnection(ws);
    });

    ws.addEventListener("error", (error) => {
      console.error(`[WebSocket] Client connection error for ${deviceId}:`, error);
      this.removeConnection(ws);
    });
  }

  /**
   * Remove client connection
   */
  removeConnection(ws: WebSocket): void {
    const conn = this.clientConnections.get(ws);
    if (!conn) {
      return;
    }

    console.log(`[WebSocket] Removing client connection: deviceId=${conn.deviceId}`);

    // Remove from client connections
    this.clientConnections.delete(ws);

    // Remove from user connections
    const userConns = this.userConnections.get(conn.userId);
    if (userConns) {
      userConns.delete(ws);
      if (userConns.size === 0) {
        this.userConnections.delete(conn.userId);
      }
    }

    // Check if any other connections exist for this device
    let hasOtherConnections = false;
    for (const [, c] of this.clientConnections.entries()) {
      if (c.deviceId === conn.deviceId) {
        hasOtherConnections = true;
        break;
      }
    }

    // If no other connections for this device, unsubscribe from user channel
    if (!hasOtherConnections) {
      const meta = this.daemonSubscriptions.get(conn.deviceId);
      if (meta) {
        console.log(`[WebSocket] Unsubscribing from user:${conn.deviceId} (no more clients)`);
        meta.subscription.unsubscribe();
        this.daemonSubscriptions.delete(conn.deviceId);
        this.deviceToUser.delete(conn.deviceId);
      }
    }
  }

  /**
   * Broadcast message to all connected clients for a user
   */
  broadcast(userId: string, message: ClientMessage): void {
    const connections = this.userConnections.get(userId);
    if (!connections || connections.size === 0) {
      console.log(`[WebSocket] No active connections for user ${userId}`);
      return;
    }

    const messageStr = JSON.stringify(message);
    let sentCount = 0;

    for (const ws of connections) {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(messageStr);
        sentCount++;
      }
    }

    console.log(`[WebSocket] Broadcasted message to ${sentCount} clients for user ${userId}`);
  }

  /**
   * Broadcast message to clients connected to a specific session
   * Note: This is a placeholder - in a full implementation, we'd track session-specific connections
   */
  broadcastToSession(sessionId: string, message: ClientMessage): void {
    // For now, broadcast to all users and let clients filter
    // In a production system, we'd track which sessions each client is viewing
    console.log(`[WebSocket] Session broadcast not fully implemented, using user broadcast`);
    throw new Error("NotImplementedError: broadcastToSession is not yet implemented");
  }

  /**
   * Send message to Daemon via Centrifugo
   * Publishes to daemon:${deviceId} channel
   */
  async sendToDaemon(deviceId: string, message: Record<string, unknown>): Promise<void> {
    const channel = `daemon:${deviceId}`;

    if (this.state !== ConnectionState.CONNECTED) {
      console.error(`[WebSocket] Cannot send to Daemon: not connected to Centrifugo`);
      throw new Error("WebSocket not connected");
    }

    console.log(`[WebSocket] Sending message to Daemon on ${channel}:`, message);

    try {
      await this.centrifuge.publish(channel, message);
    } catch (error) {
      console.error(`[WebSocket] Failed to publish to ${channel}:`, error);
      throw error;
    }
  }
}

// Singleton instance
let wsServerInstance: WSServerImpl | null = null;

/**
 * Get or create the WebSocket server instance
 * Connection is established lazily on first use
 */
export function getWSServer(): WSServer {
  if (!wsServerInstance) {
    wsServerInstance = new WSServerImpl();
  }
  return wsServerInstance;
}

/**
 * Initialize WebSocket server and connect to Centrifugo
 * Call this on application startup
 */
export async function initWSServer(): Promise<WSServer> {
  const server = getWSServer() as WSServerImpl;
  await server.connect();
  return server;
}

// Legacy API for backwards compatibility
export function createWSServer(): WSServer {
  return getWSServer();
}
