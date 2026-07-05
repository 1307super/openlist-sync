import { useState, useEffect, useCallback } from "react";
import { monitorApi } from "../api/client";
import type { MonitorLog } from "../types";
import {
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  ChevronRight as ExpandIcon,
  RefreshCw,
  Loader2,
} from "lucide-react";

interface MonitorLogViewerProps {
  /** 运行中时自动刷新（轮询） */
  isRunning?: boolean;
}

const PER_PAGE = 30;

const levelColors: Record<string, string> = {
  info: "bg-blue-500/20 text-blue-400 border border-blue-500/30",
  warn: "bg-yellow-500/20 text-yellow-400 border border-yellow-500/30",
  error: "bg-red-500/20 text-red-400 border border-red-500/30",
};

const levelDots: Record<string, string> = {
  info: "bg-blue-400",
  warn: "bg-yellow-400",
  error: "bg-red-400",
};

type LevelFilter = "all" | "info" | "warn" | "error";

export default function MonitorLogViewer({ isRunning }: MonitorLogViewerProps) {
  const [logs, setLogs] = useState<MonitorLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [levelFilter, setLevelFilter] = useState<LevelFilter>("all");
  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set());
  const [autoRefresh, setAutoRefresh] = useState(true);

  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));

  const fetchLogs = useCallback(async () => {
    try {
      const res = await monitorApi.logs(page, PER_PAGE);
      setLogs(res.items ?? []);
      setTotal(res.total);
    } catch {
      /* ignore */
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    fetchLogs();
  }, [fetchLogs]);

  // 运行中自动轮询
  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(fetchLogs, isRunning ? 3000 : 10000);
    return () => clearInterval(interval);
  }, [autoRefresh, isRunning, fetchLogs]);

  const toggleExpand = (id: number) => {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const filtered =
    levelFilter === "all" ? logs : logs.filter((l) => l.level === levelFilter);

  const formatTime = (iso: string) => {
    const d = new Date(iso);
    return d.toLocaleString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  };

  const filters: { key: LevelFilter; label: string }[] = [
    { key: "all", label: "全部" },
    { key: "info", label: "信息" },
    { key: "warn", label: "警告" },
    { key: "error", label: "错误" },
  ];

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-1">
          {filters.map((f) => (
            <button
              key={f.key}
              onClick={() => setLevelFilter(f.key)}
              className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                levelFilter === f.key
                  ? "bg-slate-700 text-white"
                  : "text-slate-400 hover:text-slate-200 hover:bg-slate-800"
              }`}
            >
              {f.label}
            </button>
          ))}
        </div>
        <button
          onClick={() => {
            setLoading(true);
            fetchLogs();
          }}
          className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-xs font-medium transition-colors ${
            autoRefresh
              ? "text-primary bg-primary/10"
              : "text-slate-400 hover:text-slate-200 hover:bg-slate-800"
          }`}
          title="切换自动刷新"
        >
          <RefreshCw className="w-3.5 h-3.5" />
          {autoRefresh ? "自动" : "手动"}
        </button>
      </div>

      <div className="bg-slate-900/50 rounded-lg border border-slate-800 divide-y divide-slate-800/50 min-h-[120px] max-h-[480px] overflow-y-auto">
        {loading && logs.length === 0 ? (
          <div className="flex items-center justify-center py-12 text-slate-500 text-sm">
            <Loader2 className="w-4 h-4 animate-spin mr-2" />
            加载日志中...
          </div>
        ) : filtered.length === 0 ? (
          <div className="flex items-center justify-center py-12 text-slate-500 text-sm">
            暂无日志
          </div>
        ) : (
          filtered.map((log) => (
            <div key={log.id} className="group">
              <div
                className={`flex items-start gap-3 px-4 py-2.5 hover:bg-slate-800/30 transition-colors ${
                  log.details ? "cursor-pointer" : ""
                }`}
                onClick={() => log.details && toggleExpand(log.id)}
              >
                <span className="text-xs text-slate-500 font-mono whitespace-nowrap pt-0.5 shrink-0">
                  {formatTime(log.createdAt)}
                </span>
                <span
                  className={`shrink-0 inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wide ${
                    levelColors[log.level]
                  }`}
                >
                  <span
                    className={`w-1.5 h-1.5 rounded-full mr-1.5 ${
                      levelDots[log.level]
                    }`}
                  />
                  {log.level}
                </span>
                <span className="text-sm text-slate-200 flex-1 break-all leading-relaxed">
                  {log.message}
                </span>
                {log.details && (
                  <span className="shrink-0 text-slate-600 group-hover:text-slate-400 transition-colors">
                    {expandedIds.has(log.id) ? (
                      <ChevronDown className="w-4 h-4" />
                    ) : (
                      <ExpandIcon className="w-4 h-4" />
                    )}
                  </span>
                )}
              </div>
              {log.details && expandedIds.has(log.id) && (
                <div className="px-4 pb-3 pl-[calc(1rem+7.5rem)]">
                  <pre className="text-xs text-slate-400 bg-slate-950/50 rounded-md p-3 overflow-x-auto whitespace-pre-wrap font-mono border border-slate-800/50">
                    {tryFormatJson(log.details)}
                  </pre>
                </div>
              )}
            </div>
          ))
        )}
      </div>

      {total > PER_PAGE && (
        <div className="flex items-center justify-between">
          <button
            disabled={page <= 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium text-slate-300 bg-slate-800 hover:bg-slate-700 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            <ChevronLeft className="w-4 h-4" />
            上一页
          </button>
          <span className="text-sm text-slate-500">
            第 {page} 页 / 共 {totalPages} 页
          </span>
          <button
            disabled={page >= totalPages}
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium text-slate-300 bg-slate-800 hover:bg-slate-700 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            下一页
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>
      )}
    </div>
  );
}

function tryFormatJson(text: string): string {
  try {
    const parsed = JSON.parse(text);
    return JSON.stringify(parsed, null, 2);
  } catch {
    return text;
  }
}
