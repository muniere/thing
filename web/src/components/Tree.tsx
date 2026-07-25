import type { Node } from "../domain/generated.ts";
import { Type } from "../domain/generated.ts";
import { StatusBadge } from "./StatusBadge.tsx";
import { PriorityBadge } from "./PriorityBadge.tsx";
import s from "./Tree.module.css";

interface Props {
  nodes: Node[];
  activeRef: string | null;
  hrefFor: (ref: string) => string;
  expanded: (ref: string) => boolean;
  onToggle: (ref: string) => void;
}

const UNCATEGORIZED = " uncategorized";

// Tree renders the pruned outline as one status-railed card per top-level node,
// grouped under category headings; nodes without an epic category (category-less
// epics and orphan issues) fall under an unlabeled group. Rows color themselves
// from data-status via the global token map.
export function Tree({ nodes, activeRef, hrefFor, expanded, onToggle }: Props) {
  if (nodes.length === 0) {
    return <p className={s.empty}>No nodes match.</p>;
  }

  // Group top-level nodes by category, preserving first-seen order.
  const order: string[] = [];
  const groups = new Map<string, Node[]>();
  for (const n of nodes) {
    const key = n.type === Type.Epic && n.category ? n.category : UNCATEGORIZED;
    if (!groups.has(key)) {
      groups.set(key, []);
      order.push(key);
    }
    groups.get(key)!.push(n);
  }
  // Real categories keep first-seen order; the uncategorized catch-all always
  // sorts last. Its heading shows only when there is at least one real category
  // (otherwise the tree is a flat, unlabeled list).
  const categories = order.filter((k) => k !== UNCATEGORIZED);
  const ordered = groups.has(UNCATEGORIZED) ? [...categories, UNCATEGORIZED] : categories;
  const grouped = categories.length > 0;

  return (
    <div className={s.tree}>
      {ordered.map((key) => (
        <div key={key}>
          {(key !== UNCATEGORIZED || grouped) && (
            <div className={s.groupLabel}>{key === UNCATEGORIZED ? "uncategorized" : key}</div>
          )}
          {groups.get(key)!.map((n) => (
            <div className={s.epicCard} data-status={n.effectiveStatus} key={n.ref}>
              <Row node={n} activeRef={activeRef} hrefFor={hrefFor} expanded={expanded} onToggle={onToggle} />
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

function Row(
  { node, activeRef, hrefFor, expanded, onToggle }: {
    node: Node;
    activeRef: string | null;
    hrefFor: (ref: string) => string;
    expanded: (ref: string) => boolean;
    onToggle: (ref: string) => void;
  },
) {
  const children = node.children ?? [];
  const hasChildren = children.length > 0;
  const open = hasChildren && expanded(node.ref);
  return (
    <div>
      <div className={s.rowWrap}>
        {hasChildren ? (
          <button
            type="button"
            className={s.caret}
            data-open={open}
            aria-expanded={open}
            aria-label="toggle children"
            onClick={(e) => {
              e.stopPropagation();
              onToggle(node.ref);
            }}
          />
        ) : (
          <span className={s.caretSpacer} />
        )}
        <a
          className={s.row}
          href={hrefFor(node.ref)}
          data-kind={node.type}
          data-status={node.effectiveStatus}
          data-selected={activeRef === node.ref ? "" : undefined}
        >
          <StatusBadge status={node.effectiveStatus} variant="dot" />
          <span className={s.title}>{node.title}</span>
          {hasChildren && <span className={s.count}>{children.length}</span>}
          {node.priority && <PriorityBadge priority={node.priority} />}
          {(node.tags ?? []).map((t) => <span key={t} className={s.tag}>#{t}</span>)}
        </a>
      </div>
      {open && (
        <ul>
          {children.map((c) => (
            <li key={c.ref}>
              <Row node={c} activeRef={activeRef} hrefFor={hrefFor} expanded={expanded} onToggle={onToggle} />
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
