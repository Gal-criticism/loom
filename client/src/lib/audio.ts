type PlayState = "playing" | "paused" | "stopped";

interface AudioManager {
  play(url: string): void;
  pause(): void;
  resume(): void;
  stop(): void;
  setVolume(volume: number): void;
  getState(): PlayState;
  onStateChange(callback: (state: PlayState) => void): void;
}

export function createAudioManager(): AudioManager {
  let audio: HTMLAudioElement | null = null;
  let state: PlayState = "stopped";
  let stateCallbacks: ((state: PlayState) => void)[] = [];

  const notifyState = () => {
    stateCallbacks.forEach((cb) => cb(state));
  };

  return {
    play(url: string) {
      if (audio) {
        audio.pause();
      }
      audio = new Audio(url);
      audio.loop = true;
      audio.play().then(() => {
        state = "playing";
        notifyState();
      }).catch(console.error);
    },
    pause() {
      audio?.pause();
      state = "paused";
      notifyState();
    },
    resume() {
      audio?.play();
      state = "playing";
      notifyState();
    },
    stop() {
      if (audio) {
        audio.pause();
        audio.currentTime = 0;
      }
      state = "stopped";
      notifyState();
    },
    setVolume(volume: number) {
      if (audio) {
        audio.volume = Math.max(0, Math.min(1, volume));
      }
    },
    getState() {
      return state;
    },
    onStateChange(callback: (state: PlayState) => void) {
      stateCallbacks.push(callback);
    },
  };
}

export const audioManager = createAudioManager();
