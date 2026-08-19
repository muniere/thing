import { useCallback, useEffect, useState } from "react";
import { App } from "./App.tsx";
import { NodeScreen } from "./NodeScreen.tsx";
import { ProjectList } from "./components/ProjectList/ProjectList.tsx";
import { type Scheme, setScheme as saveScheme, settings } from "./api.ts";
import { parseLocation, type Route } from "./route.ts";
import { applyScheme } from "./theme.ts";

// Root owns the top-level route: the picker at "/", a project's board, or one
// node on a screen of its own. It renders the matching view — keyed so a board
// remounts per project and a node's screen per node — and keeps in sync with
// Back/Forward.
//
// It does not own the board's focus. Which node the board has selected is read
// and written by useApp, straight from the query, so keyboard nav and filter
// changes never make a round trip through the router; Root's copy of that ref
// goes stale on the board, and nothing reads it.
export function Root() {
  const [route, setRoute] = useState<Route>(parseLocation);

  // The color scheme is server-wide, so it is owned here rather than by either
  // view: the picker and a board both wear it, and both offer the control.
  const [scheme, setSchemeState] = useState<Scheme>("auto");
  const refreshScheme = useCallback(async () => {
    try {
      const { scheme } = await settings();
      setSchemeState(scheme);
      applyScheme(scheme);
    } catch (e) {
      // Deliberately not surfaced: the page renders in the system's scheme,
      // which is what "auto" means anyway, so a failed fetch costs a preference
      // rather than the app.
      console.warn("GET /api/settings failed; following the system color scheme", e);
    }
  }, []);
  useEffect(() => {
    void refreshScheme();
  }, [refreshScheme]);

  // Applied before the request so the page turns as the button is pressed, and
  // put back if the server refuses — the setting is not worth a spinner, but a
  // silently reverted click would be a lie.
  const chooseScheme = useCallback(
    (next: Scheme) => {
      const previous = scheme;
      setSchemeState(next);
      applyScheme(next);
      saveScheme(next).catch((e) => {
        console.warn("PATCH /api/settings failed; keeping the previous scheme", e);
        setSchemeState(previous);
        applyScheme(previous);
      });
    },
    [scheme],
  );

  useEffect(() => {
    const onPop = () => setRoute(parseLocation());
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  // navigate is the one in-app transition that can change which view is shown, so
  // it lives here. The board's own focus moves do not come through it (see the
  // note above); a node's screen has nowhere else to go, so all of its links do.
  const navigate = useCallback((href: string) => {
    window.history.pushState(null, "", href);
    setRoute(parseLocation());
  }, []);

  // switchTo routes to a project or to the picker (null → "/"). The picker cards
  // and the in-project switcher share it — the picker only ever passes a name,
  // the switcher may pass null for "All projects".
  const switchTo = useCallback((name: string | null) => navigate(name ? `/${name}/` : "/"), [navigate]);

  if (route.kind === "picker") {
    return <ProjectList onOpen={switchTo} scheme={scheme} onScheme={chooseScheme} />;
  }
  if (route.kind === "node") {
    return (
      <NodeScreen
        key={`${route.project}/${route.ref}`}
        project={route.project}
        nodeRef={route.ref}
        navigate={navigate}
        onSwitch={switchTo}
        scheme={scheme}
        onScheme={chooseScheme}
        onRefresh={refreshScheme}
      />
    );
  }
  return (
    <App
      key={route.project}
      project={route.project}
      onSwitch={switchTo}
      scheme={scheme}
      onScheme={chooseScheme}
      onRefresh={refreshScheme}
    />
  );
}
