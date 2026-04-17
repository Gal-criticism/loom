/**
 * Production-grade message schemas with Zod validation
 * All external messages must be validated before processing
 */

import { z } from "zod";

// Base message schema with strict validation
export const daemonMessageSchema = z.union([
  z.object({
    type: z.literal("chat:response"),
    data: z.object({
      session_id: z.string().uuid({ message: "session_id must be a valid UUID" }),
      message_id: z.string().uuid().optional(),
      content: z.string().min(1).max(100000, { message: "Content too large (max 100KB)" }),
      metadata: z.record(z.unknown()).optional(),
    }).strict(),
  }),
  z.object({
    type: z.literal("chat:thinking"),
    data: z.object({
      session_id: z.string().uuid(),
      thinking: z.boolean().default(true),
      text: z.string().max(1000).optional(), // Thinking text preview
    }).strict(),
  }),
  z.object({
    type: z.literal("chat:error"),
    data: z.object({
      session_id: z.string().uuid(),
      error: z.string().min(1).max(1000),
      code: z.string().max(100).optional(),
    }).strict(),
  }),
  z.object({
    type: z.literal("chat:tool_call"),
    data: z.object({
      session_id: z.string().uuid(),
      tool_name: z.string().min(1).max(100),
      tool_input: z.record(z.unknown()),
      tool_call_id: z.string().min(1).max(100),
    }).strict(),
  }),
]);

export type DaemonMessage = z.infer<typeof daemonMessageSchema>;

// Client → Backend message validation
export const createMessageSchema = z.object({
  session_id: z.string().uuid(),
  content: z.string()
    .min(1, { message: "Content cannot be empty" })
    .max(100000, { message: "Content exceeds maximum length of 100KB" }),
  metadata: z.record(z.unknown()).optional(),
}).strict();

export type CreateMessageInput = z.infer<typeof createMessageSchema>;

// Pagination schema
export const paginationSchema = z.object({
  limit: z.coerce.number().int().min(1).max(100).default(50),
  offset: z.coerce.number().int().min(0).default(0),
});

export type PaginationInput = z.infer<typeof paginationSchema>;

// Session creation schema
export const createSessionSchema = z.object({
  title: z.string().max(200).optional(),
  character_id: z.string().uuid().optional(),
}).strict();

export type CreateSessionInput = z.infer<typeof createSessionSchema>;

// UUID param schema
export const uuidParamSchema = z.string().uuid();
