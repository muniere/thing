// Bundles the SPA with esbuild into dist/ — the bundle thingd embeds. esbuild
// replaces Vite here (there is no dev server; the dev loop runs the embedded
// binary under air, see the repo Makefile), so this script does the few things
// Vite gave us for free: bundle+minify the entry, hash the asset names, emit the
// CSS bundle as a separate hashed file, and template the hashed asset paths into
// index.html.
//
// Run via `npm run build` (which type-checks first with tsc). It empties dist/
// like Vite's emptyOutDir did, then rewrites the committed .gitkeep so the
// unconditional go:embed target stays present on a fresh checkout.
import * as esbuild from "esbuild";
import { rm, mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";

const outdir = "dist";
const assetdir = path.join(outdir, "assets");

await rm(outdir, { recursive: true, force: true });
await mkdir(assetdir, { recursive: true });
// Write .gitkeep up front, before esbuild can fail: thingd embeds dist
// unconditionally (see assets.go / the Makefile), so if a bundling error left
// dist empty, `go build`/`test`/`vet` would break with a confusing
// "pattern all:web/dist: no matching files" two tools removed from the real
// cause. Keeping the embed target present turns that into a plain missing bundle.
await writeFile(path.join(outdir, ".gitkeep"), "");

const result = await esbuild.build({
  entryPoints: ["src/main.tsx"],
  bundle: true,
  format: "esm",
  target: "es2022",
  minify: true,
  jsx: "automatic",
  // React reads process.env.NODE_ENV; define it so the production build is used
  // and no bare `process` reference reaches the browser.
  define: { "process.env.NODE_ENV": '"production"' },
  outdir: assetdir,
  entryNames: "[name]-[hash]",
  assetNames: "[name]-[hash]",
  loader: { ".svg": "dataurl", ".png": "dataurl", ".woff2": "file" },
  metafile: true,
  logLevel: "info",
});

// Locate the entry's JS output and its emitted CSS sibling from the metafile.
let jsFile, cssFile;
for (const [file, meta] of Object.entries(result.metafile.outputs)) {
  if (meta.entryPoint === "src/main.tsx") {
    jsFile = file;
    cssFile = meta.cssBundle;
  }
}
// Both are load-bearing invariants for this app: no JS means nothing renders, and
// the entry has always imported the stylesheet, so a missing CSS bundle means the
// import was lost, not that the app is intentionally unstyled. Fail loudly rather
// than ship a blank or unstyled page under a green build.
if (!jsFile) throw new Error("esbuild: entry JS output not found in metafile");
if (!cssFile) throw new Error("esbuild: entry produced no CSS bundle — expected the SPA stylesheet");

// Map a dist-relative output path to the URL thingd serves it at (site root).
const url = (p) => "/" + path.relative(outdir, p).split(path.sep).join("/");

// replaceOrThrow substitutes a template marker in index.html and hard-fails if
// the marker is absent — a plain String.replace silently returns the input
// unchanged, which would ship an index.html that never links the bundle (a blank
// or unstyled page under a green build).
const replaceOrThrow = (html, marker, replacement) => {
  if (!html.includes(marker)) {
    throw new Error(`build: marker not found in index.html, cannot link the bundle: ${marker}`);
  }
  return html.replace(marker, replacement);
};

let html = await readFile("index.html", "utf8");
html = replaceOrThrow(
  html,
  '<script type="module" src="/src/main.tsx"></script>',
  `<script type="module" src="${url(jsFile)}"></script>`,
);
html = replaceOrThrow(
  html,
  "</head>",
  `  <link rel="stylesheet" href="${url(cssFile)}" />\n  </head>`,
);
await writeFile(path.join(outdir, "index.html"), html);
