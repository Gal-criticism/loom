import { Centrifuge } from "centrifuge";

export interface WSServer {
  handleConnection(ws: WebSocket, deviceId: string): void;
  broadcast(userId: string, message: any): void;
  sendToDaemon(deviceId: string, message: any): void;
}

export function createWSServer(): WSServer {
  const centrifuge = new Centrifuge("ws://localhost:8000", {
    token: process.env.CENTRIFUGO_TOKEN || "dev-token",
  });

  return {
    handleConnection(ws: WebSocket, deviceId: string) {
      // 注册设备到 Daemon 频道
      const channel = `daemon:${deviceId}`;
      centrifuge.subscribe(channel, {
        message: (ctx: any) => {
          ws.send(JSON.stringify(ctx.data));
        },
      });
    },

    broadcast(userId: string, message: any) {
      centrifuge.publish(`user:${userId}`, message);
    },

    sendToDaemon(deviceId: string, message: any) {
      centrifuge.publish(`daemon:${deviceId}`, message);
    },
  };
}
