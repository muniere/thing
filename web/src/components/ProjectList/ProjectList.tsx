import { type DragEvent, type MouseEvent, useCallback, useEffect, useState } from "react";
import { listProjects, listThemes, moveProject, type ProjectInfo, reloadProjects, type Scheme, unregisterProject } from "../../api.ts";
import { isPlainClick } from "../../util.ts";
import { ProjectFormDialog } from "../ProjectFormDialog/ProjectFormDialog.tsx";
import { SchemeMenu } from "../SchemeMenu/SchemeMenu.tsx";
import { loadThemeMarks } from "../../theme.ts";
import s from "./ProjectList.module.css";

// DropHint marks where a dragged card would land: before or after another card.
interface DropHint {
  name: string;
  pos: "before" | "after";
}

interface Props {
  // Open a project (push /<name> and switch the view to it).
  onOpen: (name: string) => void;
  // The server-wide color scheme and the setter for it, both owned by Root.
  scheme: Scheme;
  onScheme: (scheme: Scheme) => void;
}

// ProjectList is the root picker shown at "/": a card per registered project,
// linking into /<name>. Each card is a real <a> so a modified/middle click opens
// the project in a new tab; a plain click is handled in-app via onOpen. Projects
// can also be registered (from an existing thing tree), edited, and unregistered
// here, which writes back to projects.yaml on the server. Registering and editing
// share one ProjectFormDialog — the same fields either way — which this component
// mounts only while it is open, so its draft state resets between openings.
export function ProjectList({ onOpen, scheme, onScheme }: Props) {
  const [projects, setProjects] = useState<ProjectInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Which row's kebab menu is open, by project name; null when none is.
  const [menuFor, setMenuFor] = useState<string | null>(null);
  // The open form dialog, or null when none is. A wrapper object rather than the
  // project itself, so "adding" (no project) stays distinct from "closed".
  const [form, setForm] = useState<{ project?: ProjectInfo } | null>(null);
  // The themes thingd can serve, offered in the form dialog. Empty until the
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
            {/* Mirrors the in-project "+ Epic": a button that opens the form as a
                modal, rather than a bare field in the bar. */}
            <button type="button" className={`${s.btn} ${s.btnAmber}`} onClick={() => setForm({})}>
              + Project
            </button>
          </div>
        )}
      </header>
      {/* Fixed to the viewport's corner, so it is a sibling of the content rather
          than part of the bar's layout. */}
      <SchemeMenu scheme={scheme} onChange={onScheme} />

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
                        setForm({ project: p });
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

      {form && (
        <ProjectFormDialog
          project={form.project}
          themes={themes}
          onClose={() => setForm(null)}
          onSaved={() => {
            setForm(null);
            load();
          }}
          onError={(m) => {
            setForm(null);
            setError(m);
          }}
        />
      )}
    </div>
  );
}
