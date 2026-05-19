import { HardDrive } from "lucide-react";
import type { CopyTaskProgress } from "../types";

function formatBytes(bytes: number): string {
  if (bytes === 0) return "";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let size = bytes;
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024;
    i++;
  }
  return ` ${size.toFixed(1)}${units[i]}`;
}

export default function GlobalProgress({
  tasks,
}: {
  tasks: CopyTaskProgress[];
}) {
  if (!tasks.length) return null;

  return (
    <div className="mb-6 rounded-lg border border-blue-500/20 bg-blue-500/5 p-4">
      <div className="flex items-center gap-2 mb-3">
        <HardDrive className="w-4 h-4 text-blue-400" />
        <span className="text-sm font-medium text-blue-400">
          OpenList 复制任务
        </span>
        {tasks.length > 1 && (
          <span className="text-xs text-slate-500">
            共 {tasks.length} 个任务进行中
          </span>
        )}
      </div>
      <div className="space-y-2.5">
        {tasks.map((ct) => {
          const pct = Math.round(ct.progress || 0);
          return (
            <div key={ct.id}>
              <div className="flex items-center justify-between text-xs mb-1">
                <span className="text-slate-300 truncate mr-2">
                  {ct.name || ct.status || "复制中"}
                </span>
                <span className="text-blue-400 shrink-0 tabular-nums">
                  {pct}%{formatBytes(ct.totalBytes)}
                </span>
              </div>
              <div className="h-1.5 bg-slate-700 rounded-full overflow-hidden">
                <div
                  className="h-full bg-blue-500 rounded-full transition-all duration-1000 ease-out"
                  style={{ width: `${Math.max(pct, 2)}%` }}
                />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
