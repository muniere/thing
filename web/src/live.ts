import { useEffect, useRef } from "react";

// useLiveReload subscribes to thingd's SSE stream and runs the callback on every
// "reload" event. The callback is held in a ref so a changing closure does not
// tear down and reopen the EventSource on each render.
export function useLiveReload(onReload: () => void): void {
  const cb = useRef(onReload);
  cb.current = onReload;

  useEffect(() => {
    const es = new EventSource("/events");
    es.addEventListener("reload", () => cb.current());
    // On (re)connect, refetch once to catch up on any change missed while the
    // stream was down — the server only pushes on new events, not on connect.
    es.addEventListener("open", () => cb.current());
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
