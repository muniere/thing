import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Minimal ambient declaration so we can read the dev API port from the env
// without pulling in @types/node just for this config.
declare const process: { env: Record<string, string | undefined> };

// The frontend is a pure SPA. In production thingd serves the built bundle on
// :4319; in development the Vite dev server takes that same :4319 (so the URL you
// open matches prod) and proxies /api and the /events SSE stream to thingd, which
// dev runs on a separate port. Both ports come from the env (the Makefile wires
// them, defaults 4319 / 4320) so a collision — e.g. another tool also defaulting
// to 4319 — is resolved with `make serve THINGD_WEB_PORT=<n> THINGD_API_PORT=<n>`;
// strictPort makes such a clash fail loudly rather than drift. http-proxy pipes
// responses through unbuffered, so /events streams without extra configuration.
const webPort = Number(process.env.THINGD_WEB_PORT ?? "4319");
const apiTarget = `http://localhost:${process.env.THINGD_API_PORT ?? "4320"}`;
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    port: webPort,
    strictPort: true,
    proxy: {
      "/api": { target: apiTarget, changeOrigin: true },
      "/events": { target: apiTarget, changeOrigin: true },
    },
  },
});
