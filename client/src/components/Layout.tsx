import { ReactNode } from "react";
import { BackgroundLayer } from "./BackgroundLayer";
import { CharacterLayer } from "./CharacterLayer";
import { ChatLayer } from "./ChatLayer";
import { ControlLayer } from "./ControlLayer";
import { ConnectionStatus } from "./ConnectionStatus";

interface LayoutProps {
  children?: ReactNode;
}

export function Layout({ children }: LayoutProps) {
  return (
    <div style={{ position: "relative", width: "100vw", height: "100vh", overflow: "hidden" }}>
      {/* Layer 0: 连接状态指示器 */}
      <ConnectionStatus />

      {/* Layer 1: 背景 */}
      <BackgroundLayer />

      {/* Layer 2: 角色 */}
      <CharacterLayer />

      {/* Layer 3: 对话 */}
      <ChatLayer />

      {/* Layer 4: 控制 */}
      <ControlLayer />

      {children}
    </div>
  );
}
