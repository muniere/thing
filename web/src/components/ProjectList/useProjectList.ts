import { type DragEvent, type MouseEvent, useCallback, useEffect, useState } from "react";
import { listProjects, listThemes, moveProject, type ProjectInfo, reloadProjects, unregisterProject } from "../../api.ts";
import { loadThemeMarks } from "../../theme.ts";
import { isPlainClick } from "../../util.ts";

interface Input {
  // Open a project (push /<name> and switch the view to it).
  onOpen: (name: string) => void;
}

// DropHint marks where a dragged card would land: before or after another card.
export interface DropHint {
  name: string;
  pos: "before" | "after";
}

export interface ProjectListState {
  // The registered projects, or null until the first fetch lands.
  projects: ProjectInfo[] | null;
  // The page-level banner's message, or null when there is nothing to say.
  error: string | null;
  // The themes thingd can serve, for the form dialog to offer. Empty until the
  // fetch lands, and left empty if it fails — the dialog then omits the field
  // rather than the picker failing over a cosmetic setting.
  themes: string[];
  // Re-read projects.yaml on the server and re-sync from it.
  reload: {
    running: boolean;
    start: () => void;
  };
  // The per-row kebab menu: which row's is open, and how to work it.
  menu: {
    openFor: string | null;
    toggle: (name: string) => void;
    close: () => void;
  };
  // The open form dialog, or null when none is. A wrapper object rather than the
  // project itself, so "adding" (no project) stays distinct from "closed".
  form: {
    current: { project?: ProjectInfo } | null;
    add: () => void;
    edit: (project: ProjectInfo) => void;
    close: () => void;
    saved: () => void;
    failed: (message: string) => void;
  };
  // Open a project card. A modified or middle click is left to the browser so the
  // card's href opens in a new tab like any other link.
  open: (e: MouseEvent, name: string) => void;
  // Unregister a project, leaving its data directory on disk.
  remove: (name: string) => void;
  // Drag-and-drop reorder. `dragging` is the card being dragged; `hint` marks
  // where it would land, which the list draws as an insertion line.
  drag: {
    dragging: string | null;
    hint: DropHint | null;
    start: (name: string) => void;
    over: (e: DragEvent, name: string) => void;
    drop: () => void;
    end: () => void;
  };
}

// useProjectList holds the root picker's state: the registered projects, the
// menus and dialog opened over them, and the drag-to-reorder that writes their
// order back to the server. The picker itself is left to be markup.
export function useProjectList({ onOpen }: Input): ProjectListState {
  const [projects, setProjects] = useState<ProjectInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [themes, setThemes] = useState<string[]>([]);
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [form, setForm] = useState<{ project?: ProjectInfo } | null>(null);
  const [reloading, setReloading] = useState(false);
  const [dragName, setDragName] = useState<string | null>(null);
  const [hint, setHint] = useState<DropHint | null>(null);

  const load = useCallback(() => {
    listProjects()
      .then((ps) => {
        setProjects(ps);
        // The marks on the cards read each project's own palette, which means
        // loading the themes these projects use — not every theme that exists.
        loadThemeMarks(ps.map((p) => p.theme).filter((t): t is string => !!t));
        setError(null);
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)));
  }, []);

  useEffect(() => {
    document.title = "thing";
    load();
    listThemes()
      .then(setThemes)
      .catch((e) => console.warn("GET /api/themes failed; the theme field is hidden", e));
  }, [load]);

  // Close the open kebab menu on any outside click or Escape.
  useEffect(() => {
    if (menuFor === null) return;
    const close = () => setMenuFor(null);
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setMenuFor(null);
    window.addEventListener("click", close);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("keydown", onKey);
    };
  }, [menuFor]);

  // refresh re-syncs the list from the server's projects.yaml (picking up
  // hand-edits) and reports any entries that could not be mounted. It fetches the
  // list itself rather than calling load() so the skipped-entry message it sets
  // isn't cleared by load()'s own success handler.
  const refresh = async () => {
    setReloading(true);
    try {
      const res = await reloadProjects();
      setProjects(await listProjects());
      setError(res.skipped.length ? res.skipped.map((x) => `${x.name}: ${x.reason}`).join("; ") : null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setReloading(false);
    }
  };

  const remove = async (name: string) => {
    setMenuFor(null);
    if (!window.confirm(`Unregister "${name}"? Its data directory stays on disk.`)) return;
    try {
      await unregisterProject(name);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  // dragOver decides whether the pointer is over the top or bottom half of the
  // hovered card and records that as the drop position.
  const dragOver = (e: DragEvent, name: string) => {
    if (dragName === null || name === dragName) return;
    e.preventDefault(); // allow the drop
    e.dataTransfer.dropEffect = "move";
    const r = e.currentTarget.getBoundingClientRect();
    const pos = e.clientY < r.top + r.height / 2 ? "before" : "after";
    setHint((h) => (h?.name === name && h.pos === pos ? h : { name, pos }));
  };

  // drop moves the dragged project relative to the hovered card. It reorders the
  // list optimistically, then persists; a failure reloads the true order.
  const drop = async () => {
    const from = dragName;
    const target = hint;
    setDragName(null);
    setHint(null);
    if (!from || !target || from === target.name || !projects) return;

    const rest = projects.filter((p) => p.name !== from);
    const anchor = rest.findIndex((p) => p.name === target.name);
    const at = target.pos === "before" ? anchor : anchor + 1;
    const moved = projects.find((p) => p.name === from);
    if (anchor < 0 || !moved) return;
    setProjects([...rest.slice(0, at), moved, ...rest.slice(at)]);

    try {
      await moveProject(from, target.pos === "before" ? { before: target.name } : { after: target.name });
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      load(); // revert to the server's order
    }
  };

  return {
    projects,
    error,
    themes,
    reload: { running: reloading, start: () => void refresh() },
    menu: {
      openFor: menuFor,
      toggle: (name) => setMenuFor((cur) => (cur === name ? null : name)),
      close: () => setMenuFor(null),
    },
    form: {
      current: form,
      add: () => setForm({}),
      edit: (project) => {
        setMenuFor(null);
        setForm({ project });
      },
      close: () => setForm(null),
      saved: () => {
        setForm(null);
        load();
      },
      failed: (message) => {
        setForm(null);
        setError(message);
      },
    },
    open: (e, name) => {
      if (!isPlainClick(e)) return;
      e.preventDefault();
      onOpen(name);
    },
    remove: (name) => void remove(name),
    drag: {
      dragging: dragName,
      hint,
      start: setDragName,
      over: dragOver,
      drop: () => void drop(),
      end: () => {
        setDragName(null);
        setHint(null);
      },
    },
  };
}
