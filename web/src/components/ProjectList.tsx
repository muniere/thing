import { type MouseEvent, useEffect, useState } from "react";
import { listProjects, type ProjectInfo } from "../api.ts";
import { isPlainClick } from "../util.ts";
import s from "./ProjectList.module.css";

interface Props {
  // Open a project (push /<name> and switch the view to it).
  onOpen: (name: string) => void;
}

// ProjectList is the root picker shown at "/": a card per registered project,
// linking into /<name>. Each card is a real <a> so a modified/middle click opens
// the project in a new tab; a plain click is handled in-app via onOpen.
export function ProjectList({ onOpen }: Props) {
  const [projects, setProjects] = useState<ProjectInfo[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    document.title = "thing";
    listProjects()
      .then(setProjects)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)));
  }, []);

  const open = (e: MouseEvent, name: string) => {
    if (!isPlainClick(e)) return;
    e.preventDefault();
    onOpen(name);
  };

  return (
    <div className={s.page}>
      <header className={s.head}>
        <span className={s.dot} />
        <h1 className={s.title}>thing</h1>
      </header>

      {error && <div className={s.error}>{error}</div>}

      {projects && projects.length === 0 && (
        <p className={s.empty}>
          No projects registered yet. Add one to <code>projects.yaml</code> to get started.
        </p>
      )}

      {projects && projects.length > 0 && (
        <ul className={s.grid}>
          {projects.map((p) => (
            <li key={p.name}>
              <a className={s.card} href={`/${p.name}`} onClick={(e) => open(e, p.name)}>
                <span className={s.cardTitle}>{p.title}</span>
                <span className={s.cardName}>{p.name}</span>
                <span className={s.cardDir}>{p.dir}</span>
              </a>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
