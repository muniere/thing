import { useCallback, useEffect, useMemo, useState } from "react";
import type { Node } from "./domain/generated.ts";
import { api } from "./api.ts";
import { useLiveReload } from "./live.ts";
import { collectCategories, collectTags, filterTree, filtersFromQuery, filtersToQuery, type Filters } from "./filter.ts";
import { findNode } from "./util.ts";
import { Tree } from "./components/Tree.tsx";
import { FilterBar } from "./components/FilterBar.tsx";
import { Detail } from "./components/Detail.tsx";

// The active node's ref is carried in the URL path (e.g. /epic/issue/task);
// "/" means nothing active. Refs are slug paths ([a-z0-9-] joined by "/"), so
// they are URL-safe as-is. Filters and search live in the query string instead.
function refFromPath(): string | null {
  return window.location.pathname.replace(/^\/+/, "").replace(/\/+$/, "") || null;
}

export function App() {
  const [tree, setTree] = useState<Node[]>([]);
  const [activeRef, setActiveRef] = useState<string | null>(() => refFromPath());
  const [filters, setFilters] = useState<Filters>(() => filtersFromQuery(window.location.search));
  const [error, setError] = useState<string | null>(null);
  const [newEpic, setNewEpic] = useState("");

  const reload = useCallback(async () => {
    try {
      setTree(await api.tree());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);
  useLiveReload(reload);

  // run awaits a mutation, surfaces any error, then refreshes the tree.
  const run = useCallback(
    async <T,>(p: Promise<T>): Promise<T | undefined> => {
      setError(null);
      try {
        const r = await p;
        await reload();
        return r;
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
        return undefined;
      }
    },
    [reload],
  );

  // activate makes a node the focused one (or clears it when given ""); the tree
  // and detail fire it when the user picks a node.
  const activate = useCallback((ref: string) => setActiveRef(ref || null), []);

  // Mirror the view into the URL so it survives a reload and is shareable: the
  // active node is the path (/<ref>) and the filters are the query string.
  // replaceState keeps each change out of the history stack.
  useEffect(() => {
    window.history.replaceState(null, "", `/${activeRef ?? ""}${filtersToQuery(filters)}`);
  }, [activeRef, filters]);

  // Restore the active node (path) and filters (query) on back/forward.
  useEffect(() => {
    const onPop = () => {
      setActiveRef(refFromPath());
      setFilters(filtersFromQuery(window.location.search));
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  const filtered = useMemo(() => filterTree(tree, filters), [tree, filters]);
  const categories = useMemo(() => collectCategories(tree), [tree]);
  const tags = useMemo(() => collectTags(tree), [tree]);
  const activeNode = activeRef ? findNode(tree, activeRef) : null;

  const addEpic = async () => {
    const t = newEpic.trim();
    if (!t) return;
    const res = await run(api.create("", { title: t }));
    if (res) {
      setNewEpic("");
      setActiveRef(res.ref);
    }
  };

  return (
    <div className="app">
      <header className="topbar">
        <span className="brand"><span className="dot" />thing</span>
      </header>

      {error && <div className="error" onClick={() => setError(null)}>{error}</div>}

      <div className="split">
        <FilterBar filters={filters} categories={categories} tags={tags} onChange={setFilters} />

        <section className="tree-pane">
          <div className="tree-add">
            <input
              className="input"
              placeholder="new epic title"
              value={newEpic}
              onChange={(e) => setNewEpic(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && addEpic()}
            />
            <button type="button" className="btn btn-amber" onClick={addEpic}>+ Epic</button>
          </div>
          <Tree nodes={filtered} activeRef={activeRef} onSelect={activate} />
        </section>

        <section className="detail-pane">
          {activeNode ? (
            <Detail key={activeNode.ref} node={activeNode} allNodes={tree} run={run} onSelect={activate} />
          ) : (
            <p className="empty">Select a node to view and edit it.</p>
          )}
        </section>
      </div>
    </div>
  );
}
