import { useCallback, useEffect, useState } from "react";
import { App } from "./App.tsx";
import { ProjectList } from "./components/ProjectList.tsx";

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

  useEffect(() => {
    const onPop = () => setProject(projectFromPath());
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  const open = useCallback((name: string) => {
    window.history.pushState(null, "", `/${name}`);
    setProject(name);
  }, []);

  const exit = useCallback(() => {
    window.history.pushState(null, "", "/");
    setProject(null);
  }, []);

  if (!project) {
    return <ProjectList onOpen={open} />;
  }
  return <App key={project} project={project} onExit={exit} />;
}
