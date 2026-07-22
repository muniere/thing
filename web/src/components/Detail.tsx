import { useState } from "react";
import type { Node } from "../domain/generated.ts";
import { Priority, Status, Type } from "../domain/generated.ts";
import { api } from "../api.ts";
import { renderMarkdown } from "../markdown.ts";
import { flatten } from "../util.ts";

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
  const [childTitle, setChildTitle] = useState("");

  const saveTitle = async () => {
    const t = title.trim();
    if (!t) return;
    const res = await run(api.rename(node.ref, t, isEpic ? category : undefined));
    if (res) onSelect(res.ref);
  };

  const addChild = async () => {
    const t = childTitle.trim();
    if (!t) return;
    // The parent (this node) decides the child's type on the server.
    const res = await run(api.create(node.ref, { title: t }));
    if (res) {
      setChildTitle("");
      onSelect(res.ref);
    }
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

  return (
    <div className="detail">
      <div className="detail-head">
        <span className={`badge ${node.type}`}>{node.type}</span>
        <code className="slug">{node.ref}</code>
        {node.updated && <span className="updated">updated {node.updated}</span>}
      </div>

      <label className="field">
        <span>title</span>
        <input value={title} onChange={(e) => setTitle(e.target.value)} />
      </label>
      {isEpic && (
        <label className="field">
          <span>category</span>
          <input value={category} onChange={(e) => setCategory(e.target.value)} />
        </label>
      )}
      <button type="button" onClick={saveTitle}>Save title{isEpic ? " & category" : ""}</button>

      <div className="row2">
        <label className="field">
          <span>status</span>
          <select value={node.status} onChange={(e) => run(api.status(node.ref, e.target.value))}>
            {STATUSES.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
        </label>
        <label className="field">
          <span>priority</span>
          <select value={node.priority ?? ""} onChange={(e) => run(api.priority(node.ref, e.target.value))}>
            <option value="" disabled>—</option>
            {PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        </label>
      </div>

      <section>
        <div className="section-head">
          <h3>body</h3>
          <button type="button" onClick={() => setPreview((p) => !p)}>{preview ? "Edit" : "Preview"}</button>
        </div>
        {preview ? (
          <div className="markdown" dangerouslySetInnerHTML={{ __html: renderMarkdown(node.body ?? "") }} />
        ) : (
          <>
            <textarea value={body} onChange={(e) => setBody(e.target.value)} rows={10} />
            <button type="button" onClick={() => run(api.body(node.ref, body))}>Save body</button>
          </>
        )}
      </section>

      <section>
        <h3>links</h3>
        <ul className="links">
          {(node.links ?? []).map((l) => (
            <li key={l.url}>
              <a href={l.url} target="_blank" rel="noreferrer">{l.label || l.url}</a>
              <button type="button" onClick={() => run(api.removeLink(node.ref, l.url))}>remove</button>
            </li>
          ))}
        </ul>
        <div className="addlink">
          <input placeholder="https://…" value={linkURL} onChange={(e) => setLinkURL(e.target.value)} />
          <input placeholder="label (optional)" value={linkLabel} onChange={(e) => setLinkLabel(e.target.value)} />
          <button
            type="button"
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
      </section>

      {node.type !== Type.Task && (
        <section>
          <h3>add {isEpic ? "issue" : "task"}</h3>
          <div className="addlink">
            <input placeholder="title" value={childTitle} onChange={(e) => setChildTitle(e.target.value)} />
            <button type="button" onClick={addChild}>Add</button>
          </div>
        </section>
      )}

      {!isEpic && (
        <section>
          <h3>move</h3>
          <div className="addlink">
            <select value={moveTo} onChange={(e) => setMoveTo(e.target.value)}>
              <option value="">choose new parent…</option>
              {moveTargets().map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
            </select>
            <button type="button" disabled={!moveTo} onClick={() => run(api.move(node.ref, moveTo))}>Move</button>
          </div>
        </section>
      )}

      <section className="danger">
        <button
          type="button"
          onClick={async () => {
            if (!confirm(`Delete ${node.type} "${node.title}"${node.type !== "task" ? " and its subtree" : ""}?`)) return;
            await run(api.remove(node.ref));
            onSelect("");
          }}
        >
          Delete {node.type}
        </button>
      </section>
    </div>
  );
}
