import { Route } from "@tanstack/start";
import { db } from "~/lib/prisma";
import { getCurrentUser } from "~/lib/auth";
import { Errors } from "~/lib/errors";
import { jsonSuccess, jsonError } from "~/lib/response";
import { withErrorHandler } from "~/middleware/errorHandler";

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
      };

      return jsonSuccess({ session: transformedSession });
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

      const { id } = params;
      const { title, character_id } = await request.json();

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

      // Check if session exists and belongs to user before updating
      const existingSession = await db.session.findFirst({
        where: {
          id,
          userId: user.id,
        },
      });

      if (!existingSession) {
        return jsonError(Errors.SESSION_NOT_FOUND);
      }

      // Perform the update
      const session = await db.session.update({
        where: { id },
        data: {
          title: title !== undefined ? title : existingSession.title,
          characterId: character_id !== undefined ? character_id : existingSession.characterId,
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

      return jsonSuccess({ session: transformedSession });
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

      const { id } = params;

      // Check if session exists and belongs to user before deleting
      const existingSession = await db.session.findFirst({
        where: {
          id,
          userId: user.id,
        },
      });

      if (!existingSession) {
        return jsonError(Errors.SESSION_NOT_FOUND);
      }

      await db.session.delete({
        where: { id },
      });

      return jsonSuccess({ success: true });
    });
  },
});
