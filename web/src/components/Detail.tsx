import { type MouseEvent, useState } from "react";
import type { Node } from "../domain/generated.ts";
import { Priority, Status, Type } from "../domain/generated.ts";
import { api } from "../api.ts";
import { renderMarkdown } from "../markdown.ts";
import { flatten } from "../util.ts";
import { AddForm } from "./AddForm.tsx";
import { PriorityBadge } from "./PriorityBadge.tsx";
import s from "./Detail.module.css";

interface Props {
  node: Node;
  allNodes: Node[];
  run: <T>(p: Promise<T>) => Promise<T | undefined>;
  onSelect: (ref: string) => void;
  hrefFor: (ref: string) => string;
  onNav: (e: MouseEvent, ref: string) => void;
}

const STATUSES = [Status.Todo, Status.Doing, Status.Done, Status.Paused];
const PRIORITIES = [Priority.High, Priority.Medium, Priority.Low];

// Detail is the full-edit pane for one node. It is remounted per selection (via
// a key on the element), so its draft state resets cleanly when the node changes.
export function Detail({ node, allNodes, run, onSelect, hrefFor, onNav }: Props) {
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
  const [moveTo, setMoveTo] = useState("");
  const [editing, setEditing] = useState(false);

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
      <div className={s.ref}>{node.ref}</div>

      <div className={s.meta}>
        <span className={s.chipField} data-status={node.effectiveStatus}>
          {/* The chip is colored by the effective (rolled-up) status; a parent with
              no own status reads "auto", and picking "auto" clears the pin. */}
          <select
            className={`${s.chip} ${s.statusChip}`}
            data-status={node.effectiveStatus}
            aria-label="status"
            value={node.status || (isParent ? "auto" : node.effectiveStatus)}
            onChange={(e) => run(api.status(node.ref, e.target.value === "auto" ? "" : e.target.value))}
          >
            {isParent && <option value="auto">auto</option>}
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
            <AddForm
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

      <div className={s.label}>links</div>
      {(node.links ?? []).length > 0 && (
        <ul className={s.links}>
          {(node.links ?? []).map((l) => (
            <li key={l.url}>
              <a href={l.url} target="_blank" rel="noreferrer">{l.label || l.url}</a>
              <button type="button" className={s.btnLink} onClick={() => run(api.removeLink(node.ref, l.url))}>remove</button>
            </li>
          ))}
        </ul>
      )}
      <div className={s.inlineForm}>
        <input className={s.input} placeholder="https://…" value={linkURL} onChange={(e) => setLinkURL(e.target.value)} />
        <input className={s.input} placeholder="label (optional)" value={linkLabel} onChange={(e) => setLinkLabel(e.target.value)} />
        <button
          type="button"
          className={s.btn}
          onClick={async () => {
            if (!linkURL.trim()) return;
            await run(api.addLink(node.ref, linkURL.trim(), linkLabel.trim()));
            setLinkURL("");
            setLinkLabel("");
          }}
        >
          Add link
        </button>
      </div>

      {!isEpic && (
        <>
          <div className={s.label}>move</div>
          <div className={s.inlineForm}>
            <select className={s.select} value={moveTo} onChange={(e) => setMoveTo(e.target.value)}>
              <option value="">choose new parent…</option>
              {moveTargets().map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
            </select>
            <button
              type="button"
              className={s.btn}
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
          </div>
        </>
      )}

      <div className={s.label}>danger</div>
      <button
        type="button"
        className={`${s.btn} ${s.btnDanger}`}
        onClick={async () => {
          if (!confirm(`Delete ${node.type} "${node.title}"${node.type !== "task" ? " and its subtree" : ""}?`)) return;
          await run(api.remove(node.ref));
          onSelect("");
        }}
      >
        Delete {node.type}
      </button>
    </div>
  );
}
