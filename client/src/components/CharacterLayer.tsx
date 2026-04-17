export function CharacterLayer() {
  return (
    <div
      style={{
        position: "absolute",
        inset: 0,
        zIndex: 10,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        pointerEvents: "none",
      }}
    >
      {/* Character avatar */}
      <div
        style={{
          width: "200px",
          height: "200px",
          borderRadius: "50%",
          background: "linear-gradient(135deg, #e94560 0%, #533483 100%)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontSize: "80px",
          boxShadow: "0 20px 60px rgba(233, 69, 96, 0.3)",
          animation: "float 6s ease-in-out infinite",
        }}
      >
        🤖
      </div>

      <style>{`
        @keyframes float {
          0%, 100% {
            transform: translateY(0px);
          }
          50% {
            transform: translateY(-20px);
          }
        }
      `}</style>
    </div>
  );
}
