import type { Scheme } from "./api.ts";
import { useNodeScreen } from "./useNodeScreen.ts";
import { nodeHref } from "./route.ts";
import { isPlainClick } from "./util.ts";
import { NodeDetailPanel } from "./components/NodeDetailPanel/NodeDetailPanel.tsx";
import { ProjectHeader } from "./components/ProjectHeader/ProjectHeader.tsx";
import { SchemeMenu } from "./components/SchemeMenu/SchemeMenu.tsx";
import s from "./NodeScreen.module.css";

interface Props {
  // The project and the ref this screen is showing, both from the URL path.
  project: string;
  nodeRef: string;
  // The server-wide color scheme and the setter for it, both owned by Root.
  scheme: Scheme;
  onScheme: (scheme: Scheme) => void;
  // Re-read the server-wide settings. Called on live-reload, since a scheme
  // changed in another tab arrives the same way a tree change does.
  onRefresh: () => void;
  // Switch to another project by name, or to the picker (null).
  onSwitch: (name: string | null) => void;
  // Push a URL and route to it. Leaving this screen changes which screen is
  // shown, so only Root can do it.
  navigate: (href: string) => void;
}

// NodeScreen is one node given the whole window: the same panel the board shows
// beside its tree, with nothing beside it. Everything it holds lives in
// useNodeScreen; what is left here is the layout.
export function NodeScreen({ project, nodeRef, onSwitch, scheme, onScheme, onRefresh, navigate }: Props) {
  const screen = useNodeScreen({ project, nodeRef, onRefresh, navigate });
  const boardHref = nodeHref(project, "");

  return (
    <div className={s.screen}>
      <ProjectHeader
        project={project}
        title={screen.title}
        href={boardHref}
        onNav={(e) => {
          if (!isPlainClick(e)) return;
          e.preventDefault();
          navigate(boardHref);
        }}
        onSwitch={onSwitch}
      />
      {/* Fixed to the viewport's corner, so it is a sibling of the column rather
          than part of the bar's layout. */}
      <SchemeMenu scheme={scheme} onChange={onScheme} />

      {screen.error && <div className={s.error} onClick={screen.dismissError}>{screen.error}</div>}

      <main className={s.column}>
        {screen.node ? (
          <NodeDetailPanel
            key={screen.node.ref}
            api={screen.api}
            node={screen.node}
            allNodes={screen.tree}
            run={screen.run}
            onSelect={screen.onSelect}
            hrefFor={screen.hrefFor}
            onNav={screen.onNav}
          />
        ) : (
          // Nothing at all until the tree has arrived: `node` is null while it is
          // still in flight, and flashing "no such node" at every reader of a
          // perfectly good link would be a lie told on every page load.
          screen.treeReady && (
            <p className={s.missing}>
              No node at <code>{nodeRef}</code>. <a href={boardHref}>Back to the board</a>.
            </p>
          )
        )}
      </main>
    </div>
  );
}
