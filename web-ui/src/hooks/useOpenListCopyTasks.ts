import { useState, useEffect, useCallback } from "react";
import { openlistApi } from "../api/client";
import type { CopyTaskProgress } from "../types";

export function useOpenListCopyTasks(intervalMs: number = 5000) {
  const [tasks, setTasks] = useState<CopyTaskProgress[]>([]);

  const fetchTasks = useCallback(async () => {
    try {
      const res = await openlistApi.copyTasks();
      setTasks(res.tasks ?? []);
    } catch {
      // ignore polling errors
    }
  }, []);

  useEffect(() => {
    fetchTasks();
    const id = setInterval(fetchTasks, intervalMs);
    return () => clearInterval(id);
  }, [fetchTasks, intervalMs]);

  return tasks;
}
