import { type MouseEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Node } from "./domain/generated.ts";
import { forProject, type ArchiveEntry, type Scheme } from "./api.ts";
import { useLiveReload } from "./live.ts";
import { collectCategories, collectPriorityCounts, collectStatusCounts, collectTags, defaultsToFilters, emptyFilters, filterTree, filtersActive, filtersFromQuery, filtersToQuery, hasFilterQuery, type Filters } from "./filter.ts";
import { findNode, isPlainClick } from "./util.ts";
import { applyTheme } from "./theme.ts";
import { useTreeFold, useTreeNav } from "./tree.ts";
import { NodeChainList } from "./components/NodeChainList/NodeChainList.tsx";
import { FilterForm } from "./components/FilterForm/FilterForm.tsx";
import { NodeDetailPanel } from "./components/NodeDetailPanel/NodeDetailPanel.tsx";
import { NodeFormDialog } from "./components/NodeFormDialog/NodeFormDialog.tsx";
import { ProjectSwitcher } from "./components/ProjectSwitcher/ProjectSwitcher.tsx";
import { SchemeMenu } from "./components/SchemeMenu/SchemeMenu.tsx";
import { ArchiveList } from "./components/ArchiveList/ArchiveList.tsx";
import s from "./App.module.css";

interface Props {
  // The project this view is scoped to (the first URL path segment).
  project: string;
  // The server-wide color scheme and the setter for it, both owned by Root.
  scheme: Scheme;
  onScheme: (scheme: Scheme) => void;
  // Re-read the server-wide settings. Called on live-reload, since a scheme
  // changed in another tab arrives the same way a tree change does.
  onRefresh: () => void;
  // Switch to another project by name, or to the picker (null). Wired to the
  // logo's switcher caret; Root remounts this component on the new project.
  onSwitch: (name: string | null) => void;
}

// The URL path is /<project>/<ref> (e.g. /work/epic/issue/task); /<project> alone
// means nothing is active. Refs are slug paths ([a-z0-9-] joined by "/"), so they
// are URL-safe as-is. Filters and search live in the query string instead.
function refFromPath(project: string): string | null {
  const path = window.location.pathname.replace(/^\/+/, "").replace(/\/+$/, "");
  const prefix = `${project}/`;
  return path.startsWith(prefix) ? path.slice(prefix.length) || null : null;
}

