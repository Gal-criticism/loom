export class APIError extends Error {
  constructor(
    public code: string,
    public message: string,
    public status: number,
    public details?: Record<string, any>
  ) {
    super(message);
    this.name = "APIError";
  }

  withDetails(details: Record<string, any>): APIError {
    return new APIError(this.code, this.message, this.status, details);
  }
}

// Common error codes
export const Errors = {
  // Authentication & Authorization
  UNAUTHORIZED: new APIError("UNAUTHORIZED", "Unauthorized", 401),
  FORBIDDEN: new APIError("FORBIDDEN", "Forbidden", 403),

  // Resource Not Found
  SESSION_NOT_FOUND: new APIError(
    "SESSION_NOT_FOUND",
    "Session not found",
    404
  ),
  MESSAGE_NOT_FOUND: new APIError(
    "MESSAGE_NOT_FOUND",
    "Message not found",
    404
  ),
  CHARACTER_NOT_FOUND: new APIError(
    "CHARACTER_NOT_FOUND",
    "Character not found",
    404
  ),
  USER_NOT_FOUND: new APIError("USER_NOT_FOUND", "User not found", 404),
  DEVICE_NOT_FOUND: new APIError("DEVICE_NOT_FOUND", "Device not found", 404),

  // Validation Errors
  INVALID_INPUT: new APIError("INVALID_INPUT", "Invalid input", 400),
  INVALID_TITLE: new APIError("INVALID_TITLE", "Invalid title", 400),
  INVALID_CHARACTER_ID: new APIError(
    "INVALID_CHARACTER_ID",
    "Invalid character_id",
    400
  ),
  INVALID_SESSION_ID: new APIError(
    "INVALID_SESSION_ID",
    "Invalid session_id",
    400
  ),
  INVALID_CONTENT: new APIError("INVALID_CONTENT", "Invalid content", 400),
  INVALID_PAGINATION: new APIError(
    "INVALID_PAGINATION",
    "Invalid pagination parameters",
    400
  ),

  // Missing Required Fields
  MISSING_SESSION_ID: new APIError(
    "MISSING_SESSION_ID",
    "session_id is required",
    400
  ),
  MISSING_CONTENT: new APIError(
    "MISSING_CONTENT",
    "content is required",
    400
  ),
  CONTENT_TOO_LONG: new APIError(
    "CONTENT_TOO_LONG",
    "Content exceeds maximum length of 100000 characters",
    400
  ),

  // Service Errors
  DAEMON_OFFLINE: new APIError(
    "DAEMON_OFFLINE",
    "Daemon is offline",
    503
  ),
  DATABASE_ERROR: new APIError(
    "DATABASE_ERROR",
    "Database operation failed",
    500
  ),

  // Internal Errors
  INTERNAL_ERROR: new APIError(
    "INTERNAL_ERROR",
    "Internal server error",
    500
  ),
} as const;

// Prisma error code mapping
export function mapPrismaError(error: any): APIError {
  // Prisma error codes: https://www.prisma.io/docs/reference/api-reference/error-reference
  switch (error?.code) {
    case "P2002":
      return new APIError(
        "UNIQUE_CONSTRAINT_VIOLATION",
        "A record with this value already exists",
        409,
        { target: error.meta?.target }
      );
    case "P2025":
      return new APIError(
        "RECORD_NOT_FOUND",
        "Record not found",
        404,
        { cause: error.meta?.cause }
      );
    case "P2003":
      return new APIError(
        "FOREIGN_KEY_CONSTRAINT_VIOLATION",
        "Referenced record does not exist",
        400,
        { field_name: error.meta?.field_name }
      );
    default:
      console.error("Unhandled Prisma error:", error);
      return Errors.DATABASE_ERROR;
  }
}
