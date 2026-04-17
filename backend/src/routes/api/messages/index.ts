import { Route } from "@tanstack/start";
import { db } from "~/lib/prisma";
import { getCurrentUser } from "~/lib/auth";
import { getWSServer } from "~/lib/ws";
import { formatUserMessage } from "~/lib/messageHandler";
import { Errors } from "~/lib/errors";
import { jsonSuccess, jsonError } from "~/lib/response";
import { withErrorHandler } from "~/middleware/errorHandler";

const ws = getWSServer();

// GET /api/messages?session_id=xxx - List messages for a session with pagination
export const listMessagesRoute = new Route({
  path: "/api/messages",
  method: "GET",
  handler: async (req: Request) => {
    return withErrorHandler(async () => {
      const user = await getCurrentUser(req);

      if (!user) {
        return jsonError(Errors.UNAUTHORIZED);
      }

      const url = new URL(req.url);
      const sessionId = url.searchParams.get("session_id");

      if (!sessionId) {
        return jsonError(Errors.MISSING_SESSION_ID);
      }

      // Validate session belongs to user
      const session = await db.session.findFirst({
        where: {
          id: sessionId,
          userId: user.id,
        },
      });

      if (!session) {
        return jsonError(Errors.SESSION_NOT_FOUND);
      }

      // Parse pagination parameters
      const limit = Math.min(parseInt(url.searchParams.get("limit") || "50", 10), 100);
      const offset = parseInt(url.searchParams.get("offset") || "0", 10);

      // Validate pagination parameters
      if (isNaN(limit) || limit < 0) {
        return jsonError(Errors.INVALID_PAGINATION.withDetails({ field: "limit" }));
      }

      if (isNaN(offset) || offset < 0) {
        return jsonError(Errors.INVALID_PAGINATION.withDetails({ field: "offset" }));
      }

      const messages = await db.message.findMany({
        where: { sessionId },
        orderBy: { createdAt: "asc" },
        take: limit,
        skip: offset,
      });

      // Get total count for pagination metadata
      const total = await db.message.count({
        where: { sessionId },
      });

      // Transform to match expected response format
      const transformedMessages = messages.map((message) => ({
        id: message.id,
        session_id: message.sessionId,
        role: message.role,
        content: message.content,
        tools: message.tools,
        created_at: message.createdAt,
      }));

      return jsonSuccess(
        { messages: transformedMessages },
        {
          pagination: {
            total,
            limit,
            offset,
            has_more: offset + messages.length < total,
          },
        }
      );
    });
  },
});

// POST /api/messages - Send a new message in a session
export const sendMessageRoute = new Route({
  path: "/api/messages",
  method: "POST",
  handler: async (req: Request) => {
    return withErrorHandler(async () => {
      const user = await getCurrentUser(req);

      if (!user) {
        return jsonError(Errors.UNAUTHORIZED);
      }

      const body = await req.json();
      const { session_id, content } = body;

      // Validate session_id
      if (!session_id) {
        return jsonError(Errors.MISSING_SESSION_ID);
      }

      if (typeof session_id !== "string") {
        return jsonError(Errors.INVALID_SESSION_ID);
      }

      // Validate content
      if (!content) {
        return jsonError(Errors.MISSING_CONTENT);
      }

      if (typeof content !== "string" || content.trim().length === 0) {
        return jsonError(Errors.INVALID_CONTENT);
      }

      if (content.length > 100000) {
        return jsonError(Errors.CONTENT_TOO_LONG);
      }

      // Validate session belongs to user
      const session = await db.session.findFirst({
        where: {
          id: session_id,
          userId: user.id,
        },
      });

      if (!session) {
        return jsonError(Errors.SESSION_NOT_FOUND);
      }

      // Save user message
      const message = await db.message.create({
        data: {
          sessionId: session_id,
          role: "user",
          content: content.trim(),
        },
      });

      // Transform to match expected response format
      const transformedMessage = {
        id: message.id,
        session_id: message.sessionId,
        role: message.role,
        content: message.content,
        tools: message.tools,
        created_at: message.createdAt,
      };

      // Send to Daemon via WebSocket
      try {
        const daemonMessage = formatUserMessage(session_id, message.id, message.content);
        ws.sendToDaemon(user.device_id, daemonMessage);
      } catch (error) {
        console.error("Failed to send message to daemon:", error);
        // Don't fail the request if WebSocket fails - message is already saved
      }

      return jsonSuccess({ message: transformedMessage }, undefined, { status: 201 });
    });
  },
});
