/**
 * Production-grade health checks
 * Monitors all critical dependencies
 */

import { prisma } from "./prisma";
import { logger } from "./logger";
import { getWSServer, ConnectionState } from "./ws";

export interface HealthStatus {
  status: "healthy" | "degraded" | "unhealthy";
  timestamp: string;
  version: string;
  checks: {
    database: ComponentHealth;
    websocket: ComponentHealth;
    memory: ComponentHealth;
  };
}

interface ComponentHealth {
  status: "up" | "down" | "degraded";
  responseTime?: number;
  message?: string;
  details?: Record<string, unknown>;
}

// Health check configuration
const HEALTH_CHECK_TIMEOUT = 5000; // 5 seconds
const MEMORY_THRESHOLD_PERCENT = 90;

/**
 * Check database health
 */
async function checkDatabase(): Promise<ComponentHealth> {
  const start = Date.now();

  try {
    // Perform lightweight query
    await prisma.$queryRaw`SELECT 1`;

    const responseTime = Date.now() - start;

    return {
      status: "up",
      responseTime,
      details: { query: "SELECT 1" },
    };
  } catch (error) {
    logger.error(error, "Database health check failed");

    return {
      status: "down",
      message: error instanceof Error ? error.message : "Database connection failed",
    };
  }
}

/**
 * Check WebSocket connection health
 */
function checkWebSocket(): ComponentHealth {
  const ws = getWSServer();
  const state = ws.getState();

  switch (state) {
    case ConnectionState.CONNECTED:
      return {
        status: "up",
        details: { state },
      };
    case ConnectionState.CONNECTING:
    case ConnectionState.RECONNECTING:
      return {
        status: "degraded",
        message: `WebSocket is ${state}`,
        details: { state },
      };
    default:
      return {
        status: "down",
        message: "WebSocket is disconnected",
        details: { state },
      };
  }
}

/**
 * Check memory usage
 */
function checkMemory(): ComponentHealth {
  const usage = process.memoryUsage();
  const heapUsedPercent = (usage.heapUsed / usage.heapTotal) * 100;

  const details = {
    heapUsed: `${Math.round(usage.heapUsed / 1024 / 1024)}MB`,
    heapTotal: `${Math.round(usage.heapTotal / 1024 / 1024)}MB`,
    external: `${Math.round(usage.external / 1024 / 1024)}MB`,
    rss: `${Math.round(usage.rss / 1024 / 1024)}MB`,
    heapUsedPercent: Math.round(heapUsedPercent),
  };

  if (heapUsedPercent > MEMORY_THRESHOLD_PERCENT) {
    logger.warn(details, "High memory usage detected");

    return {
      status: "degraded",
      message: `Memory usage at ${Math.round(heapUsedPercent)}%`,
      details,
    };
  }

  return {
    status: "up",
    details,
  };
}

/**
 * Perform comprehensive health check
 */
export async function getHealthStatus(): Promise<HealthStatus> {
  const [database, websocket, memory] = await Promise.all([
    checkDatabase(),
    Promise.resolve(checkWebSocket()),
    Promise.resolve(checkMemory()),
  ]);

  // Determine overall status
  const checks = { database, websocket, memory };
  const statuses = Object.values(checks).map((c) => c.status);

  let status: HealthStatus["status"] = "healthy";
  if (statuses.includes("down")) {
    status = "unhealthy";
  } else if (statuses.includes("degraded")) {
    status = "degraded";
  }

  return {
    status,
    timestamp: new Date().toISOString(),
    version: process.env.npm_package_version || "unknown",
    checks,
  };
}

/**
 * Quick liveness check (for Kubernetes/Docker)
 */
export function isAlive(): boolean {
  return true; // If we can respond, we're alive
}

/**
 * Readiness check (for Kubernetes/Docker)
 */
export async function isReady(): Promise<boolean> {
  const health = await getHealthStatus();
  return health.status !== "unhealthy";
}

/**
 * Start periodic health monitoring
 */
export function startHealthMonitoring(): void {
  const CHECK_INTERVAL = 30 * 1000; // 30 seconds

  setInterval(async () => {
    const health = await getHealthStatus();

    if (health.status !== "healthy") {
      logger.warn({
        status: health.status,
        checks: health.checks,
      }, "Health check warning");
    }
  }, CHECK_INTERVAL);
}
