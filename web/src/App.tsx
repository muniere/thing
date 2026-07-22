import { useCallback, useEffect, useMemo, useState } from "react";
import type { Node } from "./domain/generated.ts";
import { api } from "./api.ts";
import { useLiveReload } from "./live.ts";
import { collectCategories, collectTags, emptyFilters, filterTree, type Filters } from "./filter.ts";
import { findNode } from "./util.ts";
import { Tree } from "./components/Tree.tsx";
import { FilterBar } from "./components/FilterBar.tsx";
import { Detail } from "./components/Detail.tsx";

export function App() {
  const [tree, setTree] = useState<Node[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [filters, setFilters] = useState<Filters>(emptyFilters);
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

  const select = useCallback((ref: string) => setSelected(ref || null), []);

  const filtered = useMemo(() => filterTree(tree, filters), [tree, filters]);
  const categories = useMemo(() => collectCategories(tree), [tree]);
  const tags = useMemo(() => collectTags(tree), [tree]);
  const selectedNode = selected ? findNode(tree, selected) : null;

  const addEpic = async () => {
    const t = newEpic.trim();
    if (!t) return;
    const res = await run(api.create("", { title: t }));
    if (res) {
      setNewEpic("");
      setSelected(res.ref);
    }
  };

  return (
    <div className="app">
      <header>
        <h1>thing</h1>
        <FilterBar filters={filters} categories={categories} tags={tags} onChange={setFilters} />
      </header>

      {error && <div className="error" onClick={() => setError(null)}>{error}</div>}

      <div className="panes">
        <section className="pane tree-pane">
          <div className="add-epic">
            <input placeholder="new epic title" value={newEpic} onChange={(e) => setNewEpic(e.target.value)} />
            <button type="button" onClick={addEpic}>+ Epic</button>
          </div>
          <Tree nodes={filtered} selected={selected} onSelect={select} />
        </section>

        <section className="pane detail-pane">
          {selectedNode ? (
            <Detail key={selectedNode.ref} node={selectedNode} allNodes={tree} run={run} onSelect={select} />
          ) : (
            <p className="empty">Select a node to view and edit it.</p>
          )}
        </section>
      </div>
    </div>
  );
}
