import { Priority, Status } from "../domain/generated.ts";
import { emptyFilters, filtersActive, type Filters } from "../filter.ts";

interface Props {
  filters: Filters;
  categories: string[];
  tags: string[];
  statusCounts: Record<string, number>;
  priorityCounts: Record<string, number>;
  onChange: (f: Filters) => void;
}

const STATUSES = [Status.Todo, Status.Doing, Status.Done, Status.Paused];
const PRIORITIES = [Priority.High, Priority.Medium, Priority.Low];

// FilterBar is the left sidebar: a text search plus status and priority facet
// toggles (each with a node count) and category/tag pulldowns. Facets read their
// accent from the global data-status / data-priority maps.
export function FilterBar({ filters, categories, tags, statusCounts, priorityCounts, onChange }: Props) {
  // toggleFacet flips one value in a multi-select facet set (statuses/priorities).
  const toggleFacet = (key: "statuses" | "priorities", v: string) => {
    const next = new Set(filters[key]);
    if (next.has(v)) next.delete(v);
    else next.add(v);
    onChange({ ...filters, [key]: next });
  };

  return (
    <aside className="filter-pane">
      <div className="filter-group">
        <span className="filter-heading">Search</span>
        <input
          className="input"
          type="search"
          placeholder="title, tag, ref…"
          value={filters.query}
          onChange={(e) => onChange({ ...filters, query: e.target.value })}
        />
      </div>

      <div className="filter-group">
        <span className="filter-heading">Status</span>
        {STATUSES.map((s) => (
          <button
            key={s}
            type="button"
            className="facet"
            aria-pressed={filters.statuses.has(s)}
            onClick={() => toggleFacet("statuses", s)}
          >
            <span className="check" />
            <span className="status-badge" data-status={s}>
              {s}
            </span>
            <span className="facet-count">{statusCounts[s] ?? 0}</span>
          </button>
        ))}
      </div>

      <div className="filter-group">
        <span className="filter-heading">Priority</span>
        {PRIORITIES.map((p) => (
          <button
            key={p}
            type="button"
            className="facet"
            aria-pressed={filters.priorities.has(p)}
            onClick={() => toggleFacet("priorities", p)}
          >
            <span className="check" />
            <span className="prio-badge" data-priority={p}>{p}</span>
            <span className="facet-count">{priorityCounts[p] ?? 0}</span>
          </button>
        ))}
      </div>

      <div className="filter-group">
        <span className="filter-heading">Category</span>
        <select
          className="filter-select"
          value={filters.category}
          onChange={(e) => onChange({ ...filters, category: e.target.value })}
        >
          <option value="">all categories</option>
          {categories.map((c) => <option key={c} value={c}>{c}</option>)}
        </select>
      </div>

      <div className="filter-group">
        <span className="filter-heading">Tag</span>
        <select
          className="filter-select"
          value={filters.tag}
          onChange={(e) => onChange({ ...filters, tag: e.target.value })}
        >
          <option value="">all tags</option>
          {tags.map((t) => <option key={t} value={t}>#{t}</option>)}
        </select>
      </div>

      {filtersActive(filters) && (
        <button type="button" className="btn-link" onClick={() => onChange(emptyFilters)}>
          clear filters
        </button>
      )}
    </aside>
  );
}
