import { Route as rootRoute } from "./routes/__root";
import { Route as indexRoute } from "./routes/index";

declare module "@tanstack/start" {
  interface FileRoutesByPath {
    "/": {
      preLoaderRoute: typeof indexRoute;
      parentRoute: typeof rootRoute;
    };
  }
}

export const routeTree = rootRoute.addChildren([indexRoute]);
