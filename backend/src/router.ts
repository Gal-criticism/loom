import { createRouter as createTanStackRouter } from "@tanstack/start/router";

export function getRouter() {
  return createTanStackRouter({
    routeTree: import("./routeTree.gen"),
  });
}

export function createRouter() {
  return getRouter();
}
