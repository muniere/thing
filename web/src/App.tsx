import { type MouseEvent, useCallback, useEffect, useMemo, useState } from "react";
import type { Node } from "./domain/generated.ts";
import { api } from "./api.ts";
import { useLiveReload } from "./live.ts";
import { collectCategories, collectPriorityCounts, collectStatusCounts, collectTags, filterTree, filtersActive, filtersFromQuery, filtersToQuery, type Filters } from "./filter.ts";
import { findNode, isPlainClick } from "./util.ts";
import { useTreeFold, useTreeNav } from "./tree.ts";
import { Tree } from "./components/Tree.tsx";
import { FilterBar } from "./components/FilterBar.tsx";
import { Detail } from "./components/Detail.tsx";
import { AddForm } from "./components/AddForm.tsx";
import s from "./App.module.css";

// The active node's ref is carried in the URL path (e.g. /epic/issue/task);
// "/" means nothing active. Refs are slug paths ([a-z0-9-] joined by "/"), so
// they are URL-safe as-is. Filters and search live in the query string instead.
function refFromPath(): string | null {
  return window.location.pathname.replace(/^\/+/, "").replace(/\/+$/, "") || null;
}

export function App() {
  const [tree, setTree] = useState<Node[]>([]);
  const [title, setTitle] = useState("thing");
  const [dir, setDir] = useState("");
  const [activeRef, setActiveRef] = useState<string | null>(() => refFromPath());
  const [filters, setFilters] = useState<Filters>(() => filtersFromQuery(window.location.search));
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    try {
      setTree(await api.tree());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  // Load the configured title and keep the browser tab in sync with it. It also
  // roots the tree and labels the top-left logo. Refetched on live-reload since
  // editing config.yaml changes it.
  const loadConfig = useCallback(async () => {
    try {
      const c = await api.config();
      setTitle(c.title || "thing");
      setDir(c.dir);
    } catch {
      // a missing/unreachable config just leaves the defaults
    }
  }, []);
  useEffect(() => {
    void reload();
    void loadConfig();
  }, [reload, loadConfig]);
  // One SSE subscription refreshes both the tree and the config (title/dir).
  const refresh = useCallback(() => {
    void reload();
    void loadConfig();
  }, [reload, loadConfig]);
  useLiveReload(refresh);
  useEffect(() => {
    document.title = title;
  }, [title]);

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

  // hrefFor is the URL a node's anchor points at: its ref as the path plus the
  // current filters as the query. Tree rows, child rows, and the logo are real
  // <a> links so the URL is real (a ⌘/Ctrl/Shift/middle click opens the node in a
  // new tab, and the link is copyable), but a plain click is intercepted (see
  // onNav) and handled in-app rather than reloading the page.
  const hrefFor = useCallback((ref: string) => `/${ref}${filtersToQuery(filters)}`, [filters]);

  // navigate selects a node and pushes a history entry, so a plain click behaves
  // like a page navigation (Back/Forward step through visited nodes) without the
  // reload — no refetching the tree, reopening the SSE stream, or piling requests
  // onto the connection pool.
  const navigate = useCallback(
    (ref: string) => {
      window.history.pushState(null, "", `/${ref}${filtersToQuery(filters)}`);
      setActiveRef(ref || null);
    },
    [filters],
  );

  // onNav handles an anchor click: a plain click navigates in-app; a modified or
  // middle click is left to the browser so its default (open in a new tab) stands.
  const onNav = useCallback(
    (e: MouseEvent, ref: string) => {
      if (!isPlainClick(e)) return;
      e.preventDefault();
      navigate(ref);
    },
    [navigate],
  );

  // Keyboard nav, filter toggles, and create/delete/rename change the view without
  // pushing history, so mirror them into the current URL in place — no new entry,
  // no reload — keeping it shareable and correct if the page is then reloaded.
  useEffect(() => {
    window.history.replaceState(null, "", `/${activeRef ?? ""}${filtersToQuery(filters)}`);
  }, [activeRef, filters]);

  // Restore the view from the URL on Back/Forward (a popstate, which pushState
  // navigation produces).
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
  const statusCounts = useMemo(() => collectStatusCounts(tree), [tree]);
  const priorityCounts = useMemo(() => collectPriorityCounts(tree), [tree]);
  const activeNode = activeRef ? findNode(tree, activeRef) : null;
  const fold = useTreeFold(tree, filtersActive(filters), activeRef);
  useTreeNav(filtered, fold, activeRef, activate);

  // Keep the active row on screen: when the selection moves (via keyboard nav
  // especially), scroll the least amount needed to reveal it. The selected row
  // carries a stable data-selected hook (its class name is hashed by CSS Modules).
  useEffect(() => {
    if (!activeRef) return;
    document.querySelector("[data-selected]")?.scrollIntoView({ block: "nearest" });
  }, [activeRef]);

  return (
    <div className={s.app}>
      <header className={s.topbar}>
        <a className={s.brand} href={hrefFor("")} onClick={(e) => onNav(e, "")}>
          <span className={s.dot} />{title}
        </a>
        <div className={s.topbarAdd}>
          <AddForm parent="" noun="Epic" amber floating run={run} onCreated={activate} />
        </div>
      </header>

      {error && <div className={s.error} onClick={() => setError(null)}>{error}</div>}

      <div className={s.split}>
        <FilterBar
          filters={filters}
          categories={categories}
          tags={tags}
          statusCounts={statusCounts}
          priorityCounts={priorityCounts}
          onChange={setFilters}
        />

        <section className={s.treePane}>
          {dir && <div className={s.dir}>{dir}</div>}
          <Tree nodes={filtered} activeRef={activeRef} hrefFor={hrefFor} onNav={onNav} expanded={fold.expanded} onToggle={fold.toggle} />
        </section>

        <section className={s.detailPane}>
          {activeNode ? (
            <Detail key={activeNode.ref} node={activeNode} allNodes={tree} run={run} onSelect={activate} hrefFor={hrefFor} onNav={onNav} />
          ) : (
            <p className={s.empty}>Select a node to view and edit it.</p>
          )}
        </section>
      </div>
    </div>
  );
}
