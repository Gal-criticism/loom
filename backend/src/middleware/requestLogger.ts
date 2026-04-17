import { logger, getRequestLogger } from "~/lib/logger";
import { v4 as uuidv4 } from "uuid";

export function withRequestLogging(handler: Function) {
  return async (request: Request) => {
    const requestId = uuidv4();
    const requestLogger = getRequestLogger(requestId);
    const url = new URL(request.url);
    const start = Date.now();

    requestLogger.info({
      method: request.method,
      path: url.pathname,
      query: Object.fromEntries(url.searchParams),
    }, "Request started");

    try {
      const response = await handler(request);
      const duration = Date.now() - start;

      requestLogger.info({
        method: request.method,
        path: url.pathname,
        status: response.status,
        duration,
      }, "Request completed");

      // Add request ID header to response
      response.headers.set("X-Request-ID", requestId);
      return response;
    } catch (error) {
      const duration = Date.now() - start;
      requestLogger.error({
        method: request.method,
        path: url.pathname,
        error: error instanceof Error ? error.message : "Unknown error",
        duration,
      }, "Request failed");
      throw error;
    }
  };
}
