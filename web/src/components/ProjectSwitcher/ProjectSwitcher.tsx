import { type MouseEvent, useEffect, useState } from "react";
import { listProjects, type ProjectInfo } from "../../lib/api.ts";
import { loadThemeMarks } from "../../lib/theme.ts";
import { isPlainClick } from "../../lib/util.ts";
import s from "./ProjectSwitcher.module.css";

interface Props {
  // The currently open project's name; its row is marked with a check.
  current: string;
  // Switch to another project by name, or to the picker (null for "All
  // projects"). Root routes both by remounting Board on its project key.
  onSwitch: (name: string | null) => void;
}

// ProjectSwitcher is the caret beside the in-project logo: a Firebase-style
// dropdown that lists the registered projects and jumps to one without returning
// to the picker. The list is fetched the first time the panel opens (and on each
// reopen, so a project registered elsewhere shows up), so a session that never
// touches the switcher never pays for the request.
export function ProjectSwitcher({ current, onSwitch }: Props) {
  const [open, setOpen] = useState(false);
  const [projects, setProjects] = useState<ProjectInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    listProjects()
      .then((ps) => {
        setProjects(ps);
        // The marks read each project's own palette, so the themes these projects
        // use have to be loaded — not every theme that exists.
        loadThemeMarks(ps.map((p) => p.theme).filter((t): t is string => !!t));
        setError(null);
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)));
  }, [open]);

  // Close on any outside click or Escape, mirroring the picker's kebab menu. The
  // caret's own click stops propagation so it toggles rather than being closed by
  // this same listener.
  useEffect(() => {
    if (!open) return;
    const close = () => setOpen(false);
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    window.addEventListener("click", close);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("keydown", onKey);
    };
  }, [open]);

  // Each row is a real <a> (href to /<name>, or "/" for the picker) so a
  // ⌘/Ctrl/Shift/middle click opens it in a new tab like any link. A plain click
  // is intercepted and handled in-app instead.
  const pick = (e: MouseEvent, name: string | null) => {
    if (!isPlainClick(e)) return;
    e.preventDefault();
    setOpen(false);
    // Picking the current project is a no-op; the panel just closes.
    if (name === current) return;
    onSwitch(name);
  };

  return (
    <div className={s.wrap}>
      <button
        type="button"
        className={s.caret}
        title="Switch project"
        aria-label="Switch project"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
          setOpen((v) => !v);
        }}
      >
        {/* An SVG chevron rather than a "⌄" glyph: the glyph's ink sits high in
            its em box and drifts off the logo's baseline in a way that varies by
            font, whereas the SVG centers predictably in the button. */}
        <svg className={s.chev} viewBox="0 0 10 6" aria-hidden="true">
          <path d="M1 1l4 4 4-4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
      {open && (
        <div className={s.menu} role="menu" onClick={(e) => e.stopPropagation()}>
          {error && <div className={s.error}>{error}</div>}
          {projects?.map((p) => (
            <a
              key={p.name}
              href={`/${p.name}`}
              role="menuitem"
              className={p.name === current ? `${s.item} ${s.current}` : s.item}
              onClick={(e) => pick(e, p.name)}
            >
              <span className={s.check} aria-hidden="true">{p.name === current ? "✓" : ""}</span>
              {/* The mark carries the project's own data-theme, so switching is a
                  choice between colors you can see rather than names you have to
                  remember. */}
              <span className={s.mark} data-theme={p.theme || undefined} aria-hidden="true" />
              <span className={s.label}>
                <span className={s.title}>{p.title}</span>
                <span className={s.name}>/{p.name}</span>
              </span>
            </a>
          ))}
          {projects && projects.length > 0 && <div className={s.sep} />}
          <a href="/" role="menuitem" className={s.item} onClick={(e) => pick(e, null)}>
            <span className={s.check} aria-hidden="true" />
            <span className={s.label}>All projects</span>
          </a>
        </div>
      )}
    </div>
  );
}
