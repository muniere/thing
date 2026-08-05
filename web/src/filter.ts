import type { Node } from "./domain/generated.ts";

export interface Filters {
  statuses: Set<string>; // empty = all statuses
  priorities: Set<string>; // empty = all priorities
  category: string; // "" = all categories (applies to epics)
  tag: string; // "" = all tags
  query: string; // "" = no text search (matches title/tags/ref)
}

export const emptyFilters: Filters = {
  statuses: new Set(),
  priorities: new Set(),
  category: "",
  tag: "",
  query: "",
};

export function filtersActive(f: Filters): boolean {
  return f.statuses.size > 0 || f.priorities.size > 0 || f.category !== "" || f.tag !== "" || f.query !== "";
}

// FilterDefaults is the configured starting filter, as served by GET /api/config.
// It mirrors the wire shape: a missing key means that facet is not filtered, so
// no key ever has to carry "" to mean "absent".
export interface FilterDefaults {
  statuses?: string[];
  priorities?: string[];
  category?: string;
  tag?: string;
  query?: string;
}

// defaultsToFilters converts the configured defaults into the app's filter state.
// Missing keys become the same empty values emptyFilters uses.
export function defaultsToFilters(d: FilterDefaults | undefined): Filters {
  return {
    statuses: new Set(d?.statuses ?? []),
    priorities: new Set(d?.priorities ?? []),
    category: d?.category ?? "",
    tag: d?.tag ?? "",
    query: d?.query ?? "",
  };
}

function setsEqual(a: Set<string>, b: Set<string>): boolean {
  return a.size === b.size && [...a].every((v) => b.has(v));
}

// filtersEqual compares two filter states by value, so the UI can tell whether the
// current filters still match the configured defaults.
export function filtersEqual(a: Filters, b: Filters): boolean {
  return (
    setsEqual(a.statuses, b.statuses) &&
    setsEqual(a.priorities, b.priorities) &&
    a.category === b.category &&
    a.tag === b.tag &&
    a.query === b.query
  );
}

// FILTER_KEYS are every query-string key the filter state uses, including the
// sentinel. hasFilterQuery asks whether a URL says anything about filters at all:
// when it does not, the configured defaults apply.
const FILTER_KEYS = ["status", "priority", "category", "tag", "q", "filter"];
const NONE = "none";

export function hasFilterQuery(search: string): boolean {
  const p = new URLSearchParams(search);
  return FILTER_KEYS.some((k) => p.has(k));
}

// filtersToQuery / filtersFromQuery round-trip the filters through the URL query
// string so a filtered view survives a reload and is shareable. Empty facets are
// omitted. A bare query means "apply the configured defaults", so when defaults
// exist, a fully cleared filter is spelled out as ?filter=none instead of an empty
// query — otherwise reloading would silently re-apply them.
export function filtersToQuery(f: Filters, defaults: Filters = emptyFilters): string {
  const p = new URLSearchParams();
  if (f.statuses.size > 0) p.set("status", [...f.statuses].join(","));
  if (f.priorities.size > 0) p.set("priority", [...f.priorities].join(","));
  if (f.category !== "") p.set("category", f.category);
  if (f.tag !== "") p.set("tag", f.tag);
  if (f.query !== "") p.set("q", f.query);
  const s = p.toString();
  if (s) return `?${s}`;
  return filtersActive(defaults) ? `?filter=${NONE}` : "";
}

// filtersFromQuery reads the filters back. A URL that says nothing about filters
// yields the configured defaults; ?filter=none is an explicit "no filters"; anything
// else is taken verbatim, with no defaults mixed into the facets it leaves out.
export function filtersFromQuery(search: string, defaults: Filters = emptyFilters): Filters {
  const p = new URLSearchParams(search);
  if (p.get("filter") === NONE) return emptyFilters;
  if (!hasFilterQuery(search)) return defaults;
  const status = p.get("status");
  const priority = p.get("priority");
  return {
    statuses: new Set(status ? status.split(",").filter(Boolean) : []),
    priorities: new Set(priority ? priority.split(",").filter(Boolean) : []),
    category: p.get("category") ?? "",
    tag: p.get("tag") ?? "",
    query: p.get("q") ?? "",
  };
}

// selfMatches tests a node's own status, tags, and the text query against the
// filters (the query matches title/tags/ref). Category is handled as an
// epic-level prune in filterTree, not here.
function selfMatches(n: Node, f: Filters): boolean {
  if (f.statuses.size > 0 && !f.statuses.has(n.effectiveStatus)) return false;
  if (f.priorities.size > 0 && !f.priorities.has(n.priority ?? "")) return false;
  if (f.tag !== "" && !(n.tags ?? []).includes(f.tag)) return false;
  if (f.query !== "") {
    const hay = `${n.title} ${(n.tags ?? []).join(" ")} ${n.ref}`.toLowerCase();
    if (!hay.includes(f.query.toLowerCase())) return false;
  }
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

// collectStatusCounts tallies how many nodes (at any depth) carry each status,
// for the counts shown on the status filter facets. Parent statuses are
// rolled-up, matching what the status filter tests against.
export function collectStatusCounts(nodes: Node[]): Record<string, number> {
  const counts: Record<string, number> = {};
  const walk = (ns: Node[]) => {
    for (const n of ns) {
      counts[n.effectiveStatus] = (counts[n.effectiveStatus] ?? 0) + 1;
      walk(n.children ?? []);
    }
  };
  walk(nodes);
  return counts;
}

// collectPriorityCounts tallies nodes by priority (only those that have one),
// for the counts on the priority filter facets.
export function collectPriorityCounts(nodes: Node[]): Record<string, number> {
  const counts: Record<string, number> = {};
  const walk = (ns: Node[]) => {
    for (const n of ns) {
      if (n.priority) counts[n.priority] = (counts[n.priority] ?? 0) + 1;
      walk(n.children ?? []);
    }
  };
  walk(nodes);
  return counts;
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
