import { ReactNode } from "react";

interface LayerProps {
  children: ReactNode;
  zIndex: number;
  visible?: boolean;
}

export function Layer({ children, zIndex, visible = true }: LayerProps) {
  if (!visible) return null;

  return (
    <div
      style={{
        position: "absolute",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        zIndex,
        pointerEvents: visible ? "auto" : "none",
      }}
    >
      {children}
    </div>
  );
}
