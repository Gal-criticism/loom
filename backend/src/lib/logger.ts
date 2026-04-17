import pino from "pino";
import { config } from "./config";

export const logger = pino({
  level: config.app.logLevel,
  transport: config.app.env === "development"
    ? { target: "pino-pretty", options: { colorize: true } }
    : undefined,
  base: {
    service: "loom-backend",
  },
});

// Request context logger
export function getRequestLogger(requestId: string) {
  return logger.child({ requestId });
}
