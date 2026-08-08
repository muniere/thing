import { type MouseEvent, useEffect, useRef, useState } from "react";
import type { Node } from "../../domain/generated.ts";
import { Priority, Status, Type } from "../../domain/generated.ts";
import type { Api } from "../../api.ts";
import { renderMarkdown } from "../../markdown.ts";
import { flatten } from "../../util.ts";
import { NodeFormDialog } from "../NodeFormDialog/NodeFormDialog.tsx";
import { PriorityBadge } from "../PriorityBadge/PriorityBadge.tsx";
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
// the node changes.
export function NodeDetailPanel({ api, node, allNodes, run, onSelect, hrefFor, onNav }: Props) {
  const isEpic = node.type === Type.Epic;
  // A parent (epic/issue) can roll its status up from its children, offered as an
  // "auto" choice in the status pulldown; a task's status is always its own.
  const isParent = node.type !== Type.Task;

  const [title, setTitle] = useState(node.title);
  const [category, setCategory] = useState(node.category ?? "");
  const [body, setBody] = useState(node.body ?? "");
  const [preview, setPreview] = useState(true);
  const [linkURL, setLinkURL] = useState("");
  const [linkLabel, setLinkLabel] = useState("");
  const [addingLink, setAddingLink] = useState(false);
  const [moveTo, setMoveTo] = useState("");
  const [moving, setMoving] = useState(false);
  const [editing, setEditing] = useState(false);
  const [copied, setCopied] = useState(false);

  // Copy the ref to the clipboard and flash a check on the button. The async
  // Clipboard API is missing on insecure origins (thingd served over plain HTTP
  // to a non-localhost host), so fall back to a hidden textarea + execCommand.
  const copiedTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => () => clearTimeout(copiedTimer.current), []);

  const copyRef = async () => {
    try {
      if (navigator.clipboard) {
        await navigator.clipboard.writeText(node.ref);
      } else {
        const ta = document.createElement("textarea");
        ta.value = node.ref;
        ta.style.position = "fixed";
        ta.style.opacity = "0";
        document.body.appendChild(ta);
        ta.select();
        const ok = document.execCommand("copy");
        document.body.removeChild(ta);
        if (!ok) throw new Error("copy command failed");
      }
    } catch {
      alert(`Could not copy the ref. It is: ${node.ref}`);
      return;
    }
    setCopied(true);
    clearTimeout(copiedTimer.current);
    copiedTimer.current = setTimeout(() => setCopied(false), 1200);
  };

  // The change-parent picker is a modal dialog; drive the native <dialog> from the
  // moving flag so Escape and the backdrop close it (its onClose resets state).
  const moveDialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = moveDialog.current;
    if (!d) return;
    if (moving && !d.open) d.showModal();
    else if (!moving && d.open) d.close();
  }, [moving]);

  const saveTitle = async () => {
    const t = title.trim();
    if (!t) return;
    const res = await run(api.rename(node.ref, t, isEpic ? category : undefined));
    if (res) {
      setEditing(false);
      onSelect(res.ref);
    }
  };

  const cancelEdit = () => {
    setTitle(node.title);
    setCategory(node.category ?? "");
    setEditing(false);
  };

  // Valid move targets: an issue moves under an epic or to orphan; a task moves
  // under another issue. Epics stay at the root.
  const moveTargets = (): { value: string; label: string }[] => {
    const flat = flatten(allNodes);
    if (node.type === Type.Issue) {
      return [
        { value: "_orphan", label: "(orphan)" },
        ...flat.filter((n) => n.type === Type.Epic).map((n) => ({ value: n.ref, label: n.title })),
      ];
    }
    if (node.type === Type.Task) {
      return flat.filter((n) => n.type === Type.Issue).map((n) => ({ value: n.ref, label: n.title }));
    }
    return [];
  };

  const priority = node.priority ?? "";
  const children = node.children ?? [];

  return (
    <div className={s.detail}>
      {editing ? (
        <div className={s.edit}>
          <input
            className={s.input}
            autoFocus
            placeholder="title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") saveTitle();
              else if (e.key === "Escape") cancelEdit();
            }}
          />
          {isEpic && (
            <input
              className={s.input}
              placeholder="category"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
            />
          )}
          <div className={s.editActions}>
            <button type="button" className={`${s.btn} ${s.btnAmber}`} onClick={saveTitle}>Save</button>
            <button type="button" className={s.btnLink} onClick={cancelEdit}>cancel</button>
          </div>
        </div>
      ) : (
        <div className={s.titleRow}>
          <h2 className={s.title}>{node.title}</h2>
          <button type="button" className={s.btnLink} onClick={() => setEditing(true)}>edit</button>
        </div>
      )}
      <div className={s.refRow}>
        <span className={s.ref}>{node.ref}</span>
        <button
          type="button"
          className={s.copyRef}
          data-copied={copied}
          aria-label={`Copy ref "${node.ref}"`}
          title={copied ? "Copied" : "Copy ref"}
          onClick={copyRef}
        >
          {copied ? (
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
            onChange={(e) => run(api.status(node.ref, e.target.value))}
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
            onChange={(e) => run(api.priority(node.ref, e.target.value))}
          >
            <option value="" disabled>priority</option>
            {PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        </span>
        {!editing && isEpic && node.category && <span className={s.cat}>{node.category}</span>}
        {(node.tags ?? []).map((t) => <span key={t} className={s.tag}>#{t}</span>)}
        {node.updated && <span className={s.updated}>updated {node.updated}</span>}
      </div>

      <div className={s.sectionHead}>
        <span className={s.label} style={{ margin: 0 }}>body</span>
        <button type="button" className={s.btnLink} onClick={() => setPreview((p) => !p)}>
          {preview ? "edit" : "preview"}
        </button>
      </div>
      {preview ? (
        <div className={`${s.bodyPanel} markdown`} dangerouslySetInnerHTML={{ __html: renderMarkdown(node.body ?? "") }} />
      ) : (
        <div className={s.field}>
          <textarea className={s.input} value={body} onChange={(e) => setBody(e.target.value)} rows={10} />
          <div>
            <button type="button" className={`${s.btn} ${s.btnAmber}`} onClick={() => run(api.body(node.ref, body))}>Save body</button>
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
              block
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
                  onClick={() => {
                    if (!confirm(`Remove link "${l.label || l.url}"?`)) return;
                    run(api.removeLink(node.ref, l.url));
                  }}
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
      {addingLink ? (
        <div className={s.inlineForm}>
          <input className={s.input} autoFocus placeholder="https://…" value={linkURL} onChange={(e) => setLinkURL(e.target.value)} />
          <input className={s.input} placeholder="label (optional)" value={linkLabel} onChange={(e) => setLinkLabel(e.target.value)} />
          <button
            type="button"
            className={s.btn}
            onClick={async () => {
              if (!linkURL.trim()) return;
              await run(api.addLink(node.ref, linkURL.trim(), linkLabel.trim()));
              setLinkURL("");
              setLinkLabel("");
              setAddingLink(false);
            }}
          >
            Add link
          </button>
          <button type="button" className={s.btnLink} onClick={() => { setAddingLink(false); setLinkURL(""); setLinkLabel(""); }}>
            cancel
          </button>
        </div>
      ) : (
        <button type="button" className={s.btnLink} onClick={() => setAddingLink(true)}>
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
            <button
              type="button"
              className={s.btn}
              onClick={() => {
                if (confirm(`Reset this ${node.type}'s status to auto (roll up from its children)?`)) {
                  run(api.status(node.ref, ""));
                }
              }}
            >
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
            <button type="button" className={s.btn} onClick={() => setMoving(true)}>Change parent</button>
          </div>
        )}

        <div className={s.action}>
          <div className={s.actionText}>
            <div className={s.actionTitle}>Archive {node.type}</div>
            <div className={s.actionDesc}>
              Shelve this {node.type}{node.type !== Type.Task ? " and its subtree" : ""} out of the tree. Restore it later from the Archived list.
            </div>
          </div>
          <button
            type="button"
            className={s.btn}
            onClick={async () => {
              if (!confirm(`Archive ${node.type} "${node.title}"${node.type !== Type.Task ? " and its subtree" : ""}?`)) return;
              await run(api.archive(node.ref));
              onSelect("");
            }}
          >
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
          <button
            type="button"
            className={`${s.btn} ${s.btnDanger}`}
            onClick={async () => {
              if (!confirm(`Delete ${node.type} "${node.title}"${node.type !== Type.Task ? " and its subtree" : ""}?`)) return;
              await run(api.remove(node.ref));
              onSelect("");
            }}
          >
            Delete
          </button>
        </div>
      </div>

      <dialog
        ref={moveDialog}
        className={s.dialog}
        onClose={() => {
          setMoving(false);
          setMoveTo("");
        }}
      >
        <div className={s.dialogBody}>
          <div className={s.dialogTitle}>Change parent</div>
          <div className={s.dialogDesc}>Move “{node.title}” under a different parent.</div>
          <select className={s.select} value={moveTo} onChange={(e) => setMoveTo(e.target.value)}>
            <option value="">choose new parent…</option>
            {moveTargets().map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
          </select>
          <div className={s.dialogActions}>
            <button
              type="button"
              className={`${s.btn} ${s.btnAmber}`}
              disabled={!moveTo}
              onClick={async () => {
                // A move re-slugs the node (its ref carries the parent path), so
                // follow the returned new ref — otherwise the selection points at
                // the old ref and the pane goes blank after the reload.
                const res = await run(api.move(node.ref, moveTo));
                if (res) onSelect(res.ref);
              }}
            >
              Move
            </button>
            <button type="button" className={s.btnLink} onClick={() => moveDialog.current?.close()}>
              Cancel
            </button>
          </div>
        </div>
      </dialog>
    </div>
  );
}
