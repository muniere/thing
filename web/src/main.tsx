import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Root } from "./routes/root.tsx";
import "./styles/tokens.css";
import "./styles/global.css";
import "./styles/markdown.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("missing #root element");
}

createRoot(root).render(
  <StrictMode>
    <Root />
  </StrictMode>,
);
