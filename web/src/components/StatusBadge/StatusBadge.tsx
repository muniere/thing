import type { Status } from "../../domain/generated.ts";
import s from "./StatusBadge.module.css";

type Variant = "full" | "dot" | "fill";

// The status signal: a colored dot plus a mono label. The dot color comes from
// the global [data-status] → --c token map, so the badge only sets data-status.
// Variants tune it per context: "full" (default) shows dot + label in the filter
// facet's siblings; "dot" hides the label for tree rows; "fill" grows to fill the
// filter facet row.
export function StatusBadge({ status, variant = "full" }: { status: Status; variant?: Variant }) {
  const variantClass = variant === "dot" ? s.dot : variant === "fill" ? s.fill : "";
  return (
    <span className={`${s.status} ${variantClass}`} data-status={status}>
      {status}
    </span>
  );
}
