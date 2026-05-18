import { useState, useEffect, useCallback } from "react";
import { syncApi } from "../api/client";
import type { SyncProgress } from "../types";

export function useSyncProgress(taskId: number, isRunning: boolean) {
  const [progress, setProgress] = useState<SyncProgress | null>(null);

  const fetchProgress = useCallback(async () => {
    try {
      const data = await syncApi.progress(taskId);
      setProgress(data);
    } catch {
      // ignore polling errors
    }
  }, [taskId]);

  useEffect(() => {
    if (!isRunning) {
      setProgress(null);
      return;
    }
    fetchProgress();
    const id = setInterval(fetchProgress, 5000);
    return () => clearInterval(id);
  }, [isRunning, fetchProgress]);

  return progress;
}
