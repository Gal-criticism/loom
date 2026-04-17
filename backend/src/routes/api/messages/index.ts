import { Route } from "@tanstack/start";
import { db } from "~/lib/prisma";
import { getCurrentUser } from "~/lib/auth";
import { getWSServer } from "~/lib/ws";
import { formatUserMessage } from "~/lib/messageHandler";
import { Errors } from "~/lib/errors";
import { jsonSuccess, jsonError } from "~/lib/response";
import { withErrorHandler } from "~/middleware/errorHandler";
import { logger } from "~/lib/logger";
import { createMessageSchema, paginationSchema } from "~/lib/schemas";
import { checkRateLimit, rateLimitConfigs, getRateLimitHeaders } from "~/lib/ratelimit";
import { z } from "zod";

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

      // Rate limiting
      const rateLimit = checkRateLimit(`list_messages:${user.id}`, rateLimitConfigs.read);
      if (!rateLimit.allowed) {
        return jsonError(
          Errors.RATE_LIMITED.withDetails({
            retry_after: rateLimit.retryAfter,
          }),
          { headers: getRateLimitHeaders(rateLimit) }
        );
      }

      const url = new URL(req.url);
      const sessionId = url.searchParams.get("session_id");

      if (!sessionId) {
        return jsonError(Errors.MISSING_SESSION_ID);
      }

      // Validate UUID format
      const uuidResult = z.string().uuid().safeParse(sessionId);
      if (!uuidResult.success) {
        return jsonError(Errors.INVALID_SESSION_ID);
      }

      // Validate pagination
      const paginationResult = paginationSchema.safeParse({
        limit: url.searchParams.get("limit"),
        offset: url.searchParams.get("offset"),
      });

      if (!paginationResult.success) {
        return jsonError(Errors.INVALID_PAGINATION.withDetails({
          issues: paginationResult.error.issues,
        }));
      }

      const { limit, offset } = paginationResult.data;

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

      // Fetch messages with pagination
      const [messages, total] = await Promise.all([
        db.message.findMany({
          where: { sessionId },
          orderBy: { createdAt: "asc" },
          take: limit,
          skip: offset,
        }),
        db.message.count({
          where: { sessionId },
        }),
      ]);

      // Transform to match expected response format
      const transformedMessages = messages.map((message) => ({
        id: message.id,
        session_id: message.sessionId,
        role: message.role,
        content: message.content,
        tools: message.tools,
        created_at: message.createdAt.toISOString(),
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
        },
        { headers: getRateLimitHeaders(rateLimit) }
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

      // Strict rate limiting for message creation
      const rateLimit = checkRateLimit(`send_message:${user.id}`, rateLimitConfigs.messageCreate);
      if (!rateLimit.allowed) {
        return jsonError(
          Errors.RATE_LIMITED.withDetails({
            retry_after: rateLimit.retryAfter,
            message: `Rate limit exceeded. Try again in ${rateLimit.retryAfter} seconds.`,
          }),
          { headers: getRateLimitHeaders(rateLimit) }
        );
      }

      // Parse and validate body
      const body = await req.json().catch(() => null);
      if (!body) {
        return jsonError(Errors.INVALID_INPUT.withDetails({ message: "Invalid JSON body" }));
      }

      const validationResult = createMessageSchema.safeParse(body);
      if (!validationResult.success) {
        return jsonError(
          Errors.INVALID_INPUT.withDetails({
            issues: validationResult.error.issues,
          })
        );
      }

      const { session_id, content } = validationResult.data;

      // Validate session belongs to user (with timeout protection)
      const session = await Promise.race([
        db.session.findFirst({
          where: {
            id: session_id,
            userId: user.id,
          },
        }),
        new Promise<null>((_, reject) =>
          setTimeout(() => reject(new Error("Database query timeout")), 5000)
        ),
      ]).catch((error) => {
        logger.error({ error, userId: user.id, sessionId: session_id }, "Session lookup failed");
        return null;
      });

      if (!session) {
        return jsonError(Errors.SESSION_NOT_FOUND);
      }

      // Save user message and send to Daemon in parallel
      let message;
      try {
        message = await db.message.create({
          data: {
            sessionId: session_id,
            role: "user",
            content: content.trim(),
          },
        });
      } catch (error) {
        logger.error({ error, userId: user.id, sessionId: session_id }, "Failed to save message");
        return jsonError(Errors.DATABASE_ERROR);
      }

      // Transform to match expected response format
      const transformedMessage = {
        id: message.id,
        session_id: message.sessionId,
        role: message.role,
        content: message.content,
        tools: message.tools,
        created_at: message.createdAt.toISOString(),
      };

      // Send to Daemon via WebSocket (non-blocking)
      const daemonMessage = formatUserMessage(session_id, message.id, message.content);

      // Fire and forget with timeout
      Promise.race([
        ws.sendToDaemon(user.device_id, daemonMessage),
        new Promise((_, reject) =>
          setTimeout(() => reject(new Error("WebSocket send timeout")), 5000)
        ),
      ]).catch((error) => {
        logger.error({
          error,
          deviceId: user.device_id,
          messageId: message.id,
        }, "Failed to send message to Daemon");
        // Don't fail the request - message is saved and will be retried
      });

      return jsonSuccess(
        { message: transformedMessage },
        undefined,
        { status: 201, headers: getRateLimitHeaders(rateLimit) }
      );
    });
  },
});
