import { createStartHandler } from "@tanstack/start";
import { getRouter } from "./router";
import { initWSServer } from "./lib/ws";
import { logger } from "./lib/logger";
import { config } from "./lib/config";

// Initialize WebSocket server on startup
initWSServer().catch((error) => {
  logger.error(error, "Failed to initialize WebSocket server");
  // Don't exit - the server can still handle HTTP requests
});

logger.info({
  port: config.app.port,
  env: config.app.env,
  logLevel: config.app.logLevel,
}, "Loom backend starting");

export default createStartHandler({
  createRouter: () => getRouter(),
});
