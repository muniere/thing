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

// The ids of the <link> elements this module owns. The board's theme and a
// preview are separate so previewing one does not disturb the other, and each is
// a single element whose href is swapped rather than a stylesheet per theme ever
// touched.
const BOARD_LINK_ID = "theme-stylesheet";
const PREVIEW_LINK_ID = "theme-preview-stylesheet";

// Names are a URL path segment on the way to a file name. thingd refuses an
// unsafe one too, but the check is cheap and keeps a junk name from reaching the
// DOM at all.
const SAFE_NAME = /^[a-z0-9][a-z0-9-]*$/;

function linkElement(id: string): HTMLLinkElement {
  const existing = document.getElementById(id);
  if (existing instanceof HTMLLinkElement) {
    return existing;
  }
  const link = document.createElement("link");
  link.id = id;
  link.rel = "stylesheet";
  document.head.append(link);
  return link;
}

// point aims one of the module's links at a theme, or drops it when the name is
// empty or unusable. It returns the name actually used, so a caller can mirror it
// onto whatever element carries data-theme.
function point(id: string, name: string | undefined, warn: (name: string, href: string) => void): string | undefined {
  if (name && !SAFE_NAME.test(name)) {
    console.warn(`ignoring theme ${JSON.stringify(name)}: a theme name is lowercase letters, digits, and dashes`);
    name = undefined;
  }
  if (!name) {
    document.getElementById(id)?.remove();
    return undefined;
  }
  const link = linkElement(id);
  const href = `/themes/${name}.css`;
  if (link.getAttribute("href") !== href) {
    link.onerror = () => warn(name, href);
    link.setAttribute("href", href);
  }
  return name;
}

// applyTheme points the document at a palette, or back at the default when the
// name is empty (no theme configured) or unusable.
export function applyTheme(name: string | undefined): void {
  const applied = point(BOARD_LINK_ID, name, (n, href) => {
    console.warn(`theme ${JSON.stringify(n)} was not found at ${href}; using the default palette. Define it by adding ${n}.css to themes/ beside projects.yaml.`);
  });
  if (applied) {
    document.documentElement.dataset.theme = applied;
  } else {
    delete document.documentElement.dataset.theme;
  }
}

// loadThemeForPreview makes a theme's tokens available to any element carrying
// data-theme="<name>", without touching the document's own theme. The theme
// stylesheets are element-scoped for exactly this: the picker shows what a theme
// looks like on a swatch rather than by recoloring the page.
//
// Passing nothing drops the preview stylesheet again.
export function loadThemeForPreview(name: string | undefined): void {
  point(PREVIEW_LINK_ID, name, (n, href) => {
    console.warn(`theme ${JSON.stringify(n)} was not found at ${href}; its preview shows the default palette instead.`);
  });
}
