import { Route } from "@tanstack/start";
import { db } from "~/lib/prisma";
import { getCurrentUser } from "~/lib/auth";
import { Errors } from "~/lib/errors";
import { jsonSuccess, jsonError } from "~/lib/response";
import { withErrorHandler } from "~/middleware/errorHandler";
import { createSessionSchema } from "~/lib/schemas";
import { checkRateLimit, rateLimitConfigs, getRateLimitHeaders } from "~/lib/ratelimit";
import { logger } from "~/lib/logger";
import { z } from "zod";

// GET /api/sessions - List all sessions for current user
export const listSessionsRoute = new Route({
  path: "/api/sessions",
  method: "GET",
  handler: async (req: Request) => {
    return withErrorHandler(async () => {
      const user = await getCurrentUser(req);

      if (!user) {
        return jsonError(Errors.UNAUTHORIZED);
      }

      // Rate limiting
      const rateLimit = checkRateLimit(`list_sessions:${user.id}`, rateLimitConfigs.read);
      if (!rateLimit.allowed) {
        return jsonError(
          Errors.RATE_LIMITED.withDetails({ retry_after: rateLimit.retryAfter }),
          { headers: getRateLimitHeaders(rateLimit) }
        );
      }

      const sessions = await db.session.findMany({
        where: { userId: user.id },
        include: {
          character: {
            select: {
              name: true,
              avatarUrl: true,
            },
          },
          _count: {
            select: { messages: true },
          },
        },
        orderBy: { updatedAt: "desc" },
      });

      // Transform to match expected response format
      const transformedSessions = sessions.map((session) => ({
        id: session.id,
        user_id: session.userId,
        character_id: session.characterId,
        title: session.title,
        created_at: session.createdAt,
        updated_at: session.updatedAt,
        character_name: session.character?.name,
        character_avatar: session.character?.avatarUrl,
        message_count: session._count.messages,
      }));

      return jsonSuccess(
        { sessions: transformedSessions },
        undefined,
        { headers: getRateLimitHeaders(rateLimit) }
      );
    });
  },
});

// POST /api/sessions - Create new session
export const createSessionRoute = new Route({
  path: "/api/sessions",
  method: "POST",
  handler: async (req: Request) => {
    return withErrorHandler(async () => {
      const user = await getCurrentUser(req);

      if (!user) {
        return jsonError(Errors.UNAUTHORIZED);
      }

      // Rate limiting
      const rateLimit = checkRateLimit(`create_session:${user.id}`, rateLimitConfigs.session);
      if (!rateLimit.allowed) {
        return jsonError(
          Errors.RATE_LIMITED.withDetails({ retry_after: rateLimit.retryAfter }),
          { headers: getRateLimitHeaders(rateLimit) }
        );
      }

      // Parse and validate body
      const body = await req.json().catch(() => null);
      if (!body) {
        return jsonError(Errors.INVALID_INPUT.withDetails({ message: "Invalid JSON body" }));
      }

      const validationResult = createSessionSchema.safeParse(body);
      if (!validationResult.success) {
        return jsonError(
          Errors.INVALID_INPUT.withDetails({
            issues: validationResult.error.issues,
          })
        );
      }

      const { title, character_id } = validationResult.data;

      // Validate character_id if provided
      if (character_id) {
        const character = await db.character.findFirst({
          where: {
            id: character_id,
            OR: [{ userId: user.id }, { userId: null }],
          },
        });

        if (!character) {
          return jsonError(Errors.CHARACTER_NOT_FOUND);
        }
      }

      const session = await db.session.create({
        data: {
          userId: user.id,
          characterId: character_id || null,
          title: title || null,
        },
      });

      logger.info({
        sessionId: session.id,
        userId: user.id,
        characterId: character_id,
      }, "Session created");

      // Transform to match expected response format
      const transformedSession = {
        id: session.id,
        user_id: session.userId,
        character_id: session.characterId,
        title: session.title,
        created_at: session.createdAt,
        updated_at: session.updatedAt,
      };

      return jsonSuccess(
        { session: transformedSession },
        undefined,
        { status: 201, headers: getRateLimitHeaders(rateLimit) }
      );
    });
  },
});
