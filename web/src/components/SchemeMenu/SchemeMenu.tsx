import { useEffect, useState } from "react";
import type { Scheme } from "../../api.ts";
import s from "./SchemeMenu.module.css";

interface Props {
  scheme: Scheme;
  onChange: (scheme: Scheme) => void;
}

// Half-filled circle, sun, moon: the conventional trio. Each is paired with its
// name wherever it appears — in the menu and on the resting button — since the
// icons are the only ones in this app expected to carry meaning by themselves.
const ICONS: Record<Scheme, React.ReactNode> = {
  auto: (
    <>
      <circle cx="12" cy="12" r="8" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="M12 4a8 8 0 0 1 0 16z" fill="currentColor" />
    </>
  ),
  light: (
    <>
      <circle cx="12" cy="12" r="4.5" fill="none" stroke="currentColor" strokeWidth="2" />
      <path
        d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.2 5.2l1.4 1.4M17.4 17.4l1.4 1.4M18.8 5.2l-1.4 1.4M6.6 17.4l-1.4 1.4"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
      />
    </>
  ),
  dark: <path d="M20 14.5A8.5 8.5 0 0 1 9.5 4a8.5 8.5 0 1 0 10.5 10.5z" fill="currentColor" />,
};

const OPTIONS: Scheme[] = ["auto", "light", "dark"];

// SchemeMenu picks the color scheme every board renders in. It floats at the
// bottom-right rather than sitting in the top bar: the setting is server-wide and
// set once, so it has no claim on the width of every board's chrome, and the
// corner is where a reader looks for it.
//
// Pressing it opens the three choices rather than cycling through them. Cycling
// hides what the next press will do, and reaching a particular scheme can take
// two presses; a menu says what it offers before it is used, and names each icon.
//
// "auto" is a real choice rather than the absence of one — it means "follow the
// system", which is a different thing from having chosen light and happening to
// be in daylight.
export function SchemeMenu({ scheme, onChange }: Props) {
  const [open, setOpen] = useState(false);

  // Close on any outside click or Escape, mirroring the project switcher's menu.
  // The button's own click stops propagation so it toggles rather than being
  // closed by this same listener.
  useEffect(() => {
    if (!open) return;
    const close = () => setOpen(false);
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    window.addEventListener("click", close);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div className={s.wrap} onClick={(e) => e.stopPropagation()}>
      {open && (
        <div className={s.menu} role="menu">
          {OPTIONS.map((option) => (
            <button
              key={option}
              type="button"
              role="menuitemradio"
              aria-checked={scheme === option}
              className={s.item}
              onClick={() => {
                setOpen(false);
                onChange(option);
              }}
            >
              <span className={s.check} aria-hidden="true">{scheme === option ? "✓" : ""}</span>
              <svg className={s.itemIcon} viewBox="0 0 24 24" aria-hidden="true">
                {ICONS[option]}
              </svg>
              {option}
            </button>
          ))}
        </div>
      )}
      <button
        type="button"
        className={s.button}
        title={`Color scheme: ${scheme}`}
        aria-label={`Color scheme: ${scheme}`}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <svg className={s.icon} viewBox="0 0 24 24" aria-hidden="true">
          {ICONS[scheme]}
        </svg>
        {/* Named as well as drawn: a sun and a moon are conventional, but a
            half-filled circle for "auto" is not, and the resting control should
            not require knowing the vocabulary to read the current state. */}
        <span className={s.label}>{scheme}</span>
      </button>
    </div>
  );
}
