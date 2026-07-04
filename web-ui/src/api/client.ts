import type {
  SyncTask,
  SyncLog,
  CopyJob,
  Settings,
  BrowseResponse,
  TestConnectionResult,
  SyncProgress,
  CopyTaskProgress,
  PaginatedResponse,
  MonitorConfig,
  MonitorDir,
} from "../types";

const BASE = "/api";

let onUnauthorized: (() => void) | null = null;

export function setOnUnauthorized(fn: () => void) {
  onUnauthorized = fn;
}

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem("auth_token");
  if (token) {
    return { Authorization: `Bearer ${token}` };
  }
  return {};
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown
): Promise<T> {
  const headers: Record<string, string> = { ...getAuthHeaders() };
  if (body) {
    headers["Content-Type"] = "application/json";
  }
  const res = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401) {
    localStorage.removeItem("auth_token");
    onUnauthorized?.();
    throw new Error("未登录");
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: res.statusText }));
    throw new Error(err.message || `HTTP ${res.status}`);
  }
  return res.json();
}

export const settingsApi = {
  get: () => request<Settings>("GET", "/settings"),
  update: (data: Partial<Settings>) =>
    request<{ success: boolean }>("PUT", "/settings", data),
  test: () => request<TestConnectionResult>("POST", "/settings/test"),
};

export const tasksApi = {
  list: () => request<SyncTask[]>("GET", "/tasks"),
  get: (id: number) => request<SyncTask>("GET", `/tasks/${id}`),
  create: (data: Partial<SyncTask>) =>
    request<SyncTask>("POST", "/tasks", data),
  update: (id: number, data: Partial<SyncTask>) =>
    request<SyncTask>("PUT", `/tasks/${id}`, data),
  delete: (id: number) =>
    request<{ success: boolean }>("DELETE", `/tasks/${id}`),
  start: (id: number) => request<SyncTask>("POST", `/tasks/${id}/start`),
  stop: (id: number) => request<SyncTask>("POST", `/tasks/${id}/stop`),
  trigger: (id: number) =>
    request<{ message: string; task_id: number }>("POST", `/tasks/${id}/trigger`),
  logs: (id: number, page: number = 1, perPage: number = 50) =>
    request<PaginatedResponse<SyncLog>>(
      "GET",
      `/tasks/${id}/logs?page=${page}&per_page=${perPage}`
    ),
  jobs: (id: number) => request<CopyJob[]>("GET", `/tasks/${id}/jobs`),
  deleteJob: (id: number, jobId: number) =>
    request<{ success: boolean }>("DELETE", `/tasks/${id}/jobs/${jobId}`),
};

export const browseApi = {
  list: (path: string, page: number = 1, perPage: number = 100) =>
    request<BrowseResponse>("POST", "/browse/list", {
      path,
      page,
      per_page: perPage,
    }),
  dirs: (path: string) =>
    request<BrowseResponse>("POST", "/browse/dirs", { path }),
};

export const syncApi = {
  status: () =>
    request<{
      runningCount: number;
      tasks: Array<{
        id: number;
        name: string;
        status: string;
        lastSyncAt: string | null;
      }>;
    }>("GET", "/sync/status"),
  progress: (taskId: number) =>
    request<SyncProgress>("GET", `/tasks/${taskId}/progress`),
  clearLogs: () =>
    request<{ success: boolean }>("DELETE", "/logs"),
};

export const openlistApi = {
  copyTasks: () =>
    request<{
      tasks: CopyTaskProgress[];
      count: number;
      error?: string;
    }>("GET", "/openlist/copy-tasks"),
};

export const monitorApi = {
  getConfig: () => request<MonitorConfig>("GET", "/monitor/config"),
  updateConfig: (data: Partial<MonitorConfig>) =>
    request<MonitorConfig>("PUT", "/monitor/config", data),
  listDirs: () =>
    request<{ main: MonitorDir[]; chasing: MonitorDir[] }>(
      "GET",
      "/monitor/dirs"
    ),
  addDir: (path: string, kind: MonitorDir["kind"]) =>
    request<MonitorDir>("POST", "/monitor/dirs", { path, kind }),
  deleteDir: (id: number) =>
    request<{ success: boolean }>("DELETE", `/monitor/dirs/${id}`),
  trigger: () =>
    request<{ message: string; running: boolean }>(
      "POST",
      "/monitor/trigger"
    ),
  status: () => request<{ running: boolean }>("GET", "/monitor/status"),
  logs: (page: number = 1, perPage: number = 50) =>
    request<PaginatedResponse<SyncLog>>(
      "GET",
      `/monitor/logs?page=${page}&per_page=${perPage}`
    ),
};
