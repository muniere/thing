import type { Node } from "./domain/generated.ts";

// findNode locates a node anywhere in the tree by ref.
export function findNode(nodes: Node[], ref: string): Node | null {
  for (const n of nodes) {
    if (n.ref === ref) return n;
    const hit = findNode(n.children ?? [], ref);
    if (hit) return hit;
  }
  return null;
}

// flatten returns every node in depth-first order.
export function flatten(nodes: Node[]): Node[] {
  const out: Node[] = [];
  const walk = (ns: Node[]) => {
    for (const n of ns) {
      out.push(n);
      walk(n.children ?? []);
    }
  };
  walk(nodes);
  return out;
}
