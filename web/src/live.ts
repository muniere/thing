import { useEffect, useRef } from "react";

// useLiveReload subscribes to thingd's SSE stream and runs the callback on every
// "reload" event. The callback is held in a ref so a changing closure does not
// tear down and reopen the EventSource on each render.
//
// The "hello" frame carries the server's per-process bootID. The first hello
// records it and refetches to catch up; a later hello with a *different* id means
// the server was replaced (in dev, air rebuilt the binary; in prod, a restart),
// so the page reloads to pick up the new JS/CSS. A reconnect to the same process
// keeps the id and only refetches — the same catch-up a plain reconnect needs.
export function useLiveReload(onReload: () => void): void {
  const cb = useRef(onReload);
  cb.current = onReload;

  useEffect(() => {
    let boot: string | null = null;
    const es = new EventSource("/events");
    es.addEventListener("reload", () => cb.current());
    es.addEventListener("hello", (e) => {
      const id = (e as MessageEvent).data;
      if (boot !== null && boot !== id) {
        location.reload(); // server replaced with a new build — pull it in
        return;
      }
      boot = id;
      cb.current(); // catch up on anything missed while the stream was down
    });
    es.onerror = () => {
      // EventSource auto-retries transient drops; a CLOSED state is terminal, so
      // live-reload has stopped and the view may silently go stale.
      if (es.readyState === EventSource.CLOSED) {
        console.error("live-reload stream closed; reload the page to resync");
      }
    };
    return () => es.close();
  }, []);
}
