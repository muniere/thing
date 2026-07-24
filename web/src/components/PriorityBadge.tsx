import type { Priority } from "../domain/generated.ts";
import s from "./PriorityBadge.module.css";

type Variant = "badge" | "fill";

// A node's priority as quiet mono text, colored by level. "badge" (default) is
// used in tree rows and detail child rows; "fill" grows to fill the filter facet
// row. Colors key off data-priority so the palette is self-contained here.
export function PriorityBadge({ priority, variant = "badge" }: { priority: Priority; variant?: Variant }) {
  return (
    <span className={`${s.priority} ${variant === "fill" ? s.fill : ""}`} data-priority={priority}>
      {priority}
    </span>
  );
}
