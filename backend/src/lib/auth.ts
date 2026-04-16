import { db } from "./prisma";

export async function getCurrentUser(req: Request) {
  const deviceId = req.headers.get("x-device-id");

  if (!deviceId) {
    return null;
  }

  const user = await db.user.findUnique({
    where: { deviceId },
  });

  return user || null;
}
