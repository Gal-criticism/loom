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

const validLogLevels = ["debug", "info", "warn", "error"] as const;
type LogLevel = (typeof validLogLevels)[number];

function validateLogLevel(value: string | undefined): LogLevel {
  if (!value) return "info";
  if (validLogLevels.includes(value as LogLevel)) {
    return value as LogLevel;
  }
  throw new Error(
    `Invalid LOG_LEVEL: "${value}". Must be one of: ${validLogLevels.join(", ")}`
  );
}

function parsePort(value: string | undefined): number {
  const portStr = value || "3000";
  const port = parseInt(portStr, 10);
  if (isNaN(port) || port < 1 || port > 65535) {
    throw new Error(
      `Invalid PORT: "${portStr}". Must be a valid port number (1-65535).`
    );
  }
  return port;
}

function getCentrifugoToken(): string {
  const token = process.env.CENTRIFUGO_TOKEN;
  const env = process.env.NODE_ENV || "development";

  if (!token || token === "dev-token") {
    if (env === "production") {
      throw new Error(
        "CENTRIFUGO_TOKEN must be set to a non-default value in production. " +
          "The default 'dev-token' is insecure and cannot be used in production."
      );
    }
    return "dev-token";
  }

  return token;
}

let parsedConfig: z.infer<typeof configSchema>;

try {
  parsedConfig = configSchema.parse({
    database: { url: process.env.DATABASE_URL },
    centrifugo: {
      url: process.env.CENTRIFUGO_URL || "ws://localhost:8000/connection/websocket",
      token: getCentrifugoToken(),
    },
    app: {
      port: parsePort(process.env.PORT),
      env: process.env.NODE_ENV || "development",
      logLevel: validateLogLevel(process.env.LOG_LEVEL),
    },
  });
} catch (error) {
  if (error instanceof z.ZodError) {
    const issues = error.issues
      .map((issue) => `  - ${issue.path.join(".")}: ${issue.message}`)
      .join("\n");
    throw new Error(`Configuration validation failed:\n${issues}`);
  }
  throw error;
}

export const config = parsedConfig;
export type Config = typeof config;
