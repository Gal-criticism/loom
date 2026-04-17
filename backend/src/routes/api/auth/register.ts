import { Route } from "@tanstack/start";
import { db } from "~/lib/db";
import { withErrorHandler } from "~/middleware/errorHandler";
import { jsonSuccess, jsonError } from "~/lib/response";
import { Errors, APIError } from "~/lib/errors";

export const registerRoute = new Route({
  path: "/api/auth/register",
  method: "POST",
  handler: withErrorHandler(async ({ request }) => {
    const { email, password } = await request.json();

    if (!email || !password) {
      return jsonError(Errors.INVALID_INPUT.withDetails({
        message: "Email and password are required"
      }));
    }

    // TODO: 实现密码哈希
    const passwordHash = "hashed_" + password;

    try {
      const result = await db.query(
        "INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id, email",
        [email, passwordHash]
      );

      return jsonSuccess({ user: result.rows[0] });
    } catch (error: any) {
      // Check for unique constraint violation (PostgreSQL error code 23505)
      if (error?.code === "23505") {
        return jsonError(new APIError(
          "USER_ALREADY_EXISTS",
          "User already exists",
          409
        ));
      }
      throw error;
    }
  }),
});
