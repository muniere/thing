import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The frontend is a pure SPA served by thingd, which is the single entry point
// on :4319 in both modes: it serves the embedded bundle in production, and in
// development it reverse-proxies asset requests to this Vite dev server (for
// HMR) while handling /api and /events itself. So Vite needs no proxy of its
// own; the dev reverse-proxy is wired into thingd with the browse/edit UI in a
// later commit.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
