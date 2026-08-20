// Keeps the tab's icon pointing at the project on screen.
//
// The whole app is one document, so moving between the picker and a board — or
// between two projects — is a pushState rather than a load, and the icon the
// server sent with the shell would otherwise stay for the session. thingd serves
// each project's icon at one URL per project (see internal/server/favicon.go),
// which is why nothing here needs to know whether the tree carries an icon of its
// own or gets the fallback: the URL is the same either way.
//
// The link is replaced rather than re-pointed, because browsers differ on whether
// they refetch an icon whose href changed in place; a link entering the document
// is the change every one of them acts on.

// shellHref is the icon index.html ships with, read out of the document rather
// than repeated here so the mark stays written in one place. It is what the
// picker wears: the picker is the list of boards rather than one of them, so it
// has no project icon to show.
const shellHref = document.querySelector<HTMLLinkElement>('link[rel="icon"]')?.href ?? "";

// syncFavicon points the tab at the given project's icon, or back at the shell's
// own mark for the picker.
export function syncFavicon(project: string | null): void {
  const href = project === null ? shellHref : `/api/projects/${project}/icon`;
  if (href === "") {
    return;
  }
  for (const existing of document.querySelectorAll('link[rel="icon"]')) {
    existing.remove();
  }
  const el = document.createElement("link");
  el.rel = "icon";
  el.href = href;
  document.head.appendChild(el);
}
