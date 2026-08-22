import type { Node } from "../domain/generated.ts";
import { Type } from "../domain/generated.ts";

// isPlainClick reports whether a click should be handled as in-app navigation
// (primary button, no modifier). A modified or middle click falls through to the
// browser, so ⌘/Ctrl/Shift/middle-click still open the anchor in a new tab/window.
export function isPlainClick(
  e: { button: number; metaKey: boolean; ctrlKey: boolean; shiftKey: boolean; altKey: boolean },
): boolean {
  return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
}

// UNCATEGORIZED is a sentinel category key for the catch-all group (epics with no
// category, and orphan issues). The leading NUL keeps it from colliding with any
// real category name. Spelled as an escape rather than written literally: a raw
// control byte in the source makes the file binary to grep, which silently drops
// it out of searches.
const UNCATEGORIZED = "\u0000uncategorized";

// TreeGroup is a display group of top-level nodes under one heading. An empty
// label means render no heading (the tree is a single flat group).
export interface TreeGroup {
  // Unique among the groups, for use as a render key. The label is not: a project
  // may have a category actually named "uncategorized", which would collide with
  // the catch-all group's heading.
  key: string;
  label: string;
  nodes: Node[];
}

// groupTopNodes partitions the top-level nodes into display groups: real
// categories in first-seen order, then the uncategorized catch-all last. The
// uncategorized group gets an "uncategorized" heading only when a real category is
// also present; otherwise the tree is a flat, unlabeled list. Both the tree render
// and keyboard navigation build from this, so their order always matches.
export function groupTopNodes(nodes: Node[]): TreeGroup[] {
  const order: string[] = [];
  const groups = new Map<string, Node[]>();
  for (const n of nodes) {
    const key = n.type === Type.Epic && n.category ? n.category : UNCATEGORIZED;
    if (!groups.has(key)) {
      groups.set(key, []);
      order.push(key);
    }
    groups.get(key)!.push(n);
  }
  const categories = order.filter((k) => k !== UNCATEGORIZED);
  const keys = groups.has(UNCATEGORIZED) ? [...categories, UNCATEGORIZED] : categories;
  return keys.map((key) => ({
    key,
    label: key === UNCATEGORIZED ? (categories.length > 0 ? "uncategorized" : "") : key,
    nodes: groups.get(key)!,
  }));
}

// findNode locates a node anywhere in the tree by ref.
export function findNode(nodes: Node[], ref: string): Node | null {
  for (const n of nodes) {
    if (n.ref === ref) return n;
    const hit = findNode(n.children ?? [], ref);
    if (hit) return hit;
  }
  return null;
}

// flatten returns every node in depth-first order.
export function flatten(nodes: Node[]): Node[] {
  const out: Node[] = [];
  const walk = (ns: Node[]) => {
    for (const n of ns) {
      out.push(n);
      walk(n.children ?? []);
    }
  };
  walk(nodes);
  return out;
}
