import { useCallback, useEffect, useState } from "react";
import { App } from "./App.tsx";
import { ProjectList } from "./components/ProjectList/ProjectList.tsx";
import { type Scheme, setScheme as saveScheme, settings } from "./api.ts";
import { applyScheme } from "./theme.ts";

// The first URL path segment names the project; an empty path ("/") is the root
// picker. Everything after the project is that project's own ref (handled inside
// App).
function projectFromPath(): string | null {
  const seg = window.location.pathname.replace(/^\/+/, "").split("/")[0];
  return seg || null;
}

// Root owns the top-level route — which project is open, or the picker at "/".
// It renders ProjectList when no project is selected and App (keyed by project so
// it remounts on a switch) otherwise, and keeps in sync with Back/Forward.
export function Root() {
  const [project, setProject] = useState<string | null>(projectFromPath);

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
    const onPop = () => setProject(projectFromPath());
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  // switchTo routes to a project (push /<name>) or to the picker (null → "/"),
  // remounting App via its project key. The picker cards and the in-project
  // switcher share it — the picker only ever passes a name, the switcher may pass
  // null for "All projects".
  const switchTo = useCallback((name: string | null) => {
    window.history.pushState(null, "", name ? `/${name}` : "/");
    setProject(name);
  }, []);

  if (!project) {
    return <ProjectList onOpen={switchTo} scheme={scheme} onScheme={chooseScheme} />;
  }
  return (
    <App
      key={project}
      project={project}
      onSwitch={switchTo}
      scheme={scheme}
      onScheme={chooseScheme}
      onRefresh={refreshScheme}
    />
  );
}
