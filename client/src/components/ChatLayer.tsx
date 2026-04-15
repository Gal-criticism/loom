import { Chat } from "./Chat";

export function ChatLayer() {
  return (
    <div
      style={{
        position: "fixed",
        bottom: "20px",
        left: "20px",
        right: "20px",
        maxWidth: "600px",
        margin: "0 auto",
        zIndex: 50,
      }}
    >
      <Chat />
    </div>
  );
}
