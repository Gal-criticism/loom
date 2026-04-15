import { Player } from "./Player";

export function ControlLayer() {
  return (
    <div
      style={{
        position: "fixed",
        top: "20px",
        left: "20px",
        right: "20px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        zIndex: 100,
      }}
    >
      <Player />

      <button style={{ padding: "8px 16px", borderRadius: "8px", border: "none", background: "#2a2a4e", color: "#fff", cursor: "pointer" }}>
        ⚙️ 设置
      </button>
    </div>
  );
}
