import { type MouseEvent, useCallback } from "react";
import type { Node } from "../../../domain/generated.ts";
import { findNode, isPlainClick } from "../../../lib/util.ts";
import { nodeHref } from "../../../route.ts";
import { type ProjectState, useProject } from "../../../lib/useProject.ts";

interface Input {
  // The project the node belongs to (the first URL path segment).
  project: string;
  // The ref this screen is showing (everything after the project).
  nodeRef: string;
  // Re-read the server-wide settings. Called on live-reload, since a scheme
  // changed in another tab arrives the same way a tree change does.
  onRefresh: () => void;
  // Push a URL and route to it. Every link here leaves for another screen, so
  // unlike the board this view cannot resolve a click on its own.
  navigate: (href: string) => void;
}

export interface NodeScreenState extends ProjectState {
  // The node the URL names, or null — which means "no such node" once treeReady
  // says the tree has arrived, and "not yet" before that.
  node: Node | null;
  // What NodeDetailPanel needs to render its links and follow a node it just
  // created or moved.
  hrefFor: (ref: string) => string;
  onNav: (e: MouseEvent, ref: string) => void;
  onSelect: (ref: string) => void;
}

// useNodeScreen holds the standalone screen's state: the project's data and the
// one node the URL names. There is no filtering, folding, or keyboard nav here —
// the screen is one node, so moving to another one means leaving it.
export function useNodeScreen({ project, nodeRef, onRefresh, navigate }: Input): NodeScreenState {
  const loaded = useProject({ project, onRefresh });
  const node = findNode(loaded.tree, nodeRef);

  const hrefFor = useCallback((ref: string) => nodeHref(project, ref), [project]);

  // Every link on this screen leads to another node's screen, so a plain click
  // goes there rather than focusing anything: there is no second pane to focus.
  // A modified or middle click is still left to the browser.
  const onNav = useCallback(
    (e: MouseEvent, ref: string) => {
      if (!isPlainClick(e)) return;
      e.preventDefault();
      navigate(nodeHref(project, ref));
    },
    [project, navigate],
  );

  // NodeDetailPanel calls this to follow a node it just created or moved.
  const onSelect = useCallback((ref: string) => navigate(nodeHref(project, ref)), [project, navigate]);

  return { ...loaded, node, hrefFor, onNav, onSelect };
}
