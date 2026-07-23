// Let TypeScript resolve the side-effect CSS import in main.tsx (`import
// "./index.css"`). esbuild bundles the CSS at build time; tsc only needs to know
// the module exists.
declare module "*.css";
