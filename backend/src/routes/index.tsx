import { Route } from "@tanstack/start";

export const indexRoute = new Route({
  getStaticProps: async () => {
    return {
      head: {
        title: "Loom",
        meta: [
          { name: "description", content: "AI Companion with Vibe" },
        ],
      },
    };
  },
  component: () => <div>Loom MVP</div>,
});
