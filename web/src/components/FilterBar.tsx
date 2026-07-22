import { Status } from "../domain/generated.ts";
import type { Filters } from "../filter.ts";

interface Props {
  filters: Filters;
  categories: string[];
  tags: string[];
  onChange: (f: Filters) => void;
}

const STATUSES = [Status.Todo, Status.Doing, Status.Done, Status.Paused];

// FilterBar drives status (multi), category (single), and tag (single) filters.
export function FilterBar({ filters, categories, tags, onChange }: Props) {
  const toggleStatus = (s: string) => {
    const next = new Set(filters.statuses);
    if (next.has(s)) next.delete(s);
    else next.add(s);
    onChange({ ...filters, statuses: next });
  };

  return (
    <div className="filterbar">
      <div className="statuses">
        {STATUSES.map((s) => (
          <label key={s} className={filters.statuses.has(s) ? "on" : ""}>
            <input type="checkbox" checked={filters.statuses.has(s)} onChange={() => toggleStatus(s)} />
            {s}
          </label>
        ))}
      </div>
      <select value={filters.category} onChange={(e) => onChange({ ...filters, category: e.target.value })}>
        <option value="">all categories</option>
        {categories.map((c) => <option key={c} value={c}>{c}</option>)}
      </select>
      <select value={filters.tag} onChange={(e) => onChange({ ...filters, tag: e.target.value })}>
        <option value="">all tags</option>
        {tags.map((t) => <option key={t} value={t}>{t}</option>)}
      </select>
    </div>
  );
}
