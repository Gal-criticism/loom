import { Route as rootRoute } from "./routes/__root";
import { Route as indexRoute } from "./routes/index";
import {
  listSessionsRoute,
  createSessionRoute,
} from "./routes/api/sessions/index";
import {
  getSessionRoute,
  updateSessionRoute,
  deleteSessionRoute,
} from "./routes/api/sessions/[id]";
import { deviceAuthRoute } from "./routes/api/auth/device";
import {
  listMessagesRoute,
  sendMessageRoute,
} from "./routes/api/messages/index";
import {
  healthRoute,
  healthLiveRoute,
  healthReadyRoute,
} from "./routes/api/health";

declare module "@tanstack/start" {
  interface FileRoutesByPath {
    "/": {
      preLoaderRoute: typeof indexRoute;
      parentRoute: typeof rootRoute;
    };
  }
}

export const routeTree = rootRoute.addChildren([
  indexRoute,
  // Auth routes
  deviceAuthRoute,
  // Session routes
  listSessionsRoute,
  createSessionRoute,
  getSessionRoute,
  updateSessionRoute,
  deleteSessionRoute,
  // Message routes
  listMessagesRoute,
  sendMessageRoute,
  // Health check routes
  healthRoute,
  healthLiveRoute,
  healthReadyRoute,
]);
