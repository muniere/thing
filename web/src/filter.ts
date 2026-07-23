import type { Node } from "./domain/generated.ts";

export interface Filters {
  statuses: Set<string>; // empty = all statuses
  category: string; // "" = all categories (applies to epics)
  tag: string; // "" = all tags
}

export const emptyFilters: Filters = { statuses: new Set(), category: "", tag: "" };

export function filtersActive(f: Filters): boolean {
  return f.statuses.size > 0 || f.category !== "" || f.tag !== "";
}

// filtersToQuery / filtersFromQuery round-trip the filters through the URL query
// string so a filtered view survives a reload and is shareable. Empty facets are
// omitted, so no filter yields "" (a bare path).
export function filtersToQuery(f: Filters): string {
  const p = new URLSearchParams();
  if (f.statuses.size > 0) p.set("status", [...f.statuses].join(","));
  if (f.category !== "") p.set("category", f.category);
  if (f.tag !== "") p.set("tag", f.tag);
  const s = p.toString();
  return s ? `?${s}` : "";
}

export function filtersFromQuery(search: string): Filters {
  const p = new URLSearchParams(search);
  const status = p.get("status");
  return {
    statuses: new Set(status ? status.split(",").filter(Boolean) : []),
    category: p.get("category") ?? "",
    tag: p.get("tag") ?? "",
  };
}

// selfMatches tests a node's own status and tags against the filters. Category
// is handled as an epic-level prune in filterTree, not here.
function selfMatches(n: Node, f: Filters): boolean {
  if (f.statuses.size > 0 && !f.statuses.has(n.status)) return false;
  if (f.tag !== "" && !(n.tags ?? []).includes(f.tag)) return false;
  return true;
}

// filterTree returns a pruned copy. A category filter drops whole non-matching
// epic subtrees. Status/tag act as match filters but keep ancestors of any
// match, so the path to a matching node stays visible.
export function filterTree(nodes: Node[], f: Filters): Node[] {
  const out: Node[] = [];
  for (const n of nodes) {
    if (f.category !== "" && n.type === "epic" && (n.category ?? "") !== f.category) {
      continue;
    }
    const children = filterTree(n.children ?? [], f);
    if (selfMatches(n, f) || children.length > 0) {
      out.push({ ...n, children });
    }
  }
  return out;
}

// collectCategories and collectTags gather the distinct values present in the
// tree, for populating the filter dropdowns.
export function collectCategories(nodes: Node[]): string[] {
  const set = new Set<string>();
  for (const n of nodes) {
    if (n.type === "epic" && n.category) set.add(n.category);
  }
  return [...set].sort();
}

export function collectTags(nodes: Node[]): string[] {
  const set = new Set<string>();
  const walk = (ns: Node[]) => {
    for (const n of ns) {
      for (const t of n.tags ?? []) set.add(t);
      walk(n.children ?? []);
    }
  };
  walk(nodes);
  return [...set].sort();
}
