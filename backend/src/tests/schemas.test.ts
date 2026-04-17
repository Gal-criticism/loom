import { describe, it, expect } from "bun:test";
import {
  daemonMessageSchema,
  createMessageSchema,
  paginationSchema,
  createSessionSchema,
  uuidParamSchema,
} from "../lib/schemas";

describe("Schema Validation", () => {
  describe("daemonMessageSchema", () => {
    it("should validate valid chat:response message", () => {
      const message = {
        type: "chat:response" as const,
        data: {
          session_id: "550e8400-e29b-41d4-a716-446655440000",
          content: "Hello, this is a test response",
        },
      };

      const result = daemonMessageSchema.safeParse(message);
      expect(result.success).toBe(true);
    });

    it("should reject invalid UUID in session_id", () => {
      const message = {
        type: "chat:response" as const,
        data: {
          session_id: "invalid-uuid",
          content: "Test content",
        },
      };

      const result = daemonMessageSchema.safeParse(message);
      expect(result.success).toBe(false);
    });

    it("should validate valid chat:thinking message", () => {
      const message = {
        type: "chat:thinking" as const,
        data: {
          session_id: "550e8400-e29b-41d4-a716-446655440000",
          thinking: true,
        },
      };

      const result = daemonMessageSchema.safeParse(message);
      expect(result.success).toBe(true);
    });

    it("should validate valid chat:error message", () => {
      const message = {
        type: "chat:error" as const,
        data: {
          session_id: "550e8400-e29b-41d4-a716-446655440000",
          error: "Something went wrong",
        },
      };

      const result = daemonMessageSchema.safeParse(message);
      expect(result.success).toBe(true);
    });

    it("should reject content exceeding max length", () => {
      const message = {
        type: "chat:response" as const,
        data: {
          session_id: "550e8400-e29b-41d4-a716-446655440000",
          content: "x".repeat(100001),
        },
      };

      const result = daemonMessageSchema.safeParse(message);
      expect(result.success).toBe(false);
    });
  });

  describe("createMessageSchema", () => {
    it("should validate valid create message input", () => {
      const input = {
        session_id: "550e8400-e29b-41d4-a716-446655440000",
        content: "Hello, AI!",
      };

      const result = createMessageSchema.safeParse(input);
      expect(result.success).toBe(true);
    });

    it("should reject empty content", () => {
      const input = {
        session_id: "550e8400-e29b-41d4-a716-446655440000",
        content: "",
      };

      const result = createMessageSchema.safeParse(input);
      expect(result.success).toBe(false);
    });

    it("should reject missing session_id", () => {
      const input = {
        content: "Hello",
      };

      const result = createMessageSchema.safeParse(input);
      expect(result.success).toBe(false);
    });
  });

  describe("paginationSchema", () => {
    it("should use default values when not provided", () => {
      const input = {};

      const result = paginationSchema.safeParse(input);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.limit).toBe(50);
        expect(result.data.offset).toBe(0);
      }
    });

    it("should cap limit at 100", () => {
      const input = { limit: 200 };

      const result = paginationSchema.safeParse(input);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.limit).toBe(100);
      }
    });

    it("should reject negative offset", () => {
      const input = { offset: -1 };

      const result = paginationSchema.safeParse(input);
      expect(result.success).toBe(false);
    });
  });

  describe("createSessionSchema", () => {
    it("should validate with title only", () => {
      const input = { title: "My Session" };

      const result = createSessionSchema.safeParse(input);
      expect(result.success).toBe(true);
    });

    it("should validate with character_id only", () => {
      const input = {
        character_id: "550e8400-e29b-41d4-a716-446655440000",
      };

      const result = createSessionSchema.safeParse(input);
      expect(result.success).toBe(true);
    });

    it("should validate empty object", () => {
      const input = {};

      const result = createSessionSchema.safeParse(input);
      expect(result.success).toBe(true);
    });

    it("should reject title exceeding max length", () => {
      const input = { title: "x".repeat(201) };

      const result = createSessionSchema.safeParse(input);
      expect(result.success).toBe(false);
    });
  });

  describe("uuidParamSchema", () => {
    it("should validate valid UUID", () => {
      const result = uuidParamSchema.safeParse(
        "550e8400-e29b-41d4-a716-446655440000"
      );
      expect(result.success).toBe(true);
    });

    it("should reject invalid UUID", () => {
      const result = uuidParamSchema.safeParse("not-a-uuid");
      expect(result.success).toBe(false);
    });

    it("should reject empty string", () => {
      const result = uuidParamSchema.safeParse("");
      expect(result.success).toBe(false);
    });
  });
});
