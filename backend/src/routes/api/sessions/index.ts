import { Route, json } from "@tanstack/start";
import { db } from "~/lib/db";
import { getCurrentUser } from "~/lib/auth";
import { v4 as uuidv4 } from "uuid";

// GET /api/sessions - List all sessions for current user
export const listSessionsRoute = new Route({
  path: "/api/sessions",
  method: "GET",
  handler: async (req: Request) => {
    const user = await getCurrentUser(req);

    if (!user) {
      return json({ error: "Unauthorized" }, { status: 401 });
    }

    const result = await db.query(
      `SELECT s.id, s.user_id, s.character_id, s.title, s.created_at, s.updated_at,
              c.name as character_name, c.avatar_url as character_avatar
       FROM sessions s
       LEFT JOIN characters c ON s.character_id = c.id
       WHERE s.user_id = $1
       ORDER BY s.updated_at DESC`,
      [user.id]
    );

    return json({ sessions: result.rows });
  },
});

// POST /api/sessions - Create new session
export const createSessionRoute = new Route({
  path: "/api/sessions",
  method: "POST",
  handler: async (req: Request) => {
    const user = await getCurrentUser(req);

    if (!user) {
      return json({ error: "Unauthorized" }, { status: 401 });
    }

    const { title, character_id } = await req.json();

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

    const result = await db.query(
      `INSERT INTO sessions (user_id, character_id, title)
       VALUES ($1, $2, $3)
       RETURNING id, user_id, character_id, title, created_at, updated_at`,
      [user.id, character_id || null, title || null]
    );

    return json({ session: result.rows[0] }, { status: 201 });
  },
});
