import type { Node } from "./domain/generated.ts";

// Placeholder shell. The real tree / detail / filter UI is built in a later
// commit; this one only proves the scaffold compiles against the node types.
export function App() {
  const nodes: Node[] = [];
  return (
    <main>
      <h1>thing</h1>
      <p>{nodes.length} nodes</p>
    </main>
  );
}
