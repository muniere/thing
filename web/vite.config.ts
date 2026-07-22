import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Vite is only the SPA bundler here: it builds web/dist, which thingd embeds and
// serves. There is no Vite dev server — the dev loop runs the one embedded binary
// under air and reloads over SSE (see the Makefile and web/src/live.ts), so the
// same binary serves the app in dev and prod.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
