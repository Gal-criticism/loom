import { createRouter as createTanStackRouter } from "@tanstack/start/router";
import { routeTree } from "./routeTree.gen";

export function getRouter() {
  return createTanStackRouter({
    routeTree,
  });
}

export function createRouter() {
  return getRouter();
}
