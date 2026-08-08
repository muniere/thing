import { useEffect, useState } from "react";
import { editProject, type ProjectInfo, registerProject } from "../../api.ts";
import { loadThemesForPreview } from "../../theme.ts";
import { Dialog } from "../Dialog/Dialog.tsx";
import { ThemePreview } from "../ThemePreview/ThemePreview.tsx";
import s from "./ProjectFormDialog.module.css";

interface Props {
  // The project being edited, or undefined to register a new one.
  project?: ProjectInfo;
  // The themes thingd can serve. Empty omits the field rather than failing the
  // form over a cosmetic setting.
  themes: string[];
  // Reload the list after a successful register or edit.
  onSaved: () => void;
  // Surface a failure in the page-level banner.
  onError: (message: string) => void;
  // Dismiss the dialog without saving.
  onClose: () => void;
}

// ProjectFormDialog registers a project or edits a registered one. Both are the
// same three fields — a URL-safe name, the path to an existing thing tree, and
// the theme the board renders in — so they are one form rather than two that
// drift apart. What differs is where the values start and which request they are
// sent as: a new project is a PUT that mounts the name over the directory, while
// an edit is a PATCH carrying only the fields that changed, so a rename that
// leaves the directory alone does not also re-point it.
//
// It is mounted only while open (the caller renders it conditionally), so its
// draft state starts fresh on each opening and autofocus fires every time.
export function ProjectFormDialog({ project, themes, onSaved, onError, onClose }: Props) {
  const [name, setName] = useState(project?.name ?? "");
  const [dir, setDir] = useState(project?.dir ?? "");
  const [theme, setTheme] = useState(project?.theme ?? "");

  // Every theme the list offers is loaded, not just the selected one: each row
  // shows its own color, and the miniature needs the selection. Dropped on
  // unmount so a closed dialog leaves no stylesheets behind.
  useEffect(() => {
    loadThemesForPreview(themes);
    return () => loadThemesForPreview([]);
  }, [themes]);

  const submit = async () => {
    const n = name.trim();
    const d = dir.trim();
    if (!n || !d) return;
    try {
      if (!project) {
        await registerProject(n, d, theme);
      } else {
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
        await editProject(project.name, changes);
      }
      onSaved();
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <Dialog
      open
      onClose={onClose}
      title={project ? "Edit project" : "Add project"}
      // The theme preview is a miniature board; at the narrow default width it
      // scales down past legibility. Without the field there is nothing wide to
      // show, and a form of one-line fields reads better unstretched.
      size={themes.length > 0 ? "wide" : "default"}
    >
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
          {project
            ? <>Renaming changes the project's URL (<code>/{name.trim() || project.name}</code>).</>
            : "Lower-case letters, digits, and dashes. It becomes the project's URL."}
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
      {themes.length > 0 && (
        <div className={s.field}>
          <span className={s.fieldLabel} id="theme-label">Theme</span>
          {/* List beside preview rather than a select above it: choosing a theme
              is comparing them, and a dropdown hides the alternatives behind a
              click each time. Native radios carry the group semantics and
              arrow-key navigation; the input itself is hidden, the row is the
              control. */}
          <div className={s.themeChooser}>
            <div className={s.themeList} role="radiogroup" aria-labelledby="theme-label">
              {["", ...themes].map((t) => (
                <label key={t || "default"} className={s.themeOption} data-selected={theme === t || undefined}>
                  <input
                    type="radio"
                    name="theme"
                    className={s.themeRadio}
                    value={t}
                    checked={theme === t}
                    onChange={() => setTheme(t)}
                  />
                  {/* The dot carries the theme it names, so the list is scannable
                      by color and not only by name. */}
                  <span className={s.mark} data-theme={t || undefined} aria-hidden="true" />
                  {/* Lowercase like the theme names it sits among: it names the
                      same kind of thing, and a lone capital reads as a different
                      kind of entry. */}
                  <span className={s.themeName}>{t || "default"}</span>
                </label>
              ))}
            </div>
            <ThemePreview theme={theme} />
          </div>
        </div>
      )}
      <div className={s.actions}>
        <button type="button" className={`${s.btn} ${s.btnAmber}`} onClick={submit} disabled={!name.trim() || !dir.trim()}>
          {project ? "Save" : "Add"}
        </button>
        <button type="button" className={s.btnLink} onClick={onClose}>
          Cancel
        </button>
      </div>
    </Dialog>
  );
}
