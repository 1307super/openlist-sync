import { useState, useEffect, useCallback } from "react";
import { tasksApi } from "../api/client";
import type { SyncTask } from "../types";

export function useTasks() {
  const [tasks, setTasks] = useState<SyncTask[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchTasks = useCallback(async () => {
    try {
      const data = await tasksApi.list();
      setTasks(data);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchTasks();
  }, [fetchTasks]);

  const createTask = async (data: Partial<SyncTask>) => {
    const task = await tasksApi.create(data);
    setTasks((prev) => [task, ...prev]);
    return task;
  };

  const updateTask = async (id: number, data: Partial<SyncTask>) => {
    const task = await tasksApi.update(id, data);
    setTasks((prev) => prev.map((t) => (t.id === id ? task : t)));
    return task;
  };

  const deleteTask = async (id: number) => {
    await tasksApi.delete(id);
    setTasks((prev) => prev.filter((t) => t.id !== id));
  };

  const startTask = async (id: number) => {
    const task = await tasksApi.start(id);
    setTasks((prev) => prev.map((t) => (t.id === id ? task : t)));
  };

  const stopTask = async (id: number) => {
    const task = await tasksApi.stop(id);
    setTasks((prev) => prev.map((t) => (t.id === id ? task : t)));
  };

  const triggerTask = async (id: number) => {
    return tasksApi.trigger(id);
  };

  return {
    tasks,
    loading,
    fetchTasks,
    createTask,
    updateTask,
    deleteTask,
    startTask,
    stopTask,
    triggerTask,
  };
}
