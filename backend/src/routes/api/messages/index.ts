import { Route, json } from "@tanstack/start";
import { db } from "~/lib/db";
import { getCurrentUser } from "~/lib/auth";
import { getWSServer } from "~/lib/ws";
import { formatUserMessage } from "~/lib/messageHandler";

const ws = getWSServer();

// GET /api/messages?session_id=xxx - List messages for a session with pagination
export const listMessagesRoute = new Route({
  path: "/api/messages",
  method: "GET",
  handler: async (req: Request) => {
    const user = await getCurrentUser(req);

    if (!user) {
      return json({ error: "Unauthorized" }, { status: 401 });
    }

    const url = new URL(req.url);
    const sessionId = url.searchParams.get("session_id");

    if (!sessionId) {
      return json({ error: "session_id is required" }, { status: 400 });
    }

    // Validate session belongs to user
    const sessionResult = await db.query(
      "SELECT id FROM sessions WHERE id = $1 AND user_id = $2",
      [sessionId, user.id]
    );

    if (sessionResult.rows.length === 0) {
      return json({ error: "Session not found" }, { status: 404 });
    }

    // Parse pagination parameters
    const limit = Math.min(parseInt(url.searchParams.get("limit") || "50", 10), 100);
    const offset = parseInt(url.searchParams.get("offset") || "0", 10);

    // Validate pagination parameters
    if (isNaN(limit) || limit < 0) {
      return json({ error: "Invalid limit parameter" }, { status: 400 });
    }

    if (isNaN(offset) || offset < 0) {
      return json({ error: "Invalid offset parameter" }, { status: 400 });
    }

    const result = await db.query(
      `SELECT id, session_id, role, content, tools, created_at
       FROM messages
       WHERE session_id = $1
       ORDER BY created_at ASC
       LIMIT $2 OFFSET $3`,
      [sessionId, limit, offset]
    );

    // Get total count for pagination metadata
    const countResult = await db.query(
      "SELECT COUNT(*) as total FROM messages WHERE session_id = $1",
      [sessionId]
    );

    const total = parseInt(countResult.rows[0].total, 10);

    return json({
      messages: result.rows,
      pagination: {
        total,
        limit,
        offset,
        has_more: offset + result.rows.length < total,
      },
    });
  },
});

// POST /api/messages - Send a new message in a session
export const sendMessageRoute = new Route({
  path: "/api/messages",
  method: "POST",
  handler: async (req: Request) => {
    const user = await getCurrentUser(req);

    if (!user) {
      return json({ error: "Unauthorized" }, { status: 401 });
    }

    const body = await req.json();
    const { session_id, content } = body;

    // Validate session_id
    if (!session_id) {
      return json({ error: "session_id is required" }, { status: 400 });
    }

    if (typeof session_id !== "string") {
      return json({ error: "Invalid session_id" }, { status: 400 });
    }

    // Validate content
    if (!content) {
      return json({ error: "content is required" }, { status: 400 });
    }

    if (typeof content !== "string" || content.trim().length === 0) {
      return json({ error: "Invalid content" }, { status: 400 });
    }

    if (content.length > 100000) {
      return json({ error: "Content exceeds maximum length of 100000 characters" }, { status: 400 });
    }

    // Validate session belongs to user
    const sessionResult = await db.query(
      "SELECT id FROM sessions WHERE id = $1 AND user_id = $2",
      [session_id, user.id]
    );

    if (sessionResult.rows.length === 0) {
      return json({ error: "Session not found" }, { status: 404 });
    }

    // Save user message
    const messageResult = await db.query(
      `INSERT INTO messages (session_id, role, content)
       VALUES ($1, 'user', $2)
       RETURNING id, session_id, role, content, tools, created_at`,
      [session_id, content.trim()]
    );

    const message = messageResult.rows[0];

    // Send to Daemon via WebSocket
    try {
      const daemonMessage = formatUserMessage(session_id, message.id, message.content);
      ws.sendToDaemon(user.device_id, daemonMessage);
    } catch (error) {
      console.error("Failed to send message to daemon:", error);
      // Don't fail the request if WebSocket fails - message is already saved
    }

    return json({ message }, { status: 201 });
  },
});
