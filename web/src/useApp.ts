import { type MouseEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Node } from "./domain/generated.ts";
import type { ArchiveEntry } from "./api.ts";
import {
  collectCategories,
  collectPriorityCounts,
  collectStatusCounts,
  collectTags,
  filterTree,
  filtersActive,
  filtersFromQuery,
  filtersToQuery,
  hasFilterQuery,
  type Filters,
} from "./filter.ts";
import { findNode, isPlainClick } from "./util.ts";
import { type ProjectState, useProject } from "./useProject.ts";
import { type TreeFold, useTreeFold, useTreeNav } from "./tree.ts";

interface Input {
  // The project this view is scoped to (the first URL path segment).
  project: string;
  // Re-read the server-wide settings. Called on live-reload, since a scheme
  // changed in another tab arrives the same way a tree change does.
  onRefresh: () => void;
}

export interface AppState {
  // The per-project API client, for the components that mutate through it.
  api: ProjectState["api"];
  // The configured title (also the browser tab's) and the data directory the
  // board is rooted at, both from the project's config.yaml.
  title: string;
  dir: string;
  // The whole tree, and the pruned view of it the current filters select.
  tree: Node[];
  filtered: Node[];
  // The shelved subtrees listed below the tree.
  archived: ArchiveEntry[];
  // The board-level error banner, and the dismissal the banner's click performs.
  error: string | null;
  dismissError: () => void;
  // Await a mutation, surface any error, then reload the tree.
  run: <T>(p: Promise<T>) => Promise<T | undefined>;
  // The filter state, the configured starting point it resets to, and the
  // choices and counts the controls offer.
  filters: Filters;
  setFilters: (filters: Filters) => void;
  defaults: Filters;
  categories: string[];
  tags: string[];
  statusCounts: Record<string, number>;
  priorityCounts: Record<string, number>;
  // The selected node: its ref, the node itself, and how to select another.
  activeRef: string | null;
  activeNode: Node | null;
  activate: (ref: string) => void;
  // Which rows are unfolded, driven by the selection and the filters.
  fold: TreeFold;
  // The URL a node's anchor points at, and the click handler that navigates
  // in-app rather than reloading.
  hrefFor: (ref: string) => string;
  onNav: (e: MouseEvent, ref: string) => void;
}

// The URL path is /<project>/<ref> (e.g. /work/epic/issue/task); /<project> alone
// means nothing is active. Refs are slug paths ([a-z0-9-] joined by "/"), so they
// are URL-safe as-is. Filters and search live in the query string instead.
function refFromPath(project: string): string | null {
  const path = window.location.pathname.replace(/^\/+/, "").replace(/\/+$/, "");
  const prefix = `${project}/`;
  return path.startsWith(prefix) ? path.slice(prefix.length) || null : null;
}

// useApp holds a board's state: what is selected and filtered, and the URL that
// mirrors both. What was loaded lives in useProject; App itself is left to be
// layout.
export function useApp({ project, onRefresh }: Input): AppState {
  const { api, title, dir, tree, archived, defaults, configReady, error, dismissError, run } = useProject({
    project,
    onRefresh,
  });
  const [activeRef, setActiveRef] = useState<string | null>(() => refFromPath(project));
  const [filters, setFilters] = useState<Filters>(() => filtersFromQuery(window.location.search));

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

  // pathFor builds the URL path for a node ref within this project: /<project> at
  // the root, /<project>/<ref> for a node.
  const pathFor = useCallback((ref: string) => (ref ? `/${project}/${ref}` : `/${project}`), [project]);

  // activate makes a node the focused one (or clears it when given ""); the tree
  // and the detail panel fire it when the user picks a node.
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
  // current filters as the query. Tree rows, child rows, and the logo are real
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

  return {
    api,
    title,
    dir,
    tree,
    filtered,
    archived,
    error,
    dismissError,
    run,
    filters,
    setFilters,
    defaults,
    categories,
    tags,
    statusCounts,
    priorityCounts,
    activeRef,
    activeNode,
    activate,
    fold,
    hrefFor,
    onNav,
  };
}
