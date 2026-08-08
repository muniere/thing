import { type DragEvent, type MouseEvent, useCallback, useEffect, useState } from "react";
import { editProject, listProjects, listThemes, moveProject, type ProjectInfo, registerProject, reloadProjects, unregisterProject } from "../api.ts";
import { isPlainClick } from "../util.ts";
import { Dialog } from "./Dialog.tsx";
import { ThemePreview } from "./ThemePreview.tsx";
import { loadThemeMarks } from "../theme.ts";
import s from "./ProjectList.module.css";

// DropHint marks where a dragged card would land: before or after another card.
interface DropHint {
  name: string;
  pos: "before" | "after";
}

interface Props {
  // Open a project (push /<name> and switch the view to it).
  onOpen: (name: string) => void;
}

// ProjectList is the root picker shown at "/": a card per registered project,
// linking into /<name>. Each card is a real <a> so a modified/middle click opens
// the project in a new tab; a plain click is handled in-app via onOpen. Projects
// can also be registered (from an existing thing tree) and unregistered here,
// which writes back to projects.yaml on the server.
export function ProjectList({ onOpen }: Props) {
  const [projects, setProjects] = useState<ProjectInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Which row's kebab menu is open, by project name; null when none is.
  const [menuFor, setMenuFor] = useState<string | null>(null);
  // The project whose edit dialog is open, or null when none is.
  const [editing, setEditing] = useState<ProjectInfo | null>(null);
  // The themes thingd can serve, offered in the edit dialog. Empty until the
  // fetch lands, and left empty if it fails — the dialog then omits the field
  // rather than the picker failing over a cosmetic setting.
  const [themes, setThemes] = useState<string[]>([]);

  const load = useCallback(() => {
    listProjects()
      .then((ps) => {
        setProjects(ps);
        // The marks below read each project's own palette, which means loading
        // the themes these projects use — not every theme that exists.
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

  const open = (e: MouseEvent, name: string) => {
    if (!isPlainClick(e)) return;
    e.preventDefault();
    onOpen(name);
  };

  // refresh re-syncs the list from the server's projects.yaml (picking up
  // hand-edits) and reports any entries that could not be mounted. It fetches the
  // list itself rather than calling load() so the skipped-entry message it sets
  // isn't cleared by load()'s own success handler.
  const [reloading, setReloading] = useState(false);
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

  // Drag-and-drop reorder. dragName is the card being dragged; hint marks where it
  // would land (before/after another card), shown as an insertion line.
  const [dragName, setDragName] = useState<string | null>(null);
  const [hint, setHint] = useState<DropHint | null>(null);

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

  return (
    <div className={s.page}>
      <header className={s.topbar}>
        <span className={s.brand}>
          <span className={s.dot} />thing
        </span>
        {projects && (
          <div className={s.topbarAdd}>
            <AddProject onAdded={load} onError={setError} />
          </div>
        )}
      </header>

      <div className={s.content}>
        {error && <div className={s.error}>{error}</div>}

        {projects && (
          <div className={s.listHead}>
            <span className={s.count}>{projects.length} project{projects.length === 1 ? "" : "s"}</span>
            <button
              type="button"
              className={s.refresh}
              title="Re-read projects.yaml on the server"
              disabled={reloading}
              onClick={refresh}
            >
              {/* Refresh (circular arrow) as an SVG, not a glyph, so it aligns with
                  the label and can spin while the reload is in flight. The label
                  names the action — an icon alone reads as ambiguous. */}
              <svg className={reloading ? `${s.refreshIcon} ${s.spinning}` : s.refreshIcon} viewBox="0 0 24 24" aria-hidden="true">
                <path d="M21 12a9 9 0 1 1-2.64-6.36" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                <path d="M21 3v6h-6" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              {reloading ? "Reloading…" : "Reload"}
            </button>
          </div>
        )}

        {projects && projects.length === 0 && (
          <p className={s.empty}>No projects registered yet — add one below.</p>
        )}

      {projects && projects.length > 0 && (
        <ul className={s.grid}>
          {projects.map((p) => (
            <li
              key={p.name}
              className={[
                s.item,
                dragName === p.name ? s.dragging : "",
                hint?.name === p.name ? (hint.pos === "before" ? s.dropBefore : s.dropAfter) : "",
              ].filter(Boolean).join(" ")}
              draggable
              onDragStart={(e) => {
                setDragName(p.name);
                e.dataTransfer.effectAllowed = "move";
              }}
              onDragOver={(e) => dragOver(e, p.name)}
              onDrop={drop}
              onDragEnd={() => {
                setDragName(null);
                setHint(null);
              }}
            >
              <span className={s.grip} aria-hidden="true">⠿</span>
              {/* The card is not itself draggable, so its default link-drag doesn't
                  fight the row's reorder drag. */}
              <a className={s.card} href={`/${p.name}`} draggable={false} onClick={(e) => open(e, p.name)}>
                <span className={s.cardHead}>
                  {/* The mark carries the project's own data-theme, so it is that
                      board's accent rather than a color assigned here. A project
                      on no theme inherits the picker's, which is what its board
                      will look like. */}
                  <span className={s.mark} data-theme={p.theme || undefined} aria-hidden="true" />
                  <span className={s.cardTitle}>{p.title}</span>
                </span>
                {/* Always show the slug, prefixed with "/" so it reads as the URL
                    path (/<name>) rather than a repeat of the title. */}
                <span className={s.cardName}>/{p.name}</span>
                <span className={s.cardDir}>{p.dir}</span>
              </a>
              <div className={s.menuWrap}>
                <button
                  type="button"
                  className={s.kebab}
                  title={`Actions for ${p.name}`}
                  aria-label={`Actions for ${p.name}`}
                  aria-haspopup="menu"
                  aria-expanded={menuFor === p.name}
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    setMenuFor((cur) => (cur === p.name ? null : p.name));
                  }}
                >
                  ⋯
                </button>
                {menuFor === p.name && (
                  <div className={s.menu} role="menu" onClick={(e) => e.stopPropagation()}>
                    <button
                      type="button"
                      role="menuitem"
                      className={s.menuItem}
                      onClick={() => {
                        setMenuFor(null);
                        setEditing(p);
                      }}
                    >
                      Edit
                    </button>
                    <button
                      type="button"
                      role="menuitem"
                      className={s.menuDanger}
                      onClick={() => remove(p.name)}
                    >
                      Unregister
                    </button>
                  </div>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      </div>

      {editing && (
        <EditProject
          project={editing}
          themes={themes}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            load();
          }}
          onError={(m) => {
            setEditing(null);
            setError(m);
          }}
        />
      )}
    </div>
  );
}

interface AddProps {
  // Reload the list after a successful registration.
  onAdded: () => void;
  // Surface a registration error in the page-level banner.
  onError: (message: string) => void;
}

// AddProject is the top-bar "+ Project" action, mirroring the in-project "+ Epic":
// a button that drops a floating register form (a URL-safe name and the path to
// an existing thing tree). The server only mounts an already-initialized tree, so
// the hint points at `thing init` for new ones.
function AddProject({ onAdded, onError }: AddProps) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [dir, setDir] = useState("");

  const close = () => {
    setOpen(false);
    setName("");
    setDir("");
  };

  const submit = async () => {
    const n = name.trim();
    const d = dir.trim();
    if (!n || !d) return;
    try {
      await registerProject(n, d);
      close();
      onAdded();
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <>
      <button type="button" className={`${s.btn} ${s.btnAmber}`} onClick={() => setOpen(true)}>
        + Project
      </button>
      <Dialog open={open} onClose={close} title="Add project">
        <label className={s.field}>
          <span className={s.fieldLabel}>Name</span>
          <input
            className={s.input}
            autoFocus
            placeholder="my-project"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <span className={s.fieldHint}>
            Lower-case letters, digits, and dashes. It becomes the project's URL.
          </span>
        </label>
        <label className={s.field}>
          <span className={s.fieldLabel}>Data directory</span>
          <input
            className={s.input}
            placeholder="/path/to/project/.thing"
            value={dir}
            onChange={(e) => setDir(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") submit();
            }}
          />
          <span className={s.fieldHint}>
            Must already be a thing tree. Create one with <code>thing init</code> first.
          </span>
        </label>
        <div className={s.actions}>
          <button type="button" className={`${s.btn} ${s.btnAmber}`} onClick={submit} disabled={!name.trim() || !dir.trim()}>
            Add
          </button>
          <button type="button" className={s.btnLink} onClick={close}>
            Cancel
          </button>
        </div>
      </Dialog>
    </>
  );
}

interface EditProps {
  // The project being edited; its name, dir, and theme seed the form.
  project: ProjectInfo;
  // The themes available to choose from. Empty hides the field.
  themes: string[];
  // Reload the list after a successful edit.
  onSaved: () => void;
  // Surface an edit error in the page-level banner.
  onError: (message: string) => void;
  // Dismiss the dialog without saving.
  onClose: () => void;
}

// EditProject is the kebab "Edit" dialog: it edits a registered project's name
// and data directory, mirroring AddProject's form but seeded with the current
// values. It sends only the fields that changed, so a rename that leaves the
// directory alone does not also re-point it (and vice versa).
function EditProject({ project, themes, onSaved, onError, onClose }: EditProps) {
  const [name, setName] = useState(project.name);
  const [dir, setDir] = useState(project.dir);
  const [theme, setTheme] = useState(project.theme ?? "");

  const submit = async () => {
    const n = name.trim();
    const d = dir.trim();
    if (!n || !d) return;
    const changes: { name?: string; dir?: string; theme?: string } = {};
    if (n !== project.name) changes.name = n;
    if (d !== project.dir) changes.dir = d;
    // "" is a real value here — it clears the project's own theme — so this
    // compares against the current one rather than testing for emptiness.
    if (theme !== (project.theme ?? "")) changes.theme = theme;
    if (!changes.name && !changes.dir && changes.theme === undefined) {
      onClose(); // nothing changed
      return;
    }
    try {
      await editProject(project.name, changes);
      onSaved();
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <Dialog open onClose={onClose} title="Edit project" size="wide">
      <label className={s.field}>
        <span className={s.fieldLabel}>Name</span>
        <input
          className={s.input}
          autoFocus
          placeholder="my-project"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <span className={s.fieldHint}>
          Renaming changes the project's URL (<code>/{(name.trim() || project.name)}</code>).
        </span>
      </label>
      <label className={s.field}>
        <span className={s.fieldLabel}>Data directory</span>
        <input
          className={s.input}
          placeholder="/path/to/project/.thing"
          value={dir}
          onChange={(e) => setDir(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") submit();
          }}
        />
      </label>
      {themes.length > 0 && (
        <label className={s.field}>
          <span className={s.fieldLabel}>Theme</span>
          <select className={s.select} value={theme} onChange={(e) => setTheme(e.target.value)}>
            <option value="">Default</option>
            {themes.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
          <ThemePreview theme={theme} />
        </label>
      )}
      <div className={s.actions}>
        <button type="button" className={`${s.btn} ${s.btnAmber}`} onClick={submit} disabled={!name.trim() || !dir.trim()}>
          Save
        </button>
        <button type="button" className={s.btnLink} onClick={onClose}>
          Cancel
        </button>
      </div>
    </Dialog>
  );
}
