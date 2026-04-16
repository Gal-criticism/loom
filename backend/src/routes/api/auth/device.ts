import { Route, json } from "@tanstack/start";
import { db } from "~/lib/prisma";
import { v4 as uuidv4 } from "uuid";

export const deviceAuthRoute = new Route({
  path: "/api/auth/device",
  method: "POST",
  handler: async (req: Request) => {
    const { device_id } = await req.json();

    if (!device_id) {
      // Generate new device ID
      const newDeviceId = uuidv4();

      const user = await db.user.create({
        data: {
          deviceId: newDeviceId,
        },
      });

      return json({
        user: {
          id: user.id,
          device_id: user.deviceId,
        },
        is_new: true,
      });
    }

    // Find existing user
    let user = await db.user.findUnique({
      where: { deviceId: device_id },
    });

    if (!user) {
      return json({ error: "Device not found" }, { status: 404 });
    }

    // Update last login time
    user = await db.user.update({
      where: { id: user.id },
      data: { lastLogin: new Date() },
    });

    return json({
      user: {
        id: user.id,
        device_id: user.deviceId,
      },
      is_new: false,
    });
  },
});
