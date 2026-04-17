import { json } from "@tanstack/start";
import { APIError } from "./errors";

export interface SuccessResponse<T> {
  success: true;
  data: T;
  meta?: Record<string, any>;
}

export interface ErrorResponse {
  success: false;
  error: {
    code: string;
    message: string;
    details?: Record<string, any>;
  };
}

export function successResponse<T>(
  data: T,
  meta?: Record<string, any>
): SuccessResponse<T> {
  return {
    success: true,
    data,
    ...(meta && { meta }),
  };
}

export function errorResponse(
  error: APIError,
  details?: Record<string, any>
): ErrorResponse {
  return {
    success: false,
    error: {
      code: error.code,
      message: error.message,
      ...(details && { details }),
      ...(error.details && { details: error.details }),
    },
  };
}

export function createError(
  code: string,
  message: string,
  status: number,
  details?: Record<string, any>
): APIError {
  return new APIError(code, message, status, details);
}

// Helper to wrap JSON responses with consistent format
export function jsonSuccess<T>(
  data: T,
  meta?: Record<string, any>,
  init?: ResponseInit
): Response {
  return json(successResponse(data, meta), init);
}

export function jsonError(
  error: APIError,
  details?: Record<string, any>,
  init?: ResponseInit
): Response {
  return json(errorResponse(error, details), {
    ...init,
    status: error.status,
  });
}
