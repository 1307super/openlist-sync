export interface SyncTask {
  id: number;
  name: string;
  sourcePath: string;
  destPath: string;
  completionRule: "keep" | "delete_source";
  replaceRule: "skip" | "overwrite";
  matchMode: "exact" | "smart";
  scanIntervalSec: number;
  enabled: boolean;
  status: "idle" | "running" | "paused" | "error";
  lastScanAt: string | null;
  lastSyncAt: string | null;
  error: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface SyncLog {
  id: number;
  taskId: number;
  level: "info" | "warn" | "error";
  message: string;
  details: string | null;
  createdAt: string;
}

export interface CopyJob {
  id: number;
  taskId: number;
  fileName: string;
  srcDir: string;
  dstDir: string;
  openlistTaskId: string | null;
  status: "pending" | "copying" | "completed" | "failed";
  retryCount: number;
  error: string | null;
  createdAt: string;
  completedAt: string | null;
}

export interface Settings {
  openlist_base_url?: string;
  openlist_token?: string;
  tg_bot_token?: string;
  tg_chat_id?: string;
  auth_username?: string;
}

export interface FileEntry {
  name: string;
  size: number;
  is_dir: boolean;
  modified: string;
}

export interface BrowseResponse {
  content: FileEntry[];
  total: number;
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
}

export interface TestConnectionResult {
  success: boolean;
  message: string;
}

export interface SyncCycleResult {
  taskId: number;
  scanned: number;
  missing: number;
  copied: number;
  failed: number;
  deleted: number;
  durationMs: number;
  error?: string;
}

export interface CopyTaskProgress {
  id: string;
  name: string;
  state: number;
  status: string;
  progress: number;
  totalBytes: number;
  error: string;
}

export interface SyncProgress {
  running: boolean;
  copyTasks: CopyTaskProgress[] | null;
  taskCount: number;
  error?: string;
}
