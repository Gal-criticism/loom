import { Route, json } from "@tanstack/start";
import { db } from "~/lib/db";
import { getCurrentUser } from "~/lib/auth";

// GET /api/sessions/:id - Get single session details
export const getSessionRoute = new Route({
  path: "/api/sessions/$id",
  method: "GET",
  handler: async ({ request, params }: { request: Request; params: { id: string } }) => {
    const user = await getCurrentUser(request);

    if (!user) {
      return json({ error: "Unauthorized" }, { status: 401 });
    }

    const { id } = params;

    const result = await db.query(
      `SELECT s.id, s.user_id, s.character_id, s.title, s.created_at, s.updated_at,
              c.name as character_name, c.avatar_url as character_avatar
       FROM sessions s
       LEFT JOIN characters c ON s.character_id = c.id
       WHERE s.id = $1 AND s.user_id = $2`,
      [id, user.id]
    );

    if (result.rows.length === 0) {
      return json({ error: "Session not found" }, { status: 404 });
    }

    return json({ session: result.rows[0] });
  },
});

// PUT /api/sessions/:id - Update session
export const updateSessionRoute = new Route({
  path: "/api/sessions/$id",
  method: "PUT",
  handler: async ({ request, params }: { request: Request; params: { id: string } }) => {
    const user = await getCurrentUser(request);

    if (!user) {
      return json({ error: "Unauthorized" }, { status: 401 });
    }

    const { id } = params;
    const { title, character_id } = await request.json();

    // Validate title
    if (title !== undefined && (typeof title !== 'string' || title.length > 255)) {
      return json({ error: "Invalid title" }, { status: 400 });
    }

    // Validate character_id if provided
    if (character_id !== undefined) {
      if (typeof character_id !== 'number') {
        return json({ error: "Invalid character_id" }, { status: 400 });
      }

      const charResult = await db.query(
        "SELECT id FROM characters WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)",
        [character_id, user.id]
      );

      if (charResult.rows.length === 0) {
        return json({ error: "Character not found" }, { status: 404 });
      }
    }

    // Atomic update with RETURNING - no separate SELECT check needed
    const result = await db.query(
      `UPDATE sessions
       SET title = COALESCE($1, title),
           character_id = COALESCE($2, character_id),
           updated_at = NOW()
       WHERE id = $3 AND user_id = $4
       RETURNING id, user_id, character_id, title, created_at, updated_at`,
      [title, character_id, id, user.id]
    );

    if (result.rows.length === 0) {
      return json({ error: "Session not found" }, { status: 404 });
    }

    return json({ session: result.rows[0] });
  },
});

// DELETE /api/sessions/:id - Delete session
export const deleteSessionRoute = new Route({
  path: "/api/sessions/$id",
  method: "DELETE",
  handler: async ({ request, params }: { request: Request; params: { id: string } }) => {
    const user = await getCurrentUser(request);

    if (!user) {
      return json({ error: "Unauthorized" }, { status: 401 });
    }

    const { id } = params;

    // Atomic delete with RETURNING to check if row existed
    const result = await db.query(
      `DELETE FROM sessions
       WHERE id = $1 AND user_id = $2
       RETURNING id`,
      [id, user.id]
    );

    if (result.rows.length === 0) {
      return json({ error: "Session not found" }, { status: 404 });
    }

    return json({ success: true });
  },
});
