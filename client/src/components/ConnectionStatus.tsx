import { useState, useEffect } from "react";
import { centrifugo } from "~/lib/centrifugo";

type ConnectionState = "disconnected" | "connecting" | "connected" | "reconnecting";

export function ConnectionStatus() {
  const [state, setState] = useState<ConnectionState>("disconnected");
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    // Listen to state changes
    const unsubscribe = centrifugo.onStateChange((newState) => {
      setState(newState);
      // Show indicator when not connected
      setIsVisible(newState !== "connected");
    });

    // Auto-connect on mount
    centrifugo.connect().catch(console.error);

    return () => unsubscribe();
  }, []);

  if (!isVisible) return null;

  const config = {
    disconnected: { text: "未连接", color: "#ef4444", bg: "rgba(239, 68, 68, 0.2)" },
    connecting: { text: "连接中...", color: "#facc15", bg: "rgba(250, 204, 21, 0.2)" },
    reconnecting: { text: "重连中...", color: "#f97316", bg: "rgba(249, 115, 22, 0.2)" },
    connected: { text: "已连接", color: "#4ade80", bg: "rgba(74, 222, 128, 0.2)" },
  }[state];

  return (
    <div
      style={{
        position: "fixed",
        top: "20px",
        right: "20px",
        zIndex: 100,
        padding: "8px 16px",
        borderRadius: "20px",
        background: config.bg,
        border: `1px solid ${config.color}`,
        color: config.color,
        fontSize: "12px",
        display: "flex",
        alignItems: "center",
        gap: "8px",
        backdropFilter: "blur(8px)",
      }}
    >
      <span
        style={{
          width: "8px",
          height: "8px",
          borderRadius: "50%",
          background: config.color,
          animation: state === "connecting" || state === "reconnecting" ? "pulse 1.5s infinite" : undefined,
        }}
      />
      {config.text}
      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.3; }
        }
      `}</style>
    </div>
  );
}
