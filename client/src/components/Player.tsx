import { useState, useEffect } from "react";
import { audioManager } from "~/lib/audio";

// MVP: 使用免费电台
const DEFAULT_STATIONS = [
  { name: "Lofi Girl", url: "https://play.streamafrica.net/lofiradio" },
  { name: "Chillhop", url: "https://streams.fluxfm.de/Chillhop/mp3-128" },
];

export function Player() {
  const [isPlaying, setIsPlaying] = useState(false);
  const [currentStation, setCurrentStation] = useState(DEFAULT_STATIONS[0]);

  useEffect(() => {
    audioManager.onStateChange((state) => {
      setIsPlaying(state === "playing");
    });
  }, []);

  const togglePlay = () => {
    if (isPlaying) {
      audioManager.pause();
    } else {
      audioManager.play(currentStation.url);
    }
  };

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: "12px",
        padding: "8px 16px",
        background: "rgba(0,0,0,0.3)",
        borderRadius: "24px",
        backdropFilter: "blur(8px)",
      }}
    >
      <button
        onClick={togglePlay}
        style={{
          width: "32px",
          height: "32px",
          borderRadius: "50%",
          border: "none",
          background: "#e94560",
          color: "#fff",
          cursor: "pointer",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        {isPlaying ? "⏸" : "▶"}
      </button>

      <span style={{ fontSize: "14px", minWidth: "80px" }}>
        {currentStation.name}
      </span>

      <span style={{ fontSize: "12px", opacity: 0.6 }}>
        {isPlaying ? "正在播放" : "已暂停"}
      </span>
    </div>
  );
}
