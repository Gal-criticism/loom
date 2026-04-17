import { Route } from "@tanstack/start";
import { db } from "~/lib/prisma";
import { withErrorHandler } from "~/middleware/errorHandler";
import { jsonSuccess, jsonError } from "~/lib/response";
import { Errors, APIError } from "~/lib/errors";

export const registerRoute = new Route({
  path: "/api/auth/register",
  method: "POST",
  handler: withErrorHandler(async ({ request }: { request: Request }) => {
    const { email, password } = await request.json();

    if (!email || !password) {
      return jsonError(Errors.INVALID_INPUT.withDetails({
        message: "Email and password are required"
      }));
    }

    // TODO: Implement password hashing
    const passwordHash = "hashed_" + password;

    try {
      const user = await db.user.create({
        data: {
          email,
          passwordHash,
        },
        select: {
          id: true,
          email: true,
        },
      });

      return jsonSuccess({ user });
    } catch (error: any) {
      // Check for unique constraint violation (PostgreSQL error code P2002 in Prisma)
      if (error?.code === "P2002") {
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
