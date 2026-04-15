export function BackgroundLayer() {
  return (
    <div
      style={{
        position: "absolute",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        background: "linear-gradient(to bottom, #1a1a2e, #16213e)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <div style={{ opacity: 0.3, fontSize: "14px", color: "#666" }}>
        [GIF/视频背景 - MVP 暂为占位]
      </div>
    </div>
  );
}
