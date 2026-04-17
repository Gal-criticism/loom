import { z } from "zod";

const configSchema = z.object({
  database: z.object({
    url: z.string().url(),
  }),
  centrifugo: z.object({
    url: z.string().url(),
    token: z.string().min(1),
  }),
  app: z.object({
    port: z.number().default(3000),
    env: z.enum(["development", "production", "test"]).default("development"),
    logLevel: z.enum(["debug", "info", "warn", "error"]).default("info"),
  }),
});

export const config = configSchema.parse({
  database: { url: process.env.DATABASE_URL },
  centrifugo: {
    url: process.env.CENTRIFUGO_URL || "ws://localhost:8000/connection/websocket",
    token: process.env.CENTRIFUGO_TOKEN || "dev-token",
  },
  app: {
    port: parseInt(process.env.PORT || "3000"),
    env: process.env.NODE_ENV || "development",
    logLevel: (process.env.LOG_LEVEL as any) || "info",
  },
});

export type Config = typeof config;
