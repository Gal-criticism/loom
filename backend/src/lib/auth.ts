import { db } from "./db";

export async function getCurrentUser(req: Request) {
  const deviceId = req.headers.get("x-device-id");

  if (!deviceId) {
    return null;
  }

  const result = await db.query(
    "SELECT id, device_id FROM users WHERE device_id = $1",
    [deviceId]
  );

  return result.rows[0] || null;
}
