import type { Node } from "./domain/generated.ts";

// Thin client over thingd's JSON API. Nodes are addressed by their ref (a
// slug-path like "epic/issue/task"), used verbatim as the URL path. Per-field
// edits go through a single PATCH so the multi-segment ref stays the whole path.

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

// Slugs are URL-safe ([a-z0-9-]) so a ref needs no escaping to be a URL path.
export const api = {
  tree: () => req<Node[]>("GET", "/api/tree"),
  // create adds a child under the parent ref; the parent decides the type. An
  // empty parent ("") creates a top-level epic.
  create: (parent: string, input: CreateInput) =>
    req<{ ref: string }>("POST", `/api/nodes/${parent}`, input),
  status: (ref: string, status: string) =>
    req<{ ref: string }>("PATCH", `/api/nodes/${ref}`, { status }),
  priority: (ref: string, priority: string) =>
    req<{ ref: string }>("PATCH", `/api/nodes/${ref}`, { priority }),
  rename: (ref: string, title: string, category?: string) =>
    req<{ ref: string }>("PATCH", `/api/nodes/${ref}`, category === undefined ? { title } : { title, category }),
  move: (ref: string, parent: string) =>
    req<{ ref: string }>("PATCH", `/api/nodes/${ref}`, { move: parent }),
  body: (ref: string, body: string) =>
    req<{ ref: string }>("PATCH", `/api/nodes/${ref}`, { body }),
  addLink: (ref: string, url: string, label: string) =>
    req<{ ref: string }>("PATCH", `/api/nodes/${ref}`, { addLink: { url, label } }),
  removeLink: (ref: string, which: string) =>
    req<{ ref: string }>("PATCH", `/api/nodes/${ref}`, { removeLink: which }),
  remove: (ref: string) => req<void>("DELETE", `/api/nodes/${ref}`),
};
