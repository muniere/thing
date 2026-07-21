import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The frontend is a pure SPA. thingd serves the built bundle in production; in
// development `vite dev` serves it directly. Once thingd exists, the dev server
// also proxies /api and the /events SSE stream to it — wired in a later commit.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
