import { useState } from "react";
import type { Api, ArchiveEntry } from "../api.ts";
import s from "./ArchivedPanel.module.css";

interface Props {
  api: Api;
  entries: ArchiveEntry[];
  run: <T>(p: Promise<T>) => Promise<T | undefined>;
}

// ArchivedPanel is a collapsible list of shelved subtrees below the tree. Each
// row shows where it came from and offers a one-click restore to that ref. A
// restore that collides or whose parent is gone fails (its error is shown by the
// app); the row then reveals a destination field so it can be retried elsewhere,
// the web equivalent of `unarchive --to`.
export function ArchivedPanel({ api, entries, run }: Props) {
  const [open, setOpen] = useState(false);
  const [retry, setRetry] = useState<{ ref: string; to: string } | null>(null);
  if (entries.length === 0) return null;

  const restore = async (e: ArchiveEntry, to?: string) => {
    const name = e.ref.replace(/^_archives\//, "");
    // run() surfaces any error as a banner and resolves to undefined on failure.
    const res = await run(api.unarchive(name, to));
    setRetry(res === undefined ? { ref: e.ref, to: to ?? e.from } : null);
  };

  return (
    <div className={s.panel}>
      <button type="button" className={s.head} aria-expanded={open} onClick={() => setOpen((o) => !o)}>
        <span className={s.caret} data-open={open}>▸</span>
        Archived
        <span className={s.count}>{entries.length}</span>
      </button>
      {open && (
        <ul className={s.list}>
          {entries.map((e) => (
            <li key={e.ref} className={s.row} data-status={e.status}>
              <div className={s.text}>
                <div className={s.title}>{e.title || e.ref}</div>
                <div className={s.from}>
                  {e.from}
                  {e.archivedAt && <span className={s.date}> · {e.archivedAt}</span>}
                </div>
                {retry?.ref === e.ref && (
                  <div className={s.retry}>
                    <input
                      className={s.dest}
                      value={retry.to}
                      placeholder="restore to ref…"
                      onChange={(ev) => setRetry({ ref: e.ref, to: ev.target.value })}
                    />
                    <button
                      type="button"
                      className={s.restore}
                      disabled={retry.to.trim() === ""}
                      onClick={() => restore(e, retry.to.trim())}
                    >
                      Restore here
                    </button>
                  </div>
                )}
              </div>
              <button type="button" className={s.restore} onClick={() => restore(e)}>
                Restore
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
