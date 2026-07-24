import { useState } from "react";
import type { Node } from "../domain/generated.ts";
import { Priority, Status, Type } from "../domain/generated.ts";
import { api } from "../api.ts";
import { renderMarkdown } from "../markdown.ts";
import { flatten } from "../util.ts";
import { AddForm } from "./AddForm.tsx";

interface Props {
  node: Node;
  allNodes: Node[];
  run: <T>(p: Promise<T>) => Promise<T | undefined>;
  onSelect: (ref: string) => void;
}

const STATUSES = [Status.Todo, Status.Doing, Status.Done, Status.Paused];
const PRIORITIES = [Priority.High, Priority.Medium, Priority.Low];

// Detail is the full-edit pane for one node. It is remounted per selection (via
// a key on the element), so its draft state resets cleanly when the node changes.
export function Detail({ node, allNodes, run, onSelect }: Props) {
  const isEpic = node.type === Type.Epic;

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
    <div className="detail">
      <div className="crumb">{node.type} · {node.ref}{node.updated ? ` · updated ${node.updated}` : ""}</div>

      {editing ? (
        <div className="detail-edit">
          <input
            className="input"
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
              className="input"
              placeholder="category"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
            />
          )}
          <div className="detail-edit-actions">
            <button type="button" className="btn btn-amber" onClick={saveTitle}>Save</button>
            <button type="button" className="btn-link" onClick={cancelEdit}>cancel</button>
          </div>
        </div>
      ) : (
        <div className="detail-title-row">
          <h2 className="detail-title">{node.title}</h2>
          <button type="button" className="btn-link" onClick={() => setEditing(true)}>edit</button>
        </div>
      )}

      <div className="meta">
        <span className="chip-field" data-status={node.status}>
          <select
            className="chip status-chip"
            data-status={node.status}
            aria-label="status"
            value={node.status}
            onChange={(e) => run(api.status(node.ref, e.target.value))}
          >
            {STATUSES.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
        </span>
        <span className={`chip-field ${priority ? "" : "no-fill"}`}>
          <select
            className="chip priority-chip"
            data-priority={priority}
            aria-label="priority"
            value={priority}
            onChange={(e) => run(api.priority(node.ref, e.target.value))}
          >
            <option value="" disabled>priority</option>
            {PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        </span>
        {!editing && isEpic && node.category && <span className="cat">{node.category}</span>}
        {(node.tags ?? []).map((t) => <span key={t} className="tag">#{t}</span>)}
      </div>

      <div className="section-head">
        <span className="label" style={{ margin: 0 }}>body</span>
        <button type="button" className="btn-link" onClick={() => setPreview((p) => !p)}>
          {preview ? "edit" : "preview"}
        </button>
      </div>
      {preview ? (
        <div className="body-panel markdown" dangerouslySetInnerHTML={{ __html: renderMarkdown(node.body ?? "") }} />
      ) : (
        <div className="field">
          <textarea className="input" value={body} onChange={(e) => setBody(e.target.value)} rows={10} />
          <div>
            <button type="button" className="btn btn-amber" onClick={() => run(api.body(node.ref, body))}>Save body</button>
          </div>
        </div>
      )}

      {node.type !== Type.Task && (
        <>
          <div className="label">
            {isEpic ? "issues" : "tasks"}{children.length ? ` · ${children.length}` : ""}
          </div>
          {children.length > 0 && (
            <ul className="child-list">
              {children.map((c) => (
                <li key={c.ref}>
                  <button type="button" className="child-row" data-status={c.status} onClick={() => onSelect(c.ref)}>
                    <span className="child-title">{c.title}</span>
                    {c.priority && <span className="prio" data-priority={c.priority}>{c.priority}</span>}
                  </button>
                </li>
              ))}
            </ul>
          )}
          <AddForm parent={node.ref} noun={isEpic ? "issue" : "task"} run={run} onCreated={onSelect} />
        </>
      )}

      <div className="label">links</div>
      {(node.links ?? []).length > 0 && (
        <ul className="links">
          {(node.links ?? []).map((l) => (
            <li key={l.url}>
              <a href={l.url} target="_blank" rel="noreferrer">{l.label || l.url}</a>
              <button type="button" className="btn-link" onClick={() => run(api.removeLink(node.ref, l.url))}>remove</button>
            </li>
          ))}
        </ul>
      )}
      <div className="inline-form">
        <input className="input" placeholder="https://…" value={linkURL} onChange={(e) => setLinkURL(e.target.value)} />
        <input className="input" placeholder="label (optional)" value={linkLabel} onChange={(e) => setLinkLabel(e.target.value)} />
        <button
          type="button"
          className="btn"
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
          <div className="label">move</div>
          <div className="inline-form">
            <select className="filter-select" style={{ maxWidth: "16rem" }} value={moveTo} onChange={(e) => setMoveTo(e.target.value)}>
              <option value="">choose new parent…</option>
              {moveTargets().map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
            </select>
            <button type="button" className="btn" disabled={!moveTo} onClick={() => run(api.move(node.ref, moveTo))}>Move</button>
          </div>
        </>
      )}

      <div className="label">danger</div>
      <button
        type="button"
        className="btn btn-danger"
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
