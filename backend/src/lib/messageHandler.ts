import { db } from "./db";

// Message types from Daemon
export interface DaemonMessage {
  type: "chat:response" | "chat:thinking" | "chat:error";
  data: {
    session_id: string;
    message_id?: string;
    content?: string;
    thinking?: boolean;
    error?: string;
  };
}

// Client message format for broadcasting
export interface ClientMessage {
  type: "chat:response" | "chat:thinking" | "chat:error";
  data: {
    session_id: string;
    message_id?: string;
    content?: string;
    thinking?: boolean;
    error?: string;
    created_at?: string;
  };
}

/**
 * Process incoming Daemon messages
 * - Saves chat:response to database
 * - Returns formatted message for broadcasting to clients
 */
export async function processDaemonMessage(
  deviceId: string,
  message: DaemonMessage
): Promise<ClientMessage | null> {
  const { type, data } = message;

  switch (type) {
    case "chat:response": {
      // Validate required fields
      if (!data.session_id || !data.content) {
        console.error("Invalid chat:response message: missing session_id or content");
        return null;
      }

      try {
        // Save AI response to database
        const result = await db.query(
          `INSERT INTO messages (session_id, role, content)
           VALUES ($1, 'assistant', $2)
           RETURNING id, session_id, role, content, tools, created_at`,
          [data.session_id, data.content]
        );

        const savedMessage = result.rows[0];

        return {
          type: "chat:response",
          data: {
            session_id: data.session_id,
            message_id: savedMessage.id,
            content: savedMessage.content,
            created_at: savedMessage.created_at,
          },
        };
      } catch (error) {
        console.error("Failed to save AI response to database:", error);
        // Return error message to client
        return {
          type: "chat:error",
          data: {
            session_id: data.session_id,
            error: "Failed to save AI response",
          },
        };
      }
    }

    case "chat:thinking": {
      // Validate required fields
      if (!data.session_id) {
        console.error("Invalid chat:thinking message: missing session_id");
        return null;
      }

      // Forward thinking state to clients (not saved to DB)
      return {
        type: "chat:thinking",
        data: {
          session_id: data.session_id,
          thinking: data.thinking ?? true,
        },
      };
    }

    case "chat:error": {
      // Validate required fields
      if (!data.session_id) {
        console.error("Invalid chat:error message: missing session_id");
        return null;
      }

      // Forward error to clients
      return {
        type: "chat:error",
        data: {
          session_id: data.session_id,
          error: data.error || "Unknown error from Daemon",
        },
      };
    }

    default:
      console.warn(`Unknown message type received from Daemon: ${type}`);
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
      content,
    },
  };
}
