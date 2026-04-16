import { createStartHandler } from "@tanstack/start";
import { getRouter } from "./router";
import { initWSServer } from "./lib/ws";

// Initialize WebSocket server on startup
initWSServer().catch((error) => {
  console.error("Failed to initialize WebSocket server:", error);
  // Don't exit - the server can still handle HTTP requests
});

export default createStartHandler({
  createRouter: () => getRouter(),
});
