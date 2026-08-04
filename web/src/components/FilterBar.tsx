import { Priority, Status } from "../domain/generated.ts";
import { filtersActive, filtersEqual, type Filters } from "../filter.ts";
import { StatusBadge } from "./StatusBadge.tsx";
import { PriorityBadge } from "./PriorityBadge.tsx";
import s from "./FilterBar.module.css";

interface Props {
  filters: Filters;
  // The configured starting point. The clear button returns here, not to a bare
  // filter state, so a board with defaults reopens the way it was configured.
  defaults: Filters;
  categories: string[];
  tags: string[];
  statusCounts: Record<string, number>;
  priorityCounts: Record<string, number>;
  onChange: (f: Filters) => void;
}

const STATUSES = [Status.Todo, Status.Doing, Status.Done, Status.Paused];
const PRIORITIES = [Priority.High, Priority.Medium, Priority.Low];

// FilterBar is the left sidebar: a text search plus status and priority facet
// toggles (each with a node count) and category/tag pulldowns. The status/priority
// badges fill the facet row and read their accent from the badge components.
export function FilterBar({ filters, defaults, categories, tags, statusCounts, priorityCounts, onChange }: Props) {
  // toggleFacet flips one value in a multi-select facet set (statuses/priorities).
  const toggleFacet = (key: "statuses" | "priorities", v: string) => {
    const next = new Set(filters[key]);
    if (next.has(v)) next.delete(v);
    else next.add(v);
    onChange({ ...filters, [key]: next });
  };

  return (
    <aside className={s.pane}>
      <div className={s.group}>
        <span className={s.heading}>Search</span>
        <input
          className={s.input}
          type="search"
          placeholder="title, tag, ref…"
          value={filters.query}
          onChange={(e) => onChange({ ...filters, query: e.target.value })}
        />
      </div>

      <div className={s.group}>
        <span className={s.heading}>Status</span>
        {STATUSES.map((st) => (
          <button
            key={st}
            type="button"
            className={s.facet}
            aria-pressed={filters.statuses.has(st)}
            onClick={() => toggleFacet("statuses", st)}
          >
            <span className={s.check} />
            <StatusBadge status={st} variant="fill" />
            <span className={s.count}>{statusCounts[st] ?? 0}</span>
          </button>
        ))}
      </div>

      <div className={s.group}>
        <span className={s.heading}>Priority</span>
        {PRIORITIES.map((p) => (
          <button
            key={p}
            type="button"
            className={s.facet}
            aria-pressed={filters.priorities.has(p)}
            onClick={() => toggleFacet("priorities", p)}
          >
            <span className={s.check} />
            <PriorityBadge priority={p} variant="fill" />
            <span className={s.count}>{priorityCounts[p] ?? 0}</span>
          </button>
        ))}
      </div>

      <div className={s.group}>
        <span className={s.heading}>Category</span>
        <select
          className={s.select}
          value={filters.category}
          onChange={(e) => onChange({ ...filters, category: e.target.value })}
        >
          <option value="">all categories</option>
          {categories.map((c) => <option key={c} value={c}>{c}</option>)}
        </select>
      </div>

      <div className={s.group}>
        <span className={s.heading}>Tag</span>
        <select
          className={s.select}
          value={filters.tag}
          onChange={(e) => onChange({ ...filters, tag: e.target.value })}
        >
          <option value="">all tags</option>
          {tags.map((t) => <option key={t} value={t}>#{t}</option>)}
        </select>
      </div>

      {!filtersEqual(filters, defaults) && (
        <button type="button" className={s.clear} onClick={() => onChange(defaults)}>
          {filtersActive(defaults) ? "reset filters" : "clear filters"}
        </button>
      )}
    </aside>
  );
}
