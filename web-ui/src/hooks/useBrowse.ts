import { useState, useCallback } from "react";
import { browseApi } from "../api/client";
import type { FileEntry } from "../types";

export function useBrowse() {
  const [currentPath, setCurrentPath] = useState("/");
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const navigate = useCallback(async (path: string) => {
    try {
      setLoading(true);
      setError(null);
      const result = await browseApi.dirs(path);
      setEntries(result.content.filter((e) => e.is_dir));
      setCurrentPath(path || "/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to browse");
    } finally {
      setLoading(false);
    }
  }, []);

  const navigateInto = useCallback(
    (name: string) => {
      const newPath =
        currentPath === "/" ? `/${name}` : `${currentPath}/${name}`;
      navigate(newPath);
    },
    [currentPath, navigate]
  );

  const navigateUp = useCallback(() => {
    const parts = currentPath.split("/").filter(Boolean);
    parts.pop();
    navigate("/" + parts.join("/"));
  }, [currentPath, navigate]);

  return {
    currentPath,
    entries,
    loading,
    error,
    navigate,
    navigateInto,
    navigateUp,
  };
}
