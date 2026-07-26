import { type MouseEvent, useCallback, useEffect, useState } from "react";
import { listProjects, type ProjectInfo, registerProject, unregisterProject } from "../api.ts";
import { isPlainClick } from "../util.ts";
import { Dialog } from "./Dialog.tsx";
import s from "./ProjectList.module.css";

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

  const load = useCallback(() => {
    listProjects()
      .then((ps) => {
        setProjects(ps);
        setError(null);
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)));
  }, []);

  useEffect(() => {
    document.title = "thing";
    load();
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

        {projects && projects.length === 0 && (
          <p className={s.empty}>No projects registered yet — add one below.</p>
        )}

      {projects && projects.length > 0 && (
        <ul className={s.grid}>
          {projects.map((p) => (
            <li key={p.name} className={s.item}>
              <a className={s.card} href={`/${p.name}`} onClick={(e) => open(e, p.name)}>
                <span className={s.cardTitle}>{p.title}</span>
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
        <input
          className={s.input}
          autoFocus
          placeholder="name (url-safe: a-z 0-9 -)"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <input
          className={s.input}
          placeholder="data directory (an existing thing tree)"
          value={dir}
          onChange={(e) => setDir(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") submit();
          }}
        />
        <p className={s.hint}>
          The directory must already be a thing tree. Create a new one with <code>thing init</code> first.
        </p>
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
