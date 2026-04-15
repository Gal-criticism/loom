import { Route, json } from "@tanstack/start";
import { db } from "~/lib/db";

export const registerRoute = new Route({
  path: "/api/auth/register",
  method: "POST",
  handler: async (req: Request) => {
    const { email, password } = await req.json();

    // TODO: 实现密码哈希
    const passwordHash = "hashed_" + password;

    try {
      const result = await db.query(
        "INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id, email",
        [email, passwordHash]
      );

      return json({ user: result.rows[0] });
    } catch (error) {
      return json({ error: "User already exists" }, { status: 400 });
    }
  },
});
