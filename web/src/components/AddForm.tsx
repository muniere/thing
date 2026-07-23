import { useState } from "react";
import { Priority } from "../domain/generated.ts";
import { api } from "../api.ts";
import type { CreateInput } from "../api.ts";

interface Props {
  // The parent ref the new node is created under; "" creates a top-level epic.
  parent: string;
  // The word shown on the toggle button and in the title placeholder.
  noun: string;
  // Style the toggle as the primary (amber) button — used for the epic add.
  amber?: boolean;
  run: <T>(p: Promise<T>) => Promise<T | undefined>;
  onCreated: (ref: string) => void;
}

const PRIORITIES = [Priority.High, Priority.Medium, Priority.Low];

// AddForm is a button that expands into a create form, so a new node is entered
// through a form rather than a bare title box. category is offered only for a
// top-level epic (parent === ""), matching the server, which rejects a category
// on anything else. Every field but the title is optional; on success the form
// collapses and the new node is activated.
export function AddForm({ parent, noun, amber, run, onCreated }: Props) {
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [priority, setPriority] = useState("");
  const [tags, setTags] = useState("");
  const [category, setCategory] = useState("");
  const isEpic = parent === "";

  const close = () => {
    setOpen(false);
    setTitle("");
    setPriority("");
    setTags("");
    setCategory("");
  };

  const submit = async () => {
    const t = title.trim();
    if (!t) return;
    const input: CreateInput = { title: t };
    if (priority) input.priority = priority;
    const tagList = tags.split(",").map((s) => s.trim()).filter(Boolean);
    if (tagList.length) input.tags = tagList;
    if (isEpic && category.trim()) input.category = category.trim();
    const res = await run(api.create(parent, input));
    if (res) {
      close();
      onCreated(res.ref);
    }
  };

  if (!open) {
    return (
      <button type="button" className={amber ? "btn btn-amber" : "btn"} onClick={() => setOpen(true)}>
        + {noun}
      </button>
    );
  }

  return (
    <div className="add-form">
      <input
        className="input"
        autoFocus
        placeholder={`${noun} title`}
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") submit();
          else if (e.key === "Escape") close();
        }}
      />
      <div className="add-form-row">
        <select className="filter-select" aria-label="priority" value={priority} onChange={(e) => setPriority(e.target.value)}>
          <option value="">priority —</option>
          {PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
        </select>
        {isEpic && (
          <input className="input" placeholder="category" value={category} onChange={(e) => setCategory(e.target.value)} />
        )}
      </div>
      <input className="input" placeholder="tags (comma-separated)" value={tags} onChange={(e) => setTags(e.target.value)} />
      <div className="add-form-actions">
        <button type="button" className="btn btn-amber" onClick={submit} disabled={!title.trim()}>Add</button>
        <button type="button" className="btn-link" onClick={close}>Cancel</button>
      </div>
    </div>
  );
}
