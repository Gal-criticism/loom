import { Route, json } from "@tanstack/start";
import { getHealthStatus, isAlive, isReady } from "~/lib/health";
import { logger } from "~/lib/logger";

// GET /api/health - Liveness probe (Kubernetes/Docker)
export const healthLiveRoute = new Route({
  path: "/api/health/live",
  method: "GET",
  handler: async () => {
    if (isAlive()) {
      return json({ status: "alive" });
    }
    return json({ status: "dead" }, { status: 503 });
  },
});

// GET /api/health/ready - Readiness probe (Kubernetes/Docker)
export const healthReadyRoute = new Route({
  path: "/api/health/ready",
  method: "GET",
  handler: async () => {
    const ready = await isReady();
    if (ready) {
      return json({ status: "ready" });
    }
    return json({ status: "not ready" }, { status: 503 });
  },
});

// GET /api/health - Comprehensive health check
export const healthRoute = new Route({
  path: "/api/health",
  method: "GET",
  handler: async () => {
    try {
      const health = await getHealthStatus();
      const statusCode = health.status === "unhealthy" ? 503 : 200;
      return json(health, { status: statusCode });
    } catch (error) {
      logger.error(error, "Health check failed");
      return json(
        {
          status: "unhealthy",
          timestamp: new Date().toISOString(),
          error: "Health check failed",
        },
        { status: 503 }
      );
    }
  },
});
