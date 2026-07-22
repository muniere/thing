import type { Node } from "../domain/generated.ts";
import { statusBox } from "../util.ts";

interface Props {
  nodes: Node[];
  selected: string | null;
  onSelect: (ref: string) => void;
}

// Tree renders the pruned outline. Selection is by ref; clicking a row opens it
// in the detail pane.
export function Tree({ nodes, selected, onSelect }: Props) {
  if (nodes.length === 0) {
    return <p className="empty">No nodes match.</p>;
  }
  return <ul className="tree">{nodes.map((n) => <TreeNode key={n.ref} node={n} selected={selected} onSelect={onSelect} />)}</ul>;
}

function TreeNode({ node, selected, onSelect }: { node: Node; selected: string | null; onSelect: (ref: string) => void }) {
  const children = node.children ?? [];
  return (
    <li>
      <button
        type="button"
        className={`row ${node.type} ${selected === node.ref ? "selected" : ""}`}
        onClick={() => onSelect(node.ref)}
      >
        <span className="box">{statusBox(node.status)}</span>
        <span className="title">{node.title}</span>
        {node.priority && <span className={`prio prio-${node.priority}`}>{node.priority}</span>}
        {(node.tags ?? []).map((t) => <span key={t} className="tag">{t}</span>)}
      </button>
      {children.length > 0 && (
        <ul>{children.map((c) => <TreeNode key={c.ref} node={c} selected={selected} onSelect={onSelect} />)}</ul>
      )}
    </li>
  );
}
