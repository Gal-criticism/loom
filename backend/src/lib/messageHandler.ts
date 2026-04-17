import { db, prisma } from "./prisma";
import { logger } from "./logger";
import { daemonMessageSchema, type DaemonMessage } from "./schemas";
import { z } from "zod";

// Client message format for broadcasting
export interface ClientMessage {
  type: "chat:response" | "chat:thinking" | "chat:error" | "chat:tool_call";
  data: {
    session_id: string;
    message_id?: string;
    content?: string;
    thinking?: boolean;
    error?: string;
    created_at?: string;
    tool_name?: string;
    tool_input?: Record<string, unknown>;
    tool_call_id?: string;
  };
}

/**
 * Validate incoming Daemon message with Zod schema
 * Returns validated message or null if invalid
 */
function validateDaemonMessage(message: unknown): DaemonMessage | null {
  try {
    return daemonMessageSchema.parse(message);
  } catch (error) {
    if (error instanceof z.ZodError) {
      const issues = error.issues.map(i => `${i.path.join('.')}: ${i.message}`).join(', ');
      logger.error({ issues, rawMessage: message }, "Daemon message validation failed");
    } else {
      logger.error({ error, rawMessage: message }, "Unexpected error validating Daemon message");
    }
    return null;
  }
}

/**
 * Process incoming Daemon messages
 * - Validates message format
 * - Saves chat:response to database with transaction
 * - Returns formatted message for broadcasting to clients
 */
export async function processDaemonMessage(
  deviceId: string,
  rawMessage: unknown
): Promise<ClientMessage | null> {
  // Validate message format
  const message = validateDaemonMessage(rawMessage);
  if (!message) {
    return null;
  }

  const { type, data } = message;

  switch (type) {
    case "chat:response": {
      try {
        // Use transaction to ensure consistency
        const savedMessage = await prisma.$transaction(async (tx) => {
          // Verify session exists and belongs to device
          const session = await tx.session.findFirst({
            where: {
              id: data.session_id,
              user: {
                deviceId: deviceId,
              },
            },
          });

          if (!session) {
            throw new Error(`Session ${data.session_id} not found for device ${deviceId}`);
          }

          // Save AI response
          return tx.message.create({
            data: {
              sessionId: data.session_id,
              role: "assistant",
              content: data.content,
              metadata: data.metadata || {},
            },
          });
        }, {
          timeout: 10000, // 10 second timeout
          isolationLevel: "ReadCommitted",
        });

        logger.info({
          messageId: savedMessage.id,
          sessionId: data.session_id,
          deviceId,
        }, "Saved AI response to database");

        return {
          type: "chat:response",
          data: {
            session_id: data.session_id,
            message_id: savedMessage.id,
            content: savedMessage.content,
            created_at: savedMessage.createdAt.toISOString(),
          },
        };
      } catch (error) {
        logger.error({ error, deviceId, sessionId: data.session_id }, "Failed to save AI response");

        return {
          type: "chat:error",
          data: {
            session_id: data.session_id,
            error: "Failed to save AI response. Please try again.",
          },
        };
      }
    }

    case "chat:thinking": {
      logger.debug({
        deviceId,
        sessionId: data.session_id,
        thinking: data.thinking,
      }, "AI thinking state update");

      return {
        type: "chat:thinking",
        data: {
          session_id: data.session_id,
          thinking: data.thinking ?? true,
        },
      };
    }

    case "chat:error": {
      logger.warn({
        deviceId,
        sessionId: data.session_id,
        error: data.error,
      }, "Received error from Daemon");

      return {
        type: "chat:error",
        data: {
          session_id: data.session_id,
          error: data.error || "Unknown error from AI",
        },
      };
    }

    case "chat:tool_call": {
      logger.info({
        deviceId,
        sessionId: data.session_id,
        toolName: data.tool_name,
        toolCallId: data.tool_call_id,
      }, "AI tool call");

      return {
        type: "chat:tool_call",
        data: {
          session_id: data.session_id,
          tool_name: data.tool_name,
          tool_input: data.tool_input,
          tool_call_id: data.tool_call_id,
        },
      };
    }

    default:
      // This should never happen due to Zod validation
      logger.warn({ type, deviceId }, "Unknown message type from Daemon");
      return null;
  }
}

/**
 * Format user message for sending to Daemon
 */
export function formatUserMessage(
  sessionId: string,
  messageId: string,
  content: string
): { type: string; data: Record<string, unknown> } {
  return {
    type: "chat:message",
    data: {
      session_id: sessionId,
      message_id: messageId,
      content: content.slice(0, 100000), // Enforce max size
      timestamp: new Date().toISOString(),
    },
  };
}
