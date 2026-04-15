export function CharacterLayer() {
  return (
    <div
      style={{
        position: "absolute",
        top: "50%",
        left: "50%",
        transform: "translate(-50%, -50%)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        flexDirection: "column",
      }}
    >
      <div
        style={{
          width: "120px",
          height: "120px",
          borderRadius: "50%",
          background: "#2a2a4e",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontSize: "12px",
          color: "#666",
        }}
      >
        [角色占位]
      </div>
    </div>
  );
}
