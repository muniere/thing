import { Status } from "../domain/generated.ts";
import { emptyFilters, filtersActive, type Filters } from "../filter.ts";

interface Props {
  filters: Filters;
  categories: string[];
  tags: string[];
  onChange: (f: Filters) => void;
}

const STATUSES = [Status.Todo, Status.Doing, Status.Done, Status.Paused];

// FilterBar is the left sidebar: status facet toggles plus category and tag
// pulldowns. Status facets read their dot color from the global data-status map.
export function FilterBar({ filters, categories, tags, onChange }: Props) {
  const toggleStatus = (s: string) => {
    const next = new Set(filters.statuses);
    if (next.has(s)) next.delete(s);
    else next.add(s);
    onChange({ ...filters, statuses: next });
  };

  return (
    <aside className="filter-pane">
      <div className="filter-group">
        <span className="filter-heading">Status</span>
        {STATUSES.map((s) => (
          <button
            key={s}
            type="button"
            className="facet"
            aria-pressed={filters.statuses.has(s)}
            onClick={() => toggleStatus(s)}
          >
            <span className="check" />
            <span className="status-badge" data-status={s}>
              {s}
            </span>
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
