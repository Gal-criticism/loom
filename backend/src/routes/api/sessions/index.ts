import { Route } from "@tanstack/start";
import { db } from "~/lib/prisma";
import { getCurrentUser } from "~/lib/auth";
import { Errors } from "~/lib/errors";
import { jsonSuccess, jsonError } from "~/lib/response";
import { withErrorHandler } from "~/middleware/errorHandler";

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

      const sessions = await db.session.findMany({
        where: { userId: user.id },
        include: {
          character: {
            select: {
              name: true,
              avatarUrl: true,
            },
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
      }));

      return jsonSuccess({ sessions: transformedSessions });
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

      const { title, character_id } = await req.json();

      // Validate title
      if (title !== undefined && (typeof title !== "string" || title.length > 255)) {
        return jsonError(Errors.INVALID_TITLE);
      }

      // Validate character_id if provided
      if (character_id !== undefined && character_id !== null) {
        if (typeof character_id !== "string") {
          return jsonError(Errors.INVALID_CHARACTER_ID);
        }

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

      // Transform to match expected response format
      const transformedSession = {
        id: session.id,
        user_id: session.userId,
        character_id: session.characterId,
        title: session.title,
        created_at: session.createdAt,
        updated_at: session.updatedAt,
      };

      return jsonSuccess({ session: transformedSession }, undefined, { status: 201 });
    });
  },
});
