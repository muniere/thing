import { useState } from "react";
import type { Api, ArchiveDetail, ArchiveEntry } from "../../api.ts";
import s from "./ArchiveList.module.css";

interface Props {
  api: Api;
  entries: ArchiveEntry[];
  run: <T>(p: Promise<T>) => Promise<T | undefined>;
}

const nameOf = (ref: string) => ref.replace(/^_archives\//, "");

// ArchiveList is a collapsible list of shelved subtrees below the tree. A row
// shows where it came from; clicking it expands a read-only detail (the web
// equivalent of `show _archives/<name>`). Restore sends it back to where it came
// from; if that fails (occupied ref or missing parent) the row reveals a
// destination field to retry elsewhere, like `unarchive --to`.
export function ArchiveList({ api, entries, run }: Props) {
  const [open, setOpen] = useState(false);
  const [retry, setRetry] = useState<{ ref: string; to: string } | null>(null);
  const [detail, setDetail] = useState<{ ref: string; node?: ArchiveDetail } | null>(null);
  if (entries.length === 0) return null;

  const restore = async (e: ArchiveEntry, to?: string) => {
    const dest = to ?? e.from;
    // Confirm before moving the node back, mirroring the Archive action.
    if (!confirm(`Restore "${e.title || e.ref}" to ${dest}?`)) return;
    // run() surfaces any error as a banner and resolves to undefined on failure.
    const res = await run(api.unarchive(nameOf(e.ref), to));
    setRetry(res === undefined ? { ref: e.ref, to: dest } : null);
  };

  const toggleDetail = async (e: ArchiveEntry) => {
    if (detail?.ref === e.ref) {
      setDetail(null);
      return;
    }
    setDetail({ ref: e.ref });
    const node = await run(api.getArchive(nameOf(e.ref)));
    if (node) setDetail({ ref: e.ref, node });
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
              <div className={s.main}>
                <button type="button" className={s.text} aria-expanded={detail?.ref === e.ref} onClick={() => toggleDetail(e)}>
                  <div className={s.title}>{e.title || e.ref}</div>
                  <div className={s.from}>
                    {e.from}
                    {e.archivedAt && <span className={s.date}> · {e.archivedAt}</span>}
                  </div>
                </button>
                <button type="button" className={s.restore} onClick={() => restore(e)}>
                  Restore
                </button>
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

              {detail?.ref === e.ref && (
                <div className={s.detail}>
                  {detail.node ? (
                    <>
                      <div className={s.meta}>
                        {[e.type, detail.node.status, detail.node.priority, detail.node.category]
                          .filter(Boolean)
                          .join(" · ")}
                        {detail.node.tags?.length ? ` · ${detail.node.tags.join(", ")}` : ""}
                      </div>
                      {detail.node.body ? (
                        <pre className={s.body}>{detail.node.body.replace(/\n+$/, "")}</pre>
                      ) : (
                        <div className={s.empty}>no body</div>
                      )}
                    </>
                  ) : (
                    <div className={s.empty}>loading…</div>
                  )}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
