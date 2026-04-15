import { Route, json } from "@tanstack/start";
import { db } from "~/lib/db";
import { v4 as uuidv4 } from "uuid";

export const deviceAuthRoute = new Route({
  path: "/api/auth/device",
  method: "POST",
  handler: async (req: Request) => {
    const { device_id } = await req.json();

    if (!device_id) {
      // 生成新设备 ID
      const newDeviceId = uuidv4();

      const result = await db.query(
        "INSERT INTO users (device_id) VALUES ($1) RETURNING id, device_id",
        [newDeviceId]
      );

      return json({
        user: result.rows[0],
        is_new: true
      });
    }

    // 查找现有用户
    const result = await db.query(
      "SELECT id, device_id FROM users WHERE device_id = $1",
      [device_id]
    );

    if (result.rows.length === 0) {
      return json({ error: "Device not found" }, { status: 404 });
    }

    // 更新最后登录时间
    await db.query(
      "UPDATE users SET last_login = NOW() WHERE device_id = $1",
      [device_id]
    );

    return json({
      user: result.rows[0],
      is_new: false
    });
  },
});
