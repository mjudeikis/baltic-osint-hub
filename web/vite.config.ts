import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  define: {
    "import.meta.env.VITE_BUILD_ID": JSON.stringify(
      process.env.BUILD_ID ?? String(Date.now()),
    ),
  },
  plugins: [react()],
  server: {
    proxy: {
      // VITE_API_PROXY=https://osintbaltic.com points the dev server at the
      // live API, so design work runs against real data without a local
      // collector. Default is the local Go server.
      "/api": {
        target: process.env.VITE_API_PROXY ?? "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  test: {
    // Pure modules only (URL state, board levels, feed grouping, timeline
    // shaping); nothing here needs a DOM.
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
