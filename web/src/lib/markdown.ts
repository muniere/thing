import { marked } from "marked";
import DOMPurify from "dompurify";

// Render a node's free-form Markdown body to HTML, then sanitize it so a body
// authored in another tool can never inject script/style/event handlers. The
// sanitize pass is the security boundary; marked output is treated as untrusted.
// A parse failure on a pathological body falls back to sanitized plain text
// rather than throwing during render (which, with no error boundary, would blank
// the pane).
export function renderMarkdown(md: string): string {
  try {
    const html = marked.parse(md ?? "", { async: false, gfm: true, breaks: true });
    return DOMPurify.sanitize(html, { USE_PROFILES: { html: true } });
  } catch (e) {
    console.error("markdown render failed", e);
    return DOMPurify.sanitize(`<pre>${md ?? ""}</pre>`);
  }
}
