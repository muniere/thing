import type { Node } from "./domain/generated.ts";
import type { FilterDefaults } from "./filter.ts";

// Thin client over thingd's JSON API. One server hosts multiple projects, so
// every node route is scoped under /api/projects/<project>/. Nodes are addressed
// by their ref (a slug-path like "epic/issue/task"), used verbatim as the tail of
// the path. Per-field edits go through a single PATCH so the multi-segment ref
// stays the whole tail.

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const err = (await res.json()) as { error?: string };
      if (err.error) message = err.error;
    } catch {
      // non-JSON error body; keep the status line
    }
    throw new Error(message);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export interface CreateInput {
  title: string;
  priority?: string;
  category?: string;
  tags?: string[];
}

// ProjectInfo is one registered project as shown on the root picker.
export interface ProjectInfo {
  name: string;
  title: string;
  dir: string;
}

// ArchiveEntry is one shelved subtree, as listed by GET /archives. `ref` is its
// archive address ("_archives/<name>"); `from` is the ref it was archived from.
export interface ArchiveEntry {
  ref: string;
  from: string;
  title: string;
  type: string;
  priority?: string;
  status?: string;
  archivedAt?: string;
}

// ArchiveDetail is one archived entry with its full read-only content, as served
// by GET /archives/<name> — the web equivalent of `show _archives/<name>`.
export interface ArchiveDetail extends ArchiveEntry {
  category?: string;
  tags?: string[];
  links?: { url: string; label?: string }[];
  body?: string;
}

// projects lists the registered projects for the root picker. It is the only
// route not scoped to a single project.
export function listProjects(): Promise<ProjectInfo[]> {
  return req<ProjectInfo[]>("GET", "/api/projects");
}

// ReloadResult summarizes a registry reload (see reloadProjects): which mounts
// changed and any entries that could not be mounted.
export interface ReloadResult {
  added: string[];
  removed: string[];
  repointed: string[];
  skipped: { name: string; reason: string }[];
}

// reloadProjects re-reads the server's projects.yaml and reconciles the mounts to
// it — mounting new entries, unmounting dropped ones, re-pointing changed dirs,
// and matching the order. It is the picker's manual resync; the in-memory registry
// otherwise only changes through the API, so a hand-edit to the file needs this.
export function reloadProjects(): Promise<ReloadResult> {
  return req<ReloadResult>("POST", "/api/projects/reload");
}

// registerProject mounts a project at name over an existing thing tree at dir.
// The name is the resource URI, so this is a PUT; it is idempotent for the same
// name+dir and rejects a name already bound to a different dir.
export function registerProject(name: string, dir: string): Promise<{ name: string; dir: string }> {
  return req<{ name: string; dir: string }>("PUT", `/api/projects/${name}`, { dir });
}

// editProject renames a project and/or re-points its data directory. Omitted
// fields are left unchanged; the server rejects an empty name or dir, a name
// already in use (409), or a directory that is not a thing tree. A rename changes
// the project's URL (/<name>).
export function editProject(name: string, changes: { name?: string; dir?: string }): Promise<void> {
  return req<void>("PATCH", `/api/projects/${name}`, changes);
}

// unregisterProject removes a project from the registry. It unregisters only —
// the data directory is left on disk.
export function unregisterProject(name: string): Promise<void> {
  return req<void>("DELETE", `/api/projects/${name}`);
}

// moveProject reorders a project in the picker relative to an anchor project:
// pass exactly one of before/after (the anchor's name). It is a position-free
// move, so it does not depend on the current numeric order.
export function moveProject(name: string, anchor: { before: string } | { after: string }): Promise<void> {
  return req<void>("PATCH", `/api/projects/${name}`, anchor);
}

// forProject returns a client bound to one project: every route is prefixed with
// /api/projects/<project>/. Slugs are URL-safe ([a-z0-9-]) so a ref needs no
// escaping to be a URL path.
export function forProject(project: string) {
  const base = `/api/projects/${project}`;
  return {
    tree: () => req<Node[]>("GET", `${base}/tree`),
    // config carries the display settings the UI reads: the title from
    // config.yaml, the served data directory path, the filter state the board
    // starts from (absent when nothing is configured), and the color theme
    // (absent when the project selects none, meaning the default palette).
    config: () => req<{ title: string; dir: string; filter?: FilterDefaults; theme?: string }>("GET", `${base}/config`),
    // create adds a child under the parent ref; the parent decides the type. An
    // empty parent ("") creates a top-level epic.
    create: (parent: string, input: CreateInput) =>
      req<{ ref: string }>("POST", `${base}/nodes/${parent}`, input),
    status: (ref: string, status: string) =>
      req<{ ref: string }>("PATCH", `${base}/nodes/${ref}`, { status }),
    priority: (ref: string, priority: string) =>
      req<{ ref: string }>("PATCH", `${base}/nodes/${ref}`, { priority }),
    rename: (ref: string, title: string, category?: string) =>
      req<{ ref: string }>("PATCH", `${base}/nodes/${ref}`, category === undefined ? { title } : { title, category }),
    move: (ref: string, parent: string) =>
      req<{ ref: string }>("PATCH", `${base}/nodes/${ref}`, { move: parent }),
    body: (ref: string, body: string) =>
      req<{ ref: string }>("PATCH", `${base}/nodes/${ref}`, { body }),
    addLink: (ref: string, url: string, label: string) =>
      req<{ ref: string }>("PATCH", `${base}/nodes/${ref}`, { addLink: { url, label } }),
    removeLink: (ref: string, which: string) =>
      req<{ ref: string }>("PATCH", `${base}/nodes/${ref}`, { removeLink: which }),
    remove: (ref: string) => req<void>("DELETE", `${base}/nodes/${ref}`),
    // archive shelves a node (an epic/issue takes its subtree) into _archives/,
    // returning its new archive ref.
    archive: (ref: string) => req<{ ref: string }>("PATCH", `${base}/nodes/${ref}`, { archive: true }),
    // listArchive returns the shelved entries.
    listArchive: () => req<ArchiveEntry[]>("GET", `${base}/archives`),
    // getArchive returns one shelved entry's read-only detail (incl. body).
    getArchive: (name: string) => req<ArchiveDetail>("GET", `${base}/archives/${name}`),
    // unarchive restores _archives/<name> to where it came from, or to `to` when
    // given, returning the restored ref.
    unarchive: (name: string, to?: string) =>
      req<{ ref: string }>("PATCH", `${base}/archives/${name}`, to ? { to } : {}),
    // fileHref is the URL for one of a node's attachment files (node.files),
    // served straight from its directory — link to it, don't fetch it here. It
    // lives outside /api (raw bytes, not JSON), at /files/<project>/<ref>/<name>.
    fileHref: (ref: string, name: string) => `/files/${project}/${ref}/${name}`,
  };
}

// Api is the per-project client shape, threaded through the app.
export type Api = ReturnType<typeof forProject>;
