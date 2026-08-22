import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Node } from "../domain/generated.ts";
import { groupTopNodes } from "./util.ts";

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
// edit don't discard the user's manual folds. Turning a filter on unfolds the
// whole tree once, so a match can't start out hidden behind a fold. Selecting a
// node reveals it by unfolding its ancestors.
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

  // Unfold everything on the way into a filtered view, so a match cannot start
  // out hidden. This is a starting point rather than a lock: a fold made while
  // the filter is on is kept, which is what lets the carets work at all under a
  // configured default filter — a board that starts filtered would otherwise have
  // no way to fold anything, ever, and no way to turn the filter off either,
  // since the clear button hides while the filters equal the defaults.
  //
  // Keyed on entering the filtered state rather than on the filters' value:
  // typing in the search box changes them on every keystroke, and re-unfolding
  // there would throw away the reader's folds as they searched. It waits for a
  // tree so that it lands after the seeding effect above on a board that starts
  // filtered; effects run in declaration order, so on that commit this one wins.
  const unfoldedForFilter = useRef(false);
  useEffect(() => {
    if (!filtersActive) {
      unfoldedForFilter.current = false;
      return;
    }
    if (unfoldedForFilter.current || tree.length === 0) return;
    unfoldedForFilter.current = true;
    setCollapsed((prev) => (prev.size === 0 ? prev : new Set()));
  }, [filtersActive, tree]);

  // Reveal the active node by unfolding its ancestors. This also depends on the
  // tree: on a fresh load (a deep-link or a click-through, where the ref is set
  // from the URL before the tree arrives) the seeding effect above collapses the
  // active node's ancestors when the tree lands, so re-run afterward — it is
  // defined after seeding, so it wins on that commit — to keep the node visible.
  useEffect(() => {
    if (!activeRef) return;
    setCollapsed((prev) => {
      const next = new Set(prev);
      let changed = false;
      for (const a of ancestorRefs(activeRef)) if (next.delete(a)) changed = true;
      return changed ? next : prev;
    });
  }, [activeRef, tree]);

  const expanded = useCallback((ref: string) => !collapsed.has(ref), [collapsed]);
  const toggle = useCallback((ref: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (!next.delete(ref)) next.add(ref);
      return next;
    });
  }, []);

  return { expanded, toggle };
}

const NAV_KEYS = new Set([
  "j", "k", "h", "l", "g", "G", "Enter", "ArrowDown", "ArrowUp", "ArrowLeft", "ArrowRight",
]);

// A keystroke inside a text field or select is left to that field, not swallowed
// as navigation.
function inFormField(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el) return false;
  return el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT" || el.isContentEditable;
}

// parentRef("epic/issue/task") => "epic/issue"; "" for a top-level node.
function parentRef(ref: string): string {
  const i = ref.lastIndexOf("/");
  return i < 0 ? "" : ref.slice(0, i);
}

// useTreeNav installs vim/arrow keyboard navigation over the visible rows.
// j/k (and Down/Up) move the selection; g/G jump to the first/last row; h/l (and
// Left/Right) fold, unfold, or step to the parent/first child. Moving the cursor
// activates the row, keeping the detail pane in sync. It ignores keystrokes with a
// modifier or while a form field is focused.
export function useTreeNav(
  filtered: Node[],
  fold: TreeFold,
  activeRef: string | null,
  activate: (ref: string) => void,
): void {
  // The refs the keyboard can reach, in visual order: the top level follows the
  // same category grouping the tree renders (real categories first, uncategorized
  // last), and a node's children follow only when it is expanded.
  const rows = useMemo(() => {
    const out: { ref: string; hasChildren: boolean }[] = [];
    const walk = (nodes: Node[]) => {
      for (const n of nodes) {
        const kids = n.children ?? [];
        out.push({ ref: n.ref, hasChildren: kids.length > 0 });
        if (kids.length > 0 && fold.expanded(n.ref)) walk(kids);
      }
    };
    walk(groupTopNodes(filtered).flatMap((g) => g.nodes));
    return out;
  }, [filtered, fold.expanded]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey || inFormField(e.target)) return;
      if (!NAV_KEYS.has(e.key)) return;
      if (rows.length === 0) return;

      const i = rows.findIndex((r) => r.ref === activeRef);
      if (i < 0) {
        activate(rows[0].ref);
        e.preventDefault();
        return;
      }
      const cur = rows[i];

      switch (e.key) {
        case "j":
        case "ArrowDown":
          activate(rows[Math.min(i + 1, rows.length - 1)].ref);
          break;
        case "k":
        case "ArrowUp":
          activate(rows[Math.max(i - 1, 0)].ref);
          break;
        case "g":
          activate(rows[0].ref);
          break;
        case "G":
          activate(rows[rows.length - 1].ref);
          break;
        case "Enter":
          activate(cur.ref);
          break;
        case "h":
        case "ArrowLeft":
          if (cur.hasChildren && fold.expanded(cur.ref)) {
            fold.toggle(cur.ref);
          } else {
            const p = parentRef(cur.ref);
            if (p && rows.some((r) => r.ref === p)) activate(p);
          }
          break;
        case "l":
        case "ArrowRight":
          if (cur.hasChildren && !fold.expanded(cur.ref)) {
            fold.toggle(cur.ref);
          } else if (cur.hasChildren) {
            activate(rows[Math.min(i + 1, rows.length - 1)].ref);
          }
          break;
      }
      e.preventDefault();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [rows, activeRef, activate, fold.expanded, fold.toggle]);
}
