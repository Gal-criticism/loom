import { describe, it, expect } from "bun:test";
import { APIError, Errors, mapPrismaError } from "../lib/errors";

describe("Error Handling", () => {
  describe("APIError", () => {
    it("should create error with all properties", () => {
      const error = new APIError(
        "TEST_ERROR",
        "Test error message",
        400,
        { field: "value" }
      );

      expect(error.code).toBe("TEST_ERROR");
      expect(error.message).toBe("Test error message");
      expect(error.status).toBe(400);
      expect(error.details).toEqual({ field: "value" });
    });

    it("should create error without details", () => {
      const error = new APIError("TEST_ERROR", "Test message", 500);

      expect(error.code).toBe("TEST_ERROR");
      expect(error.details).toBeUndefined();
    });

    it("should support withDetails method", () => {
      const baseError = new APIError("BASE_ERROR", "Base message", 400);
      const extendedError = baseError.withDetails({ extra: "info" });

      expect(extendedError.code).toBe("BASE_ERROR");
      expect(extendedError.details).toEqual({ extra: "info" });
      // Original error should not be modified
      expect(baseError.details).toBeUndefined();
    });
  });

  describe("Predefined Errors", () => {
    it("should have UNAUTHORIZED error", () => {
      expect(Errors.UNAUTHORIZED.code).toBe("UNAUTHORIZED");
      expect(Errors.UNAUTHORIZED.status).toBe(401);
    });

    it("should have SESSION_NOT_FOUND error", () => {
      expect(Errors.SESSION_NOT_FOUND.code).toBe("SESSION_NOT_FOUND");
      expect(Errors.SESSION_NOT_FOUND.status).toBe(404);
    });

    it("should have RATE_LIMITED error", () => {
      expect(Errors.RATE_LIMITED.code).toBe("RATE_LIMITED");
      expect(Errors.RATE_LIMITED.status).toBe(429);
    });

    it("should have DATABASE_ERROR error", () => {
      expect(Errors.DATABASE_ERROR.code).toBe("DATABASE_ERROR");
      expect(Errors.DATABASE_ERROR.status).toBe(500);
    });
  });

  describe("mapPrismaError", () => {
    it("should map P2002 unique constraint violation", () => {
      const prismaError = {
        code: "P2002",
        meta: { target: ["email"] },
      };

      const error = mapPrismaError(prismaError);
      expect(error.code).toBe("UNIQUE_CONSTRAINT_VIOLATION");
      expect(error.status).toBe(409);
    });

    it("should map P2025 record not found", () => {
      const prismaError = {
        code: "P2025",
        meta: { cause: "Record not found" },
      };

      const error = mapPrismaError(prismaError);
      expect(error.code).toBe("RECORD_NOT_FOUND");
      expect(error.status).toBe(404);
    });

    it("should map P2003 foreign key constraint", () => {
      const prismaError = {
        code: "P2003",
        meta: { field_name: "user_id" },
      };

      const error = mapPrismaError(prismaError);
      expect(error.code).toBe("FOREIGN_KEY_CONSTRAINT_VIOLATION");
      expect(error.status).toBe(400);
    });

    it("should return DATABASE_ERROR for unknown errors", () => {
      const prismaError = { code: "P9999" };

      const error = mapPrismaError(prismaError);
      expect(error.code).toBe("DATABASE_ERROR");
    });

    it("should handle null/undefined error", () => {
      const error = mapPrismaError(null);
      expect(error.code).toBe("DATABASE_ERROR");
    });
  });
});
