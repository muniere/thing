import { useState } from "react";
import { Priority } from "../domain/generated.ts";
import type { Api, CreateInput } from "../api.ts";
import s from "./AddForm.module.css";

interface Props {
  // The per-project API client the create goes through.
  api: Api;
  // The parent ref the new node is created under; "" creates a top-level epic.
  parent: string;
  // The word used in the title placeholder ("<noun> title") and, by default, the
  // toggle button ("+ <noun>").
  noun: string;
  // Overrides the collapsed toggle's text (default "+ <noun>").
  label?: string;
  // Style the toggle as the primary (amber) button — used for the epic add.
  amber?: boolean;
  // Full-width, left-aligned toggle — used for the detail pane's child add.
  block?: boolean;
  // Drop the expanded form down from the toggle as a floating panel — used for the
  // top-bar epic add.
  floating?: boolean;
  run: <T>(p: Promise<T>) => Promise<T | undefined>;
  onCreated: (ref: string) => void;
}

const PRIORITIES = [Priority.High, Priority.Medium, Priority.Low];

// AddForm is a button that expands into a create form, so a new node is entered
// through a form rather than a bare title box. category is offered only for a
// top-level epic (parent === ""), matching the server, which rejects a category
// on anything else. Every field but the title is optional; on success the form
// collapses and the new node is activated.
export function AddForm({ api, parent, noun, label, amber, block, floating, run, onCreated }: Props) {
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

  // Floating mode overlays the form, so the toggle stays mounted to hold the top
  // bar's height; inline mode replaces the toggle with the form in normal flow.
  const toggleCls = [s.btn, amber ? s.btnAmber : "", block ? s.block : ""].filter(Boolean).join(" ");
  const toggle = (!open || floating) && (
    <button type="button" className={toggleCls} onClick={() => setOpen(true)}>
      {label ?? `+ ${noun}`}
    </button>
  );

  if (!open) return toggle || null;

  return (
    <>
      {toggle}
      <div className={`${s.form} ${floating ? s.floating : ""}`}>
      <input
        className={s.input}
        autoFocus
        placeholder={`${noun} title`}
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") submit();
          else if (e.key === "Escape") close();
        }}
      />
      <div className={s.row}>
        <select className={s.select} aria-label="priority" value={priority} onChange={(e) => setPriority(e.target.value)}>
          <option value="">priority —</option>
          {PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
        </select>
        {isEpic && (
          <input className={s.input} placeholder="category" value={category} onChange={(e) => setCategory(e.target.value)} />
        )}
      </div>
      <input className={s.input} placeholder="tags (comma-separated)" value={tags} onChange={(e) => setTags(e.target.value)} />
      <div className={s.actions}>
        <button type="button" className={`${s.btn} ${s.btnAmber}`} onClick={submit} disabled={!title.trim()}>Add</button>
        <button type="button" className={s.btnLink} onClick={close}>Cancel</button>
      </div>
      </div>
    </>
  );
}
