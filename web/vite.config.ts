import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The frontend is a pure SPA served by thingd, the single entry point on :4319.
// In development (`thingd --dev`) it reverse-proxies asset requests to this Vite
// dev server for HMR while handling /api and /events itself; in production it
// serves the embedded bundle (wired in a later commit). So Vite needs no proxy
// of its own; it just needs a fixed port thingd can target — hence strictPort,
// which fails loudly rather than drifting off 5173.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    strictPort: true,
  },
});
