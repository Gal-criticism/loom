import { createStartHandler } from "@tanstack/start";
import { getRouter } from "./router";
import { initWSServer } from "./lib/ws";
import { logger } from "./lib/logger";
import { config } from "./lib/config";
// NOTE: Rate limiting disabled - can be re-enabled by importing from "./lib/ratelimit"
import { startHealthMonitoring } from "./lib/health";

// Initialize WebSocket server on startup
initWSServer().catch((error) => {
  logger.error(error, "Failed to initialize WebSocket server");
  // Don't exit - the server can still handle HTTP requests
});

// Start production-grade services
// NOTE: startRateLimitCleanup() disabled
startHealthMonitoring();

logger.info({
  port: config.app.port,
  env: config.app.env,
  logLevel: config.app.logLevel,
  features: {
    rateLimiting: false, // Disabled
    healthMonitoring: true,
    messageValidation: true,
    transactionSupport: true,
  },
}, "Loom backend starting (production mode)");

export default createStartHandler({
  createRouter: () => getRouter(),
});
