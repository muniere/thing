import { ProjectFormDialog } from "../ProjectFormDialog/ProjectFormDialog.tsx";
import { SchemeMenu } from "../SchemeMenu/SchemeMenu.tsx";
import { useProjectList } from "./useProjectList.ts";
import type { Scheme } from "../../api.ts";
import s from "./ProjectList.module.css";

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
  const { projects, error, themes, reload, menu, form, open, remove, drag } = useProjectList({ onOpen });

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
            <button type="button" className={`${s.btn} ${s.btnAmber}`} onClick={form.add}>
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
              disabled={reload.running}
              onClick={reload.start}
            >
              {/* Refresh (circular arrow) as an SVG, not a glyph, so it aligns with
                  the label and can spin while the reload is in flight. The label
                  names the action — an icon alone reads as ambiguous. */}
              <svg className={reload.running ? `${s.refreshIcon} ${s.spinning}` : s.refreshIcon} viewBox="0 0 24 24" aria-hidden="true">
                <path d="M21 12a9 9 0 1 1-2.64-6.36" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                <path d="M21 3v6h-6" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              {reload.running ? "Reloading…" : "Reload"}
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
                drag.dragging === p.name ? s.dragging : "",
                drag.hint?.name === p.name ? (drag.hint.pos === "before" ? s.dropBefore : s.dropAfter) : "",
              ].filter(Boolean).join(" ")}
              draggable
              onDragStart={(e) => {
                drag.start(p.name);
                e.dataTransfer.effectAllowed = "move";
              }}
              onDragOver={(e) => drag.over(e, p.name)}
              onDrop={drag.drop}
              onDragEnd={drag.end}
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
                  aria-expanded={menu.openFor === p.name}
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    menu.toggle(p.name);
                  }}
                >
                  ⋯
                </button>
                {menu.openFor === p.name && (
                  <div className={s.menu} role="menu" onClick={(e) => e.stopPropagation()}>
                    <button
                      type="button"
                      role="menuitem"
                      className={s.menuItem}
                      onClick={() => form.edit(p)}
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

      {form.current && (
        <ProjectFormDialog
          project={form.current.project}
          themes={themes}
          onClose={form.close}
          onSaved={form.saved}
          onError={form.failed}
        />
      )}
    </div>
  );
}