export function App({ project, onSwitch, scheme, onScheme, onRefresh }: Props) {
  const api = useMemo(() => forProject(project), [project]);
  const [tree, setTree] = useState<Node[]>([]);
  const [archived, setArchived] = useState<ArchiveEntry[]>([]);
  const [title, setTitle] = useState(project);
  const [dir, setDir] = useState("");
  const [activeRef, setActiveRef] = useState<string | null>(() => refFromPath(project));
  const [filters, setFilters] = useState<Filters>(() => filtersFromQuery(window.location.search));
  const [defaults, setDefaults] = useState<Filters>(emptyFilters);
  // Whether the config fetch has settled (either way — see loadConfig's catch).
  // Until then `defaults` is a placeholder, not "no configured defaults", so the
  // replaceState effect below must not write the URL from it: see its comment.
  const [configReady, setConfigReady] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // pathFor builds the URL path for a node ref within this project: /<project> at
  // the root, /<project>/<ref> for a node.
  const pathFor = useCallback((ref: string) => (ref ? `/${project}/${ref}` : `/${project}`), [project]);

  const reload = useCallback(async () => {
    // The tree is the primary view; the archive list is supplementary. Fetch both
    // together but handle them independently so a failing archive fetch never
    // blanks the tree.
    const [t, a] = await Promise.allSettled([api.tree(), api.listArchive()]);
    if (t.status === "fulfilled") {
      setTree(t.value);
    } else {
      setError(t.reason instanceof Error ? t.reason.message : String(t.reason));
      return;
    }
    setArchived(a.status === "fulfilled" ? a.value : []);
  }, [api]);

  // Load the configured title and keep the browser tab in sync with it. It also
  // roots the tree and labels the top-left logo. Refetched on live-reload since
  // editing config.yaml changes it.
  const loadConfig = useCallback(async () => {
    try {
      const c = await api.config();
      setTitle(c.title || project);
      setDir(c.dir);
      setDefaults(defaultsToFilters(c.filter));
      // The theme is a document-level concern (it drives html[data-theme]), not
      // React state, so it is applied here rather than rendered. Applying it on
      // every config load also means editing config.yaml recolors the board over
      // live-reload, the way the title already updates.
      applyTheme(c.theme);
      setConfigReady(true);
    } catch (e) {
      // A missing/unreachable config just leaves the defaults, but still marks
      // config as settled so the URL sync below (and the effect above) can proceed
      // — an unreachable /api/config must not stall the UI indefinitely. Deliberately
      // not setError: this can fire on a transient reconnect (e.g. thingd restarting),
      // and a banner would flash on an otherwise-healthy board. Still worth knowing
      // about, so it is not completely silent — just not user-facing.
      console.warn("GET /api/config failed; using placeholder title/dir and no configured filter defaults", e);
      setConfigReady(true);
    }
  }, [api, project]);
  useEffect(() => {
    void reload();
    void loadConfig();
  }, [reload, loadConfig]);
  // The theme belongs to the open project, so drop it on the way out: leaving it
  // set would color the root picker — and, briefly, the next project — with the
  // palette of the project just left. Root remounts this component per project,
  // so this runs on every switch as well as on the way back to the picker.
  useEffect(() => () => applyTheme(undefined), []);
  // One SSE subscription refreshes both the tree and the config (title/dir).
  const refresh = useCallback(() => {
    void reload();
    void loadConfig();
    onRefresh();
  }, [reload, loadConfig, onRefresh]);
  useLiveReload(project, refresh);
  useEffect(() => {
    document.title = title;
  }, [title]);

  // The configured defaults arrive with the config fetch, one tick after the first
  // render has already read the URL. Apply them only when the URL said nothing about
  // filters, and let the replaceState effect below spell them out in the URL so a
  // shared link does not depend on the reader's own config. Keyed on the defaults'
  // value so a live-reload refetch never resets a filter the user is working with.
  const appliedDefaults = useRef<string | null>(null);
  useEffect(() => {
    const key = filtersToQuery(defaults);
    if (appliedDefaults.current === key) return;
    appliedDefaults.current = key;
    if (!hasFilterQuery(window.location.search)) setFilters(defaults);
  }, [defaults]);

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

  // queryString is the filter portion of the URL. Before the config fetch
  // settles, `defaults` is still the emptyFilters placeholder rather than the
  // real (possibly empty) configured value, so deriving it from filters/defaults
  // here would be wrong in the same way the replaceState effect below guards
  // against: e.g. landing on ?filter=none and reading it as the placeholder's
  // "nothing configured" would turn the query into a bare URL, and the sentinel
  // for the user's explicit "show everything" would be lost the moment they
  // click a row or copy a link. So until configReady, keep whatever the URL
  // already says, verbatim.
  const queryString = useCallback(
    () => (configReady ? filtersToQuery(filters, defaults) : window.location.search),
    [configReady, filters, defaults],
  );

  // hrefFor is the URL a node's anchor points at: its ref as the path plus the
  // current filters as the query. NodeChainList rows, child rows, and the logo are real
  // <a> links so the URL is real (a ⌘/Ctrl/Shift/middle click opens the node in a
  // new tab, and the link is copyable), but a plain click is intercepted (see
  // onNav) and handled in-app rather than reloading the page.
  const hrefFor = useCallback((ref: string) => `${pathFor(ref)}${queryString()}`, [pathFor, queryString]);

  // navigate selects a node and pushes a history entry, so a plain click behaves
  // like a page navigation (Back/Forward step through visited nodes) without the
  // reload — no refetching the tree, reopening the SSE stream, or piling requests
  // onto the connection pool.
  const navigate = useCallback(
    (ref: string) => {
      window.history.pushState(null, "", `${pathFor(ref)}${queryString()}`);
      setActiveRef(ref || null);
    },
    [pathFor, queryString],
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
  //
  // Wait for the config fetch to settle first. Before it does, `defaults` is
  // still the emptyFilters placeholder, not the real (possibly empty) configured
  // value — writing the URL from it here would encode ?filter=none (an explicit
  // "no filters", which reads as empty against a placeholder empty defaults) as a
  // bare URL, permanently erasing the sentinel before the real defaults arrive.
  useEffect(() => {
    if (!configReady) return;
    window.history.replaceState(null, "", `${pathFor(activeRef ?? "")}${filtersToQuery(filters, defaults)}`);
  }, [pathFor, activeRef, filters, defaults, configReady]);

  // Restore the view from the URL on Back/Forward (a popstate, which pushState
  // navigation produces).
  useEffect(() => {
    const onPop = () => {
      setActiveRef(refFromPath(project));
      setFilters(filtersFromQuery(window.location.search, defaults));
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, [project, defaults]);

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
        <div className={s.brandGroup}>
          <a className={s.brand} href={hrefFor("")} onClick={(e) => onNav(e, "")}>
            <span className={s.dot} />{title}
          </a>
          <ProjectSwitcher current={project} onSwitch={onSwitch} />
        </div>
        <div className={s.topbarAdd}>
          <NodeFormDialog api={api} parent="" noun="Epic" amber run={run} onCreated={activate} />
        </div>
      </header>
      {/* Fixed to the viewport's corner, so it is a sibling of the panes rather
          than part of the bar's layout. */}
      <SchemeMenu scheme={scheme} onChange={onScheme} />

      {error && <div className={s.error} onClick={() => setError(null)}>{error}</div>}

      <div className={s.split}>
        <FilterForm
          filters={filters}
          defaults={defaults}
          categories={categories}
          tags={tags}
          statusCounts={statusCounts}
          priorityCounts={priorityCounts}
          onChange={setFilters}
        />

        <section className={s.treePane}>
          {dir && <div className={s.dir}>{dir}</div>}
          <NodeChainList nodes={filtered} activeRef={activeRef} hrefFor={hrefFor} onNav={onNav} expanded={fold.expanded} onToggle={fold.toggle} />
          <ArchiveList api={api} entries={archived} run={run} />
        </section>

        <section className={s.detailPane}>
          {activeNode ? (
            <NodeDetailPanel key={activeNode.ref} api={api} node={activeNode} allNodes={tree} run={run} onSelect={activate} hrefFor={hrefFor} onNav={onNav} />
          ) : (
            <p className={s.empty}>Select a node to view and edit it.</p>
          )}
        </section>
      </div>
    </div>
  );
}
