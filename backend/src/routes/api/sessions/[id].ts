import { Route } from "@tanstack/start";
import { db } from "~/lib/prisma";
import { getCurrentUser } from "~/lib/auth";
import { Errors } from "~/lib/errors";
import { jsonSuccess, jsonError } from "~/lib/response";
import { withErrorHandler } from "~/middleware/errorHandler";
import { createSessionSchema, uuidParamSchema } from "~/lib/schemas";
import { checkRateLimit, rateLimitConfigs, getRateLimitHeaders } from "~/lib/ratelimit";
import { logger } from "~/lib/logger";
import { z } from "zod";

// GET /api/sessions/:id - Get single session details
export const getSessionRoute = new Route({
  path: "/api/sessions/$id",
  method: "GET",
  handler: async ({ request, params }: { request: Request; params: { id: string } }) => {
    return withErrorHandler(async () => {
      const user = await getCurrentUser(request);

      if (!user) {
        return jsonError(Errors.UNAUTHORIZED);
      }

      // Rate limiting
      const rateLimit = checkRateLimit(`get_session:${user.id}`, rateLimitConfigs.read);
      if (!rateLimit.allowed) {
        return jsonError(
          Errors.RATE_LIMITED.withDetails({ retry_after: rateLimit.retryAfter }),
          { headers: getRateLimitHeaders(rateLimit) }
        );
      }

      // Validate UUID
      const idResult = uuidParamSchema.safeParse(params.id);
      if (!idResult.success) {
        return jsonError(Errors.INVALID_SESSION_ID);
      }

      const { id } = params;

      const session = await db.session.findFirst({
        where: {
          id,
          userId: user.id,
        },
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
      });

      if (!session) {
        return jsonError(Errors.SESSION_NOT_FOUND);
      }

      // Transform to match expected response format
      const transformedSession = {
        id: session.id,
        user_id: session.userId,
        character_id: session.characterId,
        title: session.title,
        created_at: session.createdAt,
        updated_at: session.updatedAt,
        character_name: session.character?.name,
        character_avatar: session.character?.avatarUrl,
        message_count: session._count.messages,
      };

      return jsonSuccess(
        { session: transformedSession },
        undefined,
        { headers: getRateLimitHeaders(rateLimit) }
      );
    });
  },
});

// PUT /api/sessions/:id - Update session
export const updateSessionRoute = new Route({
  path: "/api/sessions/$id",
  method: "PUT",
  handler: async ({ request, params }: { request: Request; params: { id: string } }) => {
    return withErrorHandler(async () => {
      const user = await getCurrentUser(request);

      if (!user) {
        return jsonError(Errors.UNAUTHORIZED);
      }

      // Rate limiting
      const rateLimit = checkRateLimit(`update_session:${user.id}`, rateLimitConfigs.session);
      if (!rateLimit.allowed) {
        return jsonError(
          Errors.RATE_LIMITED.withDetails({ retry_after: rateLimit.retryAfter }),
          { headers: getRateLimitHeaders(rateLimit) }
        );
      }

      // Validate UUID
      const idResult = uuidParamSchema.safeParse(params.id);
      if (!idResult.success) {
        return jsonError(Errors.INVALID_SESSION_ID);
      }

      const { id } = params;

      // Parse and validate body
      const body = await request.json().catch(() => null);
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

      // Use transaction to ensure atomicity
      try {
        const session = await db.$transaction(async (tx) => {
          // Check if session exists and belongs to user
          const existingSession = await tx.session.findFirst({
            where: {
              id,
              userId: user.id,
            },
          });

          if (!existingSession) {
            throw new Error("SESSION_NOT_FOUND");
          }

          // Perform the update
          return tx.session.update({
            where: { id },
            data: {
              title: title !== undefined ? title : existingSession.title,
              characterId: character_id !== undefined ? character_id : existingSession.characterId,
            },
          });
        }, {
          timeout: 10000,
          isolationLevel: "ReadCommitted",
        });

        logger.info({
          sessionId: session.id,
          userId: user.id,
        }, "Session updated");

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
          { headers: getRateLimitHeaders(rateLimit) }
        );
      } catch (error) {
        if (error instanceof Error && error.message === "SESSION_NOT_FOUND") {
          return jsonError(Errors.SESSION_NOT_FOUND);
        }
        throw error;
      }
    });
  },
});

// DELETE /api/sessions/:id - Delete session
export const deleteSessionRoute = new Route({
  path: "/api/sessions/$id",
  method: "DELETE",
  handler: async ({ request, params }: { request: Request; params: { id: string } }) => {
    return withErrorHandler(async () => {
      const user = await getCurrentUser(request);

      if (!user) {
        return jsonError(Errors.UNAUTHORIZED);
      }

      // Rate limiting
      const rateLimit = checkRateLimit(`delete_session:${user.id}`, rateLimitConfigs.session);
      if (!rateLimit.allowed) {
        return jsonError(
          Errors.RATE_LIMITED.withDetails({ retry_after: rateLimit.retryAfter }),
          { headers: getRateLimitHeaders(rateLimit) }
        );
      }

      // Validate UUID
      const idResult = uuidParamSchema.safeParse(params.id);
      if (!idResult.success) {
        return jsonError(Errors.INVALID_SESSION_ID);
      }

      const { id } = params;

      // Use transaction to ensure atomicity
      try {
        await db.$transaction(async (tx) => {
          // Check if session exists and belongs to user
          const existingSession = await tx.session.findFirst({
            where: {
              id,
              userId: user.id,
            },
          });

          if (!existingSession) {
            throw new Error("SESSION_NOT_FOUND");
          }

          await tx.session.delete({
            where: { id },
          });
        }, {
          timeout: 10000,
          isolationLevel: "ReadCommitted",
        });

        logger.info({
          sessionId: id,
          userId: user.id,
        }, "Session deleted");

        return jsonSuccess(
          { success: true },
          undefined,
          { headers: getRateLimitHeaders(rateLimit) }
        );
      } catch (error) {
        if (error instanceof Error && error.message === "SESSION_NOT_FOUND") {
          return jsonError(Errors.SESSION_NOT_FOUND);
        }
        throw error;
      }
    });
  },
});
