import type { MouseEvent } from "react";
import type { Node } from "../../domain/generated.ts";
import { Priority, Status, Type } from "../../domain/generated.ts";
import type { Api } from "../../api.ts";
import { renderMarkdown } from "../../markdown.ts";
import { NodeFormDialog } from "../NodeFormDialog/NodeFormDialog.tsx";
import { PriorityBadge } from "../PriorityBadge/PriorityBadge.tsx";
import { useNodeDetailPanel } from "./useNodeDetailPanel.ts";
import s from "./NodeDetailPanel.module.css";

interface Props {
  api: Api;
  node: Node;
  allNodes: Node[];
  run: <T>(p: Promise<T>) => Promise<T | undefined>;
  onSelect: (ref: string) => void;
  hrefFor: (ref: string) => string;
  onNav: (e: MouseEvent, ref: string) => void;
}

const STATUSES = [Status.Todo, Status.Doing, Status.Done, Status.Paused];
const PRIORITIES = [Priority.High, Priority.Medium, Priority.Low];

// NodeDetailPanel is the full-edit surface for one node. It is remounted per
// selection (via a key on the element), so its draft state resets cleanly when
// the node changes. Everything it does lives in useNodeDetailPanel; what is left
// here is the markup.
export function NodeDetailPanel({ api, node, allNodes, run, onSelect, hrefFor, onNav }: Props) {
  const isEpic = node.type === Type.Epic;
  // A parent (epic/issue) can roll its status up from its children, offered as an
  // "auto" choice in the status pulldown; a task's status is always its own.
  const isParent = node.type !== Type.Task;

  const { editor, body, copy, links, move, actions } = useNodeDetailPanel({ api, node, allNodes, run, onSelect });

  const priority = node.priority ?? "";
  const children = node.children ?? [];

  return (
    <div className={s.detail}>
      {editor.editing ? (
        <div className={s.edit}>
          <input
            className={s.input}
            autoFocus
            placeholder="title"
            value={editor.title}
            onChange={(e) => editor.setTitle(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") editor.save();
              else if (e.key === "Escape") editor.cancel();
            }}
          />
          {isEpic && (
            <input
              className={s.input}
              placeholder="category"
              value={editor.category}
              onChange={(e) => editor.setCategory(e.target.value)}
            />
          )}
          <div className={s.editActions}>
            <button type="button" className={`${s.btn} ${s.btnAmber}`} onClick={editor.save}>Save</button>
            <button type="button" className={s.btnLink} onClick={editor.cancel}>cancel</button>
          </div>
        </div>
      ) : (
        <div className={s.titleRow}>
          <h2 className={s.title}>{node.title}</h2>
          <button type="button" className={s.btnLink} onClick={editor.start}>edit</button>
        </div>
      )}
      <div className={s.refRow}>
        <span className={s.ref}>{node.ref}</span>
        <button
          type="button"
          className={s.copyRef}
          data-copied={copy.copied}
          aria-label={`Copy ref "${node.ref}"`}
          title={copy.copied ? "Copied" : "Copy ref"}
          onClick={copy.copyRef}
        >
          {copy.copied ? (
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M20 6 9 17l-5-5" />
            </svg>
          ) : (
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <rect x="9" y="9" width="11" height="11" rx="2" />
              <path d="M5 15V5a2 2 0 0 1 2-2h10" />
            </svg>
          )}
        </button>
      </div>

      <div className={s.meta}>
        <span className={s.chipField} data-status={node.effectiveStatus}>
          {/* Colored by the effective (rolled-up) status. Picking a value pins it;
              a pinned parent is reset to the rollup from the actions section. */}
          <select
            className={`${s.chip} ${s.statusChip}`}
            data-status={node.effectiveStatus}
            aria-label="status"
            value={node.effectiveStatus}
            onChange={(e) => actions.setStatus(e.target.value)}
          >
            {STATUSES.map((st) => <option key={st} value={st}>{st}</option>)}
          </select>
        </span>
        <span className={`${s.chipField} ${priority ? "" : s.noFill}`}>
          <select
            className={`${s.chip} ${s.priorityChip}`}
            data-priority={priority}
            aria-label="priority"
            value={priority}
            onChange={(e) => actions.setPriority(e.target.value)}
          >
            <option value="" disabled>priority</option>
            {PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        </span>
        {!editor.editing && isEpic && node.category && <span className={s.cat}>{node.category}</span>}
        {(node.tags ?? []).map((t) => <span key={t} className={s.tag}>#{t}</span>)}
        {node.updated && <span className={s.updated}>updated {node.updated}</span>}
      </div>

      <div className={s.sectionHead}>
        <span className={s.label} style={{ margin: 0 }}>body</span>
        <button type="button" className={s.btnLink} onClick={body.togglePreview}>
          {body.preview ? "edit" : "preview"}
        </button>
      </div>
      {body.preview ? (
        <div className={`${s.bodyPanel} markdown`} dangerouslySetInnerHTML={{ __html: renderMarkdown(node.body ?? "") }} />
      ) : (
        <div className={s.field}>
          <textarea className={s.input} value={body.draft} onChange={(e) => body.setDraft(e.target.value)} rows={10} />
          <div>
            <button type="button" className={`${s.btn} ${s.btnAmber}`} onClick={body.save}>Save body</button>
          </div>
        </div>
      )}

      {node.type !== Type.Task && (
        <>
          <div className={s.label}>
            {isEpic ? "issues" : "tasks"}{children.length ? ` · ${children.length}` : ""}
          </div>
          {children.length > 0 && (
            <ul className={s.childList}>
              {children.map((c) => (
                <li key={c.ref}>
                  <a className={s.childRow} href={hrefFor(c.ref)} onClick={(e) => onNav(e, c.ref)} data-status={c.effectiveStatus}>
                    <span className={s.childTitle}>{c.title}</span>
                    {c.priority && <PriorityBadge priority={c.priority} />}
                  </a>
                </li>
              ))}
            </ul>
          )}
          <div className={s.childAdd}>
            <NodeFormDialog
              api={api}
              parent={node.ref}
              noun={isEpic ? "issue" : "task"}
              label={`+ Add a new ${isEpic ? "issue" : "task"}`}
              link
              run={run}
              onCreated={onSelect}
            />
          </div>
        </>
      )}

      {(node.files ?? []).length > 0 && (
        <>
          <div className={s.label}>files</div>
          <div className={s.listPanel}>
            <ul className={s.links}>
              {(node.files ?? []).map((f) => (
                <li key={f}>
                  <span className={s.linkItem}>
                    <a className={s.linkRow} href={api.fileHref(node.ref, f)} target="_blank" rel="noreferrer">
                      <span className={s.linkText}>{f}</span>
                      <span className={s.linkGo} aria-hidden="true" />
                    </a>
                  </span>
                </li>
              ))}
            </ul>
          </div>
        </>
      )}

      <div className={s.label}>links</div>
      {(node.links ?? []).length > 0 && (
        <div className={s.listPanel}>
          <ul className={s.links}>
          {(node.links ?? []).map((l) => (
            <li key={l.url}>
              <span className={s.linkItem}>
                <a className={s.linkRow} href={l.url} target="_blank" rel="noreferrer">
                  <span className={s.linkText}>{l.label || l.url}</span>
                  <span className={s.linkGo} aria-hidden="true" />
                </a>
                <button
                  type="button"
                  className={s.removeLink}
                  aria-label={`Remove link "${l.label || l.url}"`}
                  title="Remove link"
                  onClick={() => links.remove(l)}
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                    <path d="M4 7h16M9 7V4h6v3M6 7l1 13a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-13" />
                    <path d="M10 11v6M14 11v6" />
                  </svg>
                </button>
              </span>
            </li>
          ))}
          </ul>
        </div>
      )}
      {links.adding ? (
        <div className={s.inlineForm}>
          <input className={s.input} autoFocus placeholder="https://…" value={links.url} onChange={(e) => links.setURL(e.target.value)} />
          <input className={s.input} placeholder="label (optional)" value={links.label} onChange={(e) => links.setLabel(e.target.value)} />
          <button type="button" className={s.btn} onClick={links.add}>
            Add link
          </button>
          <button type="button" className={s.btnLink} onClick={links.cancel}>
            cancel
          </button>
        </div>
      ) : (
        <button type="button" className={s.btnLink} onClick={links.start}>
          + Add link
        </button>
      )}

      <div className={s.label}>actions</div>
      <div className={s.actions}>
        {isParent && node.status && (
          <div className={s.action}>
            <div className={s.actionText}>
              <div className={s.actionTitle}>Roll up status</div>
              <div className={s.actionDesc}>
                Status is pinned to {node.status}. Reset it to roll up from this {node.type}'s children.
              </div>
            </div>
            <button type="button" className={s.btn} onClick={actions.resetStatus}>
              Reset to auto
            </button>
          </div>
        )}

        {!isEpic && (
          <div className={s.action}>
            <div className={s.actionText}>
              <div className={s.actionTitle}>Change parent</div>
              <div className={s.actionDesc}>Move this {node.type} under a different parent.</div>
            </div>
            <button type="button" className={s.btn} onClick={move.start}>Change parent</button>
          </div>
        )}

        <div className={s.action}>
          <div className={s.actionText}>
            <div className={s.actionTitle}>Archive {node.type}</div>
            <div className={s.actionDesc}>
              Shelve this {node.type}{node.type !== Type.Task ? " and its subtree" : ""} out of the tree. Restore it later from the Archived list.
            </div>
          </div>
          <button type="button" className={s.btn} onClick={actions.archive}>
            Archive
          </button>
        </div>

        <div className={s.action}>
          <div className={s.actionText}>
            <div className={s.actionTitle}>Delete {node.type}</div>
            <div className={s.actionDesc}>
              Permanently remove this {node.type}{node.type !== Type.Task ? " and its subtree" : ""}.
            </div>
          </div>
          <button type="button" className={`${s.btn} ${s.btnDanger}`} onClick={actions.remove}>
            Delete
          </button>
        </div>
      </div>

      {(node.markers ?? []).length > 0 && (
        <div className={s.markers}>
          {(node.markers ?? []).map((m, i) => (
            <div key={i} className={s.marker} data-severity={m.severity}>
              <span className={s.markerIcon} aria-hidden="true">{m.severity === "warn" ? "⚠️" : "ℹ️"}</span>
              <span className={s.markerMessage}>{m.message}</span>
            </div>
          ))}
        </div>
      )}

      <dialog ref={move.dialog} className={s.dialog} onClose={move.reset}>
        <div className={s.dialogBody}>
          <div className={s.dialogTitle}>Change parent</div>
          <div className={s.dialogDesc}>Move “{node.title}” under a different parent.</div>
          <select className={s.select} value={move.target} onChange={(e) => move.setTarget(e.target.value)}>
            <option value="">choose new parent…</option>
            {move.targets.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
          </select>
          <div className={s.dialogActions}>
            <button
              type="button"
              className={`${s.btn} ${s.btnAmber}`}
              disabled={!move.target}
              onClick={move.submit}
            >
              Move
            </button>
            <button type="button" className={s.btnLink} onClick={move.close}>
              Cancel
            </button>
          </div>
        </div>
      </dialog>
    </div>
  );
}
