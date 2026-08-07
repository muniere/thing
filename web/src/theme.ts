// Applying the color theme a project selects in its config.yaml.
//
// Nothing here knows which themes exist. A theme is a stylesheet thingd serves
// at /themes/<name>.css — built into the binary, or dropped in a themes
// directory on disk — so applying one is: set html[data-theme] and link that
// stylesheet. Adding a theme is adding a file, with no matching change here.
//
// A name that resolves to nothing 404s, no rule matches the attribute, and the
// board stays on the default palette defined in styles/tokens.css. That is also
// what a typo in config.yaml comes to, so it warns rather than failing.

// The id of the <link> this module owns, so switching projects replaces that one
// element instead of stacking a stylesheet per theme ever applied.
const LINK_ID = "theme-stylesheet";

// Names are a URL path segment on the way to a file name. thingd refuses an
// unsafe one too, but the check is cheap and keeps a junk name from reaching the
// DOM at all.
const SAFE_NAME = /^[a-z0-9][a-z0-9-]*$/;

function linkElement(): HTMLLinkElement {
  const existing = document.getElementById(LINK_ID);
  if (existing instanceof HTMLLinkElement) {
    return existing;
  }
  const link = document.createElement("link");
  link.id = LINK_ID;
  link.rel = "stylesheet";
  document.head.append(link);
  return link;
}

// applyTheme points the document at a palette, or back at the default when the
// name is empty (no theme configured) or unusable.
export function applyTheme(name: string | undefined): void {
  if (name && !SAFE_NAME.test(name)) {
    console.warn(`ignoring theme ${JSON.stringify(name)} from config.yaml: a theme name is lowercase letters, digits, and dashes`);
    name = undefined;
  }
  const root = document.documentElement;
  if (!name) {
    delete root.dataset.theme;
    document.getElementById(LINK_ID)?.remove();
    return;
  }
  root.dataset.theme = name;
  const link = linkElement();
  const href = `/themes/${name}.css`;
  if (link.getAttribute("href") === href) {
    return;
  }
  link.onerror = () => {
    console.warn(`theme ${JSON.stringify(name)} was not found at ${href}; using the default palette. Define it by adding ${name}.css to themes/ beside projects.yaml.`);
  };
  link.setAttribute("href", href);
}
