import { defineConfig } from "vite";
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
});
