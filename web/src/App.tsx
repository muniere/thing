import type { Scheme } from "./api.ts";
import { useApp } from "./useApp.ts";
import { NodeChainList } from "./components/NodeChainList/NodeChainList.tsx";
import { FilterForm } from "./components/FilterForm/FilterForm.tsx";
import { NodeDetailPanel } from "./components/NodeDetailPanel/NodeDetailPanel.tsx";
import { NodeFormDialog } from "./components/NodeFormDialog/NodeFormDialog.tsx";
import { ProjectSwitcher } from "./components/ProjectSwitcher/ProjectSwitcher.tsx";
import { SchemeMenu } from "./components/SchemeMenu/SchemeMenu.tsx";
import { ArchiveList } from "./components/ArchiveList/ArchiveList.tsx";
import s from "./App.module.css";

interface Props {
  // The project this view is scoped to (the first URL path segment).
  project: string;
  // The server-wide color scheme and the setter for it, both owned by Root.
  scheme: Scheme;
  onScheme: (scheme: Scheme) => void;
  // Re-read the server-wide settings. Called on live-reload, since a scheme
  // changed in another tab arrives the same way a tree change does.
  onRefresh: () => void;
  // Switch to another project by name, or to the picker (null). Wired to the
  // logo's switcher caret; Root remounts this component on the new project.
  onSwitch: (name: string | null) => void;
}

// App is one project's board: the filter controls, the tree, and the detail panel
// for whatever is selected. Everything it holds lives in useApp; what is left
// here is the layout.
export function App({ project, onSwitch, scheme, onScheme, onRefresh }: Props) {
  const board = useApp({ project, onRefresh });

  return (
    <div className={s.app}>
      <header className={s.topbar}>
        <div className={s.brandGroup}>
          <a className={s.brand} href={board.rootHref} onClick={(e) => board.onNav(e, "")}>
            <span className={s.dot} />{board.title}
          </a>
          <ProjectSwitcher current={project} onSwitch={onSwitch} />
        </div>
        <div className={s.topbarAdd}>
          <NodeFormDialog api={board.api} parent="" noun="Epic" amber run={board.run} onCreated={board.activate} />
        </div>
      </header>
      {/* Fixed to the viewport's corner, so it is a sibling of the panes rather
          than part of the bar's layout. */}
      <SchemeMenu scheme={scheme} onChange={onScheme} />

      {board.error && <div className={s.error} onClick={board.dismissError}>{board.error}</div>}

      <div className={s.split}>
        <FilterForm
          filters={board.filters}
          defaults={board.defaults}
          categories={board.categories}
          tags={board.tags}
          statusCounts={board.statusCounts}
          priorityCounts={board.priorityCounts}
          onChange={board.setFilters}
        />

        <section className={s.treePane}>
          {board.dir && <div className={s.dir}>{board.dir}</div>}
          <NodeChainList
            nodes={board.filtered}
            activeRef={board.activeRef}
            hrefFor={board.hrefFor}
            onNav={board.onNav}
            expanded={board.fold.expanded}
            onToggle={board.fold.toggle}
          />
          <ArchiveList api={board.api} entries={board.archived} run={board.run} />
        </section>

        <section className={s.detailPane}>
          {board.activeNode ? (
            <NodeDetailPanel
              key={board.activeNode.ref}
              api={board.api}
              node={board.activeNode}
              allNodes={board.tree}
              run={board.run}
              onSelect={board.activate}
              hrefFor={board.hrefFor}
              onNav={board.onNav}
            />
          ) : (
            <p className={s.empty}>Select a node to view and edit it.</p>
          )}
        </section>
      </div>
    </div>
  );
}
