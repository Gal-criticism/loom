import { APIError, Errors, mapPrismaError } from "~/lib/errors";
import { jsonError } from "~/lib/response";
import { logger } from "~/lib/logger";

/**
 * Wraps a route handler with centralized error handling.
 * Catches APIError instances and returns standardized error responses.
 * Also handles Prisma errors and unexpected errors.
 */
export async function withErrorHandler<T>(
  handler: () => Promise<T>
): Promise<T> {
  try {
    return await handler();
  } catch (error) {
    // Handle known API errors
    if (error instanceof APIError) {
      return jsonError(error) as unknown as T;
    }

    // Handle Prisma errors
    if (
      error instanceof Error &&
      (error.name === "PrismaClientKnownRequestError" ||
        error.name?.includes("Prisma"))
    ) {
      logger.error(error, "Prisma error");
      const mappedError = mapPrismaError(error);
      return jsonError(mappedError) as unknown as T;
    }

    // Handle unexpected errors
    logger.error(error, "Unhandled error");
    return jsonError(Errors.INTERNAL_ERROR) as unknown as T;
  }
}

/**
 * Higher-order function to wrap route handlers with error handling.
 * Use this when you want to wrap an entire handler function.
 */
export function withErrorHandling<TArgs extends any[], TReturn>(
  handler: (...args: TArgs) => Promise<TReturn>
): (...args: TArgs) => Promise<TReturn> {
  return async (...args: TArgs) => {
    return withErrorHandler(() => handler(...args));
  };
}

/**
 * Asserts that a value exists, throwing an APIError if it doesn't.
 */
export function assertExists<T>(
  value: T | null | undefined,
  error: APIError
): asserts value is T {
  if (value === null || value === undefined) {
    throw error;
  }
}

/**
 * Asserts that a condition is true, throwing an APIError if it's not.
 */
export function assertTrue(
  condition: boolean,
  error: APIError
): asserts condition is true {
  if (!condition) {
    throw error;
  }
}
