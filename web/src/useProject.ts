import { useCallback, useEffect, useMemo, useState } from "react";
import type { Node } from "./domain/generated.ts";
import { type ArchiveEntry, forProject } from "./api.ts";
import { useLiveReload } from "./live.ts";
import { defaultsToFilters, emptyFilters, type Filters } from "./filter.ts";
import { applyTheme } from "./theme.ts";

interface Input {
  // The project to load (the first URL path segment).
  project: string;
  // Re-read the server-wide settings. Called on live-reload, since a scheme
  // changed in another tab arrives the same way a tree change does.
  onRefresh: () => void;
}

export interface ProjectState {
  // The per-project API client, for the components that mutate through it.
  api: ReturnType<typeof forProject>;
  // The configured title (also the browser tab's) and the data directory the
  // project is rooted at, both from its config.yaml.
  title: string;
  dir: string;
  // The whole tree, and the shelved subtrees beside it.
  tree: Node[];
  archived: ArchiveEntry[];
  // The configured filter defaults, which only the board applies — they are read
  // from the same config fetch as the title, so they arrive here.
  defaults: Filters;
  // Whether the config fetch has settled (either way — see loadConfig's catch).
  // Until then `defaults` is a placeholder, not "no configured defaults", which
  // the board's URL sync has to account for.
  configReady: boolean;
  // Whether the first tree fetch has settled. `tree` starts empty, so a view that
  // resolves a single ref needs this to tell "still loading" from "no such node".
  treeReady: boolean;
  // The error banner, and the dismissal a click on it performs.
  error: string | null;
  dismissError: () => void;
  // Await a mutation, surface any error, then reload the tree.
  run: <T>(p: Promise<T>) => Promise<T | undefined>;
}

// useProject loads one project and keeps it current: its tree, its archive, its
// config, and the live-reload subscription that refreshes all three. It holds
// nothing about how the project is being looked at — no selection, no filters —
// so both the board and a single node's screen can sit on top of it.
export function useProject({ project, onRefresh }: Input): ProjectState {
  const api = useMemo(() => forProject(project), [project]);
  const [tree, setTree] = useState<Node[]>([]);
  const [archived, setArchived] = useState<ArchiveEntry[]>([]);
  const [title, setTitle] = useState(project);
  const [dir, setDir] = useState("");
  const [defaults, setDefaults] = useState<Filters>(emptyFilters);
  const [configReady, setConfigReady] = useState(false);
  const [treeReady, setTreeReady] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    // The tree is the primary view; the archive list is supplementary. Fetch both
    // together but handle them independently so a failing archive fetch never
    // blanks the tree.
    const [t, a] = await Promise.allSettled([api.tree(), api.listArchive()]);
    // Settled either way: a view waiting on this must not hang on a failed fetch,
    // and the error banner already says what went wrong.
    setTreeReady(true);
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
      // config as settled so the URL sync in useApp (and the effect there) can proceed
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
  // palette of the project just left. The views above remount per project, so
  // this runs on every switch as well as on the way back to the picker.
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

  return {
    api,
    title,
    dir,
    tree,
    archived,
    defaults,
    configReady,
    treeReady,
    error,
    dismissError: () => setError(null),
    run,
  };
}
