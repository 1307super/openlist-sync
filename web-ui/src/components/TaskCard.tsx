import {
  Play,
  Square,
  RefreshCw,
  Pencil,
  Trash2,
  ArrowRight,
  Timer,
  Clock,
} from "lucide-react";
import type { SyncTask } from "../types";

interface TaskCardProps {
  task: SyncTask;
  onStart: (id: number) => void;
  onStop: (id: number) => void;
  onTrigger: (id: number) => void;
  onEdit: (task: SyncTask) => void;
  onDelete: (id: number) => void;
}

const statusLabels: Record<SyncTask["status"], string> = {
  idle: "等待中",
  running: "运行中",
  paused: "已暂停",
  error: "错误",
};

function formatRelative(dateStr: string | null): string {
  if (!dateStr) return "从未";
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);

  if (diffSec < 60) return "刚刚";
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin} 分钟前`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr} 小时前`;
  const diffDay = Math.floor(diffHr / 24);
  return `${diffDay} 天前`;
}

function formatInterval(seconds: number): string {
  if (seconds < 60) return `每 ${seconds} 秒`;
  const min = Math.floor(seconds / 60);
  if (min < 60) return `每 ${min} 分钟`;
  const hr = Math.floor(min / 60);
  return `每 ${hr} 小时`;
}

const statusStyles: Record<SyncTask["status"], string> = {
  idle: "bg-green-500/15 text-green-400",
  running: "bg-blue-500/20 text-blue-400",
  paused: "bg-yellow-500/20 text-yellow-400",
  error: "bg-red-500/20 text-red-400",
};

export default function TaskCard({
  task,
  onStart,
  onStop,
  onTrigger,
  onEdit,
  onDelete,
}: TaskCardProps) {
  const isRunning = task.status === "running";

  return (
    <div className="bg-slate-800 rounded-lg p-4 border border-slate-700/50 hover:border-slate-600 transition-colors">
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2.5 mb-1">
            <h3 className="text-sm font-semibold text-white truncate">
              {task.name}
            </h3>
            <span
              className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium shrink-0 ${statusStyles[task.status]}`}
            >
              {task.status === "running" && (
                <span className="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse" />
              )}
              {statusLabels[task.status]}
            </span>
          </div>
          <div className="flex items-center gap-1.5 text-xs text-slate-400 truncate">
            <span className="truncate" title={task.sourcePath}>
              {task.sourcePath}
            </span>
            <ArrowRight className="w-3 h-3 shrink-0 text-slate-600" />
            <span className="truncate" title={task.destPath}>
              {task.destPath}
            </span>
          </div>
        </div>
      </div>

      <div className="flex items-center gap-4 text-xs text-slate-500 mb-3">
        <span className="flex items-center gap-1">
          <Clock className="w-3.5 h-3.5" />
          {formatRelative(task.lastSyncAt)}
        </span>
        <span className="flex items-center gap-1">
          <Timer className="w-3.5 h-3.5" />
          {formatInterval(task.scanIntervalSec)}
        </span>
        {task.error && (
          <span className="text-red-400 truncate" title={task.error}>
            {task.error}
          </span>
        )}
      </div>

      <div className="flex items-center gap-1.5 flex-wrap">
        {isRunning ? (
          <button
            onClick={() => onStop(task.id)}
            className="p-2 rounded-lg text-yellow-400 hover:bg-yellow-500/10 transition-colors"
            title="停止"
          >
            <Square className="w-4 h-4" />
          </button>
        ) : (
          <button
            onClick={() => onStart(task.id)}
            className="p-2 rounded-lg text-green-400 hover:bg-green-500/10 transition-colors"
            title="启动"
          >
            <Play className="w-4 h-4" />
          </button>
        )}
        <button
          onClick={() => onTrigger(task.id)}
          className="p-2 rounded-lg text-blue-400 hover:bg-blue-500/10 transition-colors"
          title="立即同步"
        >
          <RefreshCw className="w-4 h-4" />
        </button>
        <button
          onClick={() => onEdit(task)}
          className="p-2 rounded-lg text-slate-400 hover:bg-slate-700 hover:text-white transition-colors"
          title="编辑"
        >
          <Pencil className="w-4 h-4" />
        </button>
        <button
          onClick={() => onDelete(task.id)}
          className="p-2 rounded-lg text-slate-400 hover:bg-red-500/10 hover:text-red-400 transition-colors"
          title="删除"
        >
          <Trash2 className="w-4 h-4" />
        </button>
      </div>
    </div>
  );
}
