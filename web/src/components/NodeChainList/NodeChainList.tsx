import type { MouseEvent } from "react";
import type { Node } from "../../domain/generated.ts";
import { groupTopNodes } from "../../lib/util.ts";
import { StatusBadge } from "../StatusBadge/StatusBadge.tsx";
import { PriorityBadge } from "../PriorityBadge/PriorityBadge.tsx";
import s from "./NodeChainList.module.css";

interface Props {
  nodes: Node[];
  activeRef: string | null;
  hrefFor: (ref: string) => string;
  onNav: (e: MouseEvent, ref: string) => void;
  expanded: (ref: string) => boolean;
  onToggle: (ref: string) => void;
}

// NodeChainList renders the pruned outline as one status-railed card per
// top-level node, grouped under category headings (via groupTopNodes, the same
// order keyboard nav walks). Rows color themselves from data-status via the
// global token map.
export function NodeChainList({ nodes, activeRef, hrefFor, onNav, expanded, onToggle }: Props) {
  if (nodes.length === 0) {
    return <p className={s.empty}>No nodes match.</p>;
  }
  return (
    <div className={s.tree}>
      {groupTopNodes(nodes).map((g) => (
        <div key={g.key}>
          {g.label && <div className={s.groupLabel}>{g.label}</div>}
          {g.nodes.map((n) => (
            <div className={s.epicCard} data-status={n.effectiveStatus} key={n.ref}>
              <Row node={n} activeRef={activeRef} hrefFor={hrefFor} onNav={onNav} expanded={expanded} onToggle={onToggle} />
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

function Row(
  { node, activeRef, hrefFor, onNav, expanded, onToggle }: {
    node: Node;
    activeRef: string | null;
    hrefFor: (ref: string) => string;
    onNav: (e: MouseEvent, ref: string) => void;
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
          onClick={(e) => onNav(e, node.ref)}
          data-kind={node.type}
          data-status={node.effectiveStatus}
          data-selected={activeRef === node.ref ? "" : undefined}
        >
          <StatusBadge status={node.effectiveStatus} variant="dot" />
          <span className={s.title}>{node.title}</span>
          {hasChildren && <span className={s.count}>{children.length}</span>}
          {node.priority && <PriorityBadge priority={node.priority} />}
        </a>
      </div>
      {open && (
        <ul>
          {children.map((c) => (
            <li key={c.ref}>
              <Row node={c} activeRef={activeRef} hrefFor={hrefFor} onNav={onNav} expanded={expanded} onToggle={onToggle} />
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
