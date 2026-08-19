import type { MouseEvent, ReactNode } from "react";
import { ProjectSwitcher } from "../ProjectSwitcher/ProjectSwitcher.tsx";
import s from "./ProjectHeader.module.css";

interface Props {
  // The project this header belongs to, for the switcher's current entry.
  project: string;
  // The configured title, shown next to the dot.
  title: string;
  // Where the logo links, and what a plain click on it does. On the board it
  // clears the focus in place; on a node's screen it leaves for the board.
  href: string;
  onNav: (e: MouseEvent) => void;
  // Switch to another project by name, or to the picker (null).
  onSwitch: (name: string | null) => void;
  // The bar's right-hand slot: the board puts its +Epic button here, a node's
  // screen puts nothing.
  children?: ReactNode;
}

// ProjectHeader is the bar every view of a project wears: the logo, which is
// also the way back to the project's board, the switcher that leaves for another
// project, and one slot for whatever action the view offers.
export function ProjectHeader({ project, title, href, onNav, onSwitch, children }: Props) {
  return (
    <header className={s.header}>
      <div className={s.brandGroup}>
        <a className={s.brand} href={href} onClick={onNav}>
          <span className={s.dot} />{title}
        </a>
        <ProjectSwitcher current={project} onSwitch={onSwitch} />
      </div>
      {children && <div className={s.slot}>{children}</div>}
    </header>
  );
}
