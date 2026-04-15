import { createStartHandler } from "@tanstack/start";
import { getRouter } from "./router";

export default createStartHandler({
  createRouter: () => getRouter(),
});
