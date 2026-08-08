import { type RefObject, useEffect, useRef, useState } from "react";
import type { Node, NodeLink } from "../../domain/generated.ts";
import { Type } from "../../domain/generated.ts";
import type { Api } from "../../api.ts";
import { flatten } from "../../util.ts";

interface Input {
  api: Api;
  node: Node;
  allNodes: Node[];
  run: <T>(p: Promise<T>) => Promise<T | undefined>;
  onSelect: (ref: string) => void;
}

// MoveTarget is one choice in the change-parent picker: the ref to move under
// (or "_orphan"), labelled by the target's title.
export interface MoveTarget {
  value: string;
  label: string;
}

export interface NodeDetailPanelState {
  // The title/category editor, which replaces the heading while open.
  editor: {
    editing: boolean;
    title: string;
    setTitle: (title: string) => void;
    category: string;
    setCategory: (category: string) => void;
    start: () => void;
    save: () => void;
    cancel: () => void;
  };
  // The body, as a rendered preview or an editable draft.
  body: {
    preview: boolean;
    togglePreview: () => void;
    draft: string;
    setDraft: (draft: string) => void;
    save: () => void;
  };
  // The ref's copy button, which flashes a check once the ref is on the clipboard.
  copy: {
    copied: boolean;
    copyRef: () => void;
  };
  // The related-links list and the inline form that adds to it.
  links: {
    adding: boolean;
    url: string;
    setURL: (url: string) => void;
    label: string;
    setLabel: (label: string) => void;
    start: () => void;
    add: () => void;
    cancel: () => void;
    remove: (link: NodeLink) => void;
  };
  // The change-parent modal. `dialog` drives the native <dialog> element, so
  // Escape and a backdrop click close it like any other modal.
  move: {
    open: boolean;
    dialog: RefObject<HTMLDialogElement | null>;
    targets: MoveTarget[];
    target: string;
    setTarget: (target: string) => void;
    start: () => void;
    submit: () => void;
    close: () => void;
    reset: () => void;
  };
  // The mutations with no draft state of their own. Each confirms first where it
  // is destructive or hard to undo.
  actions: {
    setStatus: (status: string) => void;
    setPriority: (priority: string) => void;
    resetStatus: () => void;
    archive: () => void;
    remove: () => void;
  };
}

// useNodeDetailPanel holds everything the detail panel does, leaving the panel
// itself to be markup. The panel is remounted per selection (via a key on the
// element), so every draft here starts from the node it was mounted for and
// resets cleanly when the selection changes.
//
// Each mutation goes through `run`, which surfaces a failure as the board's error
// banner and reloads the tree on success, so nothing here handles errors itself.
export function useNodeDetailPanel({ api, node, allNodes, run, onSelect }: Input): NodeDetailPanelState {
  const isEpic = node.type === Type.Epic;

  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState(node.title);
  const [category, setCategory] = useState(node.category ?? "");
  const [body, setBody] = useState(node.body ?? "");
  const [preview, setPreview] = useState(true);
  const [copied, setCopied] = useState(false);
  const [addingLink, setAddingLink] = useState(false);
  const [linkURL, setLinkURL] = useState("");
  const [linkLabel, setLinkLabel] = useState("");
  const [moving, setMoving] = useState(false);
  const [moveTo, setMoveTo] = useState("");

  // The check on the copy button is cleared on a timer, which has to be dropped
  // if the panel goes away first.
  const copiedTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => () => clearTimeout(copiedTimer.current), []);

  // The change-parent picker is a modal dialog; drive the native <dialog> from the
  // moving flag so Escape and the backdrop close it (its onClose calls reset).
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

  // Copy the ref to the clipboard and flash a check on the button. The async
  // Clipboard API is missing on insecure origins (thingd served over plain HTTP
  // to a non-localhost host), so fall back to a hidden textarea + execCommand.
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

  const closeLinkForm = () => {
    setAddingLink(false);
    setLinkURL("");
    setLinkLabel("");
  };

  const addLink = async () => {
    if (!linkURL.trim()) return;
    await run(api.addLink(node.ref, linkURL.trim(), linkLabel.trim()));
    closeLinkForm();
  };

  // Valid move targets: an issue moves under an epic or to orphan; a task moves
  // under another issue. Epics stay at the root.
  const moveTargets = (): MoveTarget[] => {
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

  const submitMove = async () => {
    // A move re-slugs the node (its ref carries the parent path), so follow the
    // returned new ref — otherwise the selection points at the old ref and the
    // panel goes blank after the reload.
    const res = await run(api.move(node.ref, moveTo));
    if (res) onSelect(res.ref);
  };

  // The subtree wording is shared by the archive and delete confirmations, which
  // both take one with the node when it has children to take.
  const subtree = node.type !== Type.Task ? " and its subtree" : "";

  return {
    editor: {
      editing,
      title,
      setTitle,
      category,
      setCategory,
      start: () => setEditing(true),
      save: saveTitle,
      cancel: cancelEdit,
    },
    body: {
      preview,
      togglePreview: () => setPreview((p) => !p),
      draft: body,
      setDraft: setBody,
      save: () => void run(api.body(node.ref, body)),
    },
    copy: { copied, copyRef: () => void copyRef() },
    links: {
      adding: addingLink,
      url: linkURL,
      setURL: setLinkURL,
      label: linkLabel,
      setLabel: setLinkLabel,
      start: () => setAddingLink(true),
      add: () => void addLink(),
      cancel: closeLinkForm,
      remove: (link) => {
        const name = link.label || link.url;
        if (!confirm(`Remove link "${name}"?`)) return;
        void run(api.removeLink(node.ref, link.url));
      },
    },
    move: {
      open: moving,
      dialog: moveDialog,
      targets: moveTargets(),
      target: moveTo,
      setTarget: setMoveTo,
      start: () => setMoving(true),
      submit: () => void submitMove(),
      close: () => moveDialog.current?.close(),
      reset: () => {
        setMoving(false);
        setMoveTo("");
      },
    },
    actions: {
      setStatus: (status) => void run(api.status(node.ref, status)),
      setPriority: (priority) => void run(api.priority(node.ref, priority)),
      resetStatus: () => {
        if (!confirm(`Reset this ${node.type}'s status to auto (roll up from its children)?`)) return;
        void run(api.status(node.ref, ""));
      },
      archive: async () => {
        if (!confirm(`Archive ${node.type} "${node.title}"${subtree}?`)) return;
        await run(api.archive(node.ref));
        onSelect("");
      },
      remove: async () => {
        if (!confirm(`Delete ${node.type} "${node.title}"${subtree}?`)) return;
        await run(api.remove(node.ref));
        onSelect("");
      },
    },
  };
}
