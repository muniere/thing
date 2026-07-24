import { useCallback, useEffect, useRef, useState } from "react";
import type { Node } from "./domain/generated.ts";

// ancestorRefs("epic/issue/task") => ["epic", "epic/issue"]. A ref is a slug
// path, so its ancestors are its proper prefixes.
function ancestorRefs(ref: string): string[] {
  const parts = ref.split("/");
  const out: string[] = [];
  for (let i = 1; i < parts.length; i++) out.push(parts.slice(0, i).join("/"));
  return out;
}

export interface TreeFold {
  // expanded reports whether a node's children are shown.
  expanded: (ref: string) => boolean;
  // toggle folds an expanded node or unfolds a collapsed one.
  toggle: (ref: string) => void;
}

// useTreeFold tracks which nodes are collapsed. Top-level nodes (epics, orphan
// issues) start open; a nested node that has children starts folded, matching
// the outline convention. Each node is seeded once, so the SSE reloads after an
// edit don't discard the user's manual folds. When a filter is active every node
// is force-expanded so a match can't hide behind a fold. Selecting a node reveals
// it by unfolding its ancestors.
export function useTreeFold(tree: Node[], filtersActive: boolean, activeRef: string | null): TreeFold {
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const seen = useRef<Set<string>>(new Set());

  // Seed the fold default for nodes not observed before: a nested node with
  // children starts collapsed. Nodes already seen keep whatever fold the user set.
  useEffect(() => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      let changed = false;
      const walk = (nodes: Node[], top: boolean) => {
        for (const n of nodes) {
          const children = n.children ?? [];
          if (!seen.current.has(n.ref)) {
            seen.current.add(n.ref);
            if (!top && children.length > 0) {
              next.add(n.ref);
              changed = true;
            }
          }
          walk(children, false);
        }
      };
      walk(tree, true);
      return changed ? next : prev;
    });
  }, [tree]);

  // Reveal the active node by unfolding its ancestors.
  useEffect(() => {
    if (!activeRef) return;
    setCollapsed((prev) => {
      const next = new Set(prev);
      let changed = false;
      for (const a of ancestorRefs(activeRef)) if (next.delete(a)) changed = true;
      return changed ? next : prev;
    });
  }, [activeRef]);

  const expanded = useCallback(
    (ref: string) => filtersActive || !collapsed.has(ref),
    [filtersActive, collapsed],
  );
  const toggle = useCallback((ref: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (!next.delete(ref)) next.add(ref);
      return next;
    });
  }, []);

  return { expanded, toggle };
}
