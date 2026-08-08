import { type ReactNode, useEffect, useRef } from "react";
import s from "./Dialog.module.css";

interface Props {
  // Whether the dialog is shown; the component syncs the native element to it.
  open: boolean;
  // Called when the dialog closes by any route — Escape, backdrop click, or the
  // native close event — so the parent can flip its own open state.
  onClose: () => void;
  // Optional heading shown at the top of the panel.
  title?: string;
  // "wide" widens the panel to the width the detail pane caps content at, for
  // content that needs the room — the project edit dialog's theme preview is a
  // miniature board, and at the default width it scales down past legibility.
  // Everything else stays at the narrow default, where a form of one-line fields
  // reads better than a stretched one.
  size?: "default" | "wide";
  children: ReactNode;
}

// Dialog is a modal built on the native <dialog> element, so it gets a backdrop,
// focus trapping, and Escape-to-close for free. It opens/closes to follow `open`
// and reports every close through onClose, including a click on the backdrop
// outside the panel. The panel content mounts only while open, so form state
// resets between openings and autofocus fires each time.
export function Dialog({ open, onClose, title, size = "default", children }: Props) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    else if (!open && d.open) d.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className={size === "wide" ? `${s.dialog} ${s.wide}` : s.dialog}
      onClose={onClose}
      onClick={(e) => {
        // A click landing on the <dialog> itself (not its panel child) is a
        // backdrop click; the panel stops propagation so inner clicks don't close.
        if (e.target === ref.current) onClose();
      }}
    >
      {open && (
        <div className={s.panel} onClick={(e) => e.stopPropagation()}>
          {title && <h2 className={s.title}>{title}</h2>}
          {children}
        </div>
      )}
    </dialog>
  );
}
