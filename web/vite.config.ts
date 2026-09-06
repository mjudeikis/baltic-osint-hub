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
      "/api": "http://localhost:8080",
    },
  },
  test: {
    // Pure modules only (URL state, board levels, feed grouping, timeline
    // shaping); nothing here needs a DOM.
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
