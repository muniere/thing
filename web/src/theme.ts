// Applying the color theme a project selects in projects.yaml.
//
// Nothing here knows which themes exist. A theme is a stylesheet thingd serves
// at /themes/<name>.css — built into the binary, or dropped in a themes
// directory on disk — so applying one is: set data-theme and link that
// stylesheet. Adding a theme is adding a file, with no matching change here.
//
// A name that resolves to nothing 404s, no rule matches the attribute, and the
// board stays on the default palette defined in styles/tokens.css. That is also
// what a typo in projects.yaml comes to, so it warns rather than failing.

// The <link> elements this module owns are grouped by purpose, so one group's
// churn never disturbs another's: the board's own theme, the theme an edit dialog
// is previewing, and the set behind the picker's per-project marks. Within a
// group an element is reused by name rather than stacking a stylesheet per theme
// ever touched.
const BOARD = "board";
const PREVIEW = "preview";
const MARKS = "marks";

// Names are a URL path segment on the way to a file name. thingd refuses an
// unsafe one too, but the check is cheap and keeps a junk name from reaching the
// DOM at all.
const SAFE_NAME = /^[a-z0-9][a-z0-9-]*$/;

const linkId = (group: string, name: string) => `theme-${group}-${name}`;

// load reconciles a group's stylesheets to exactly `names`: it adds what is
// missing, drops what is no longer wanted, and leaves the rest untouched. It
// returns the names that survived validation, so a caller can mirror them onto
// whatever elements carry data-theme.
function load(group: string, names: string[], warn: (name: string, href: string) => void): string[] {
  const wanted = new Set<string>();
  for (const name of names) {
    if (SAFE_NAME.test(name)) {
      wanted.add(name);
    } else {
      console.warn(`ignoring theme ${JSON.stringify(name)}: a theme name is lowercase letters, digits, and dashes`);
    }
  }
  for (const link of document.querySelectorAll<HTMLLinkElement>(`link[data-theme-group="${group}"]`)) {
    if (!wanted.has(link.dataset.themeName ?? "")) {
      link.remove();
    }
  }
  for (const name of wanted) {
    if (document.getElementById(linkId(group, name))) {
      continue;
    }
    const href = `/themes/${name}.css`;
    const link = document.createElement("link");
    link.id = linkId(group, name);
    link.rel = "stylesheet";
    link.dataset.themeGroup = group;
    link.dataset.themeName = name;
    link.onerror = () => warn(name, href);
    link.href = href;
    document.head.append(link);
  }
  return [...wanted];
}

// applyTheme points the document at a palette, or back at the default when the
// name is empty (no theme configured) or unusable.
export function applyTheme(name: string | undefined): void {
  const [applied] = load(BOARD, name ? [name] : [], (n, href) => {
    console.warn(`theme ${JSON.stringify(n)} was not found at ${href}; using the default palette. Define it by adding ${n}.css to themes/ beside projects.yaml.`);
  });
  if (applied) {
    document.documentElement.dataset.theme = applied;
  } else {
    delete document.documentElement.dataset.theme;
  }
}

// loadThemesForPreview makes several themes' tokens available to any element
// carrying data-theme="<name>", without touching the document's own theme. The
// theme stylesheets are element-scoped for exactly this: a theme is shown by
// rendering something in it, not by recoloring the page.
//
// It takes the whole set the edit dialog offers rather than only the selected
// one, because the dialog shows every choice as a dot in its own color beside the
// miniature of the selected one. Passing an empty list drops them again.
export function loadThemesForPreview(names: string[]): void {
  load(PREVIEW, names, (n, href) => {
    console.warn(`theme ${JSON.stringify(n)} was not found at ${href}; it shows the default palette instead.`);
  });
}

// loadThemeMarks makes several themes' tokens available at once, for the marks
// that tell projects apart in a list. It loads the themes those projects use
// rather than every theme that exists, so the cost follows the number of
// projects; a name nothing defines is left to fall back to the default palette,
// which is what that project's board does too.
export function loadThemeMarks(names: string[]): void {
  load(MARKS, names, () => {
    // Silent: the board and the edit dialog already warn about a name that
    // resolves to nothing, and a mark that quietly shows the default palette is
    // exactly what that project will look like.
  });
}
