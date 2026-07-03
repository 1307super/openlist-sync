import { useState, useEffect, useCallback } from "react";
import {
  Wand2,
  Loader2,
  Plus,
  Trash2,
  FolderPlus,
  Play,
  Power,
  Folder,
  RefreshCw,
} from "lucide-react";
import { monitorApi } from "../api/client";
import type { MonitorConfig, MonitorDir } from "../types";
import DirectoryPicker from "../components/DirectoryPicker";

export default function MonitorPage() {
  const [config, setConfig] = useState<MonitorConfig | null>(null);
  const [mainDirs, setMainDirs] = useState<MonitorDir[]>([]);
  const [chasingDirs, setChasingDirs] = useState<MonitorDir[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [triggering, setTriggering] = useState(false);
  const [intervalDraft, setIntervalDraft] = useState(1800);
  const [picker, setPicker] = useState<{
    kind: MonitorDir["kind"];
  } | null>(null);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [cfg, dirs] = await Promise.all([
        monitorApi.getConfig(),
        monitorApi.listDirs(),
      ]);
      setConfig(cfg);
      setIntervalDraft(cfg.scanIntervalSec);
      setMainDirs(dirs.main);
      setChasingDirs(dirs.chasing);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const toggleEnabled = useCallback(async () => {
    if (!config) return;
    try {
      setSaving(true);
      const updated = await monitorApi.updateConfig({
        enabled: !config.enabled,
      });
      setConfig(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }, [config]);

  const saveInterval = useCallback(async () => {
    try {
      setSaving(true);
      const updated = await monitorApi.updateConfig({
        scanIntervalSec: Math.max(10, intervalDraft),
      });
      setConfig(updated);
      setIntervalDraft(updated.scanIntervalSec);
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }, [intervalDraft]);

  const trigger = useCallback(async () => {
    try {
      setTriggering(true);
      await monitorApi.trigger();
    } catch (err) {
      setError(err instanceof Error ? err.message : "触发失败");
    } finally {
      setTriggering(false);
    }
  }, []);

  const handleAddDir = useCallback(
    async (path: string, kind: MonitorDir["kind"]) => {
      try {
        const dir = await monitorApi.addDir(path, kind);
        if (kind === "main") {
          setMainDirs((prev) => [...prev, dir]);
        } else {
          setChasingDirs((prev) => [...prev, dir]);
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "添加失败");
      }
    },
    []
  );

  const handleDeleteDir = useCallback(
    async (id: number, kind: MonitorDir["kind"]) => {
      try {
        await monitorApi.deleteDir(id);
        if (kind === "main") {
          setMainDirs((prev) => prev.filter((d) => d.id !== id));
        } else {
          setChasingDirs((prev) => prev.filter((d) => d.id !== id));
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "删除失败");
      }
    },
    []
  );

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto p-6 md:p-10 space-y-8">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white tracking-tight flex items-center gap-2">
            <Wand2 className="w-6 h-6 text-primary" />
            监控处理
          </h1>
          <p className="mt-1 text-sm text-slate-400">
            对主目录/追更目录执行 CAS 同步、目录大小重命名、纯 SxxExx 模板重命名、HiveWeb 标签添加。
          </p>
        </div>
        <button
          onClick={refresh}
          className="btn-secondary inline-flex items-center gap-2 shrink-0"
          title="刷新"
        >
          <RefreshCw className="w-4 h-4" />
          刷新
        </button>
      </div>

      {error && (
        <div className="px-4 py-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-sm text-rose-300">
          {error}
        </div>
      )}

      {/* 服务开关 */}
      <section className="bg-slate-800/50 rounded-xl border border-slate-700/50 overflow-hidden">
        <div className="flex items-center gap-2.5 px-6 py-4 border-b border-slate-700/50 bg-slate-800/80">
          <Power className="w-4 h-4 text-primary" />
          <h2 className="text-sm font-semibold text-white tracking-wide uppercase">
            服务开关
          </h2>
        </div>

        <div className="p-6 space-y-5">
          <div className="flex items-center justify-between gap-4">
            <div>
              <div className="text-sm text-slate-300">启用定时处理</div>
              <p className="text-xs text-slate-500 mt-0.5">
                {config?.enabled ? "已启用，将按间隔自动执行" : "已停用"}
              </p>
            </div>
            <button
              onClick={toggleEnabled}
              disabled={saving}
              className={`relative inline-flex h-7 w-12 items-center rounded-full transition-colors shrink-0 ${
                config?.enabled ? "bg-primary" : "bg-slate-600"
              }`}
            >
              <span
                className={`inline-block h-5 w-5 transform rounded-full bg-white transition-transform ${
                  config?.enabled ? "translate-x-6" : "translate-x-1"
                }`}
              />
            </button>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-1.5">
              扫描间隔（秒）
            </label>
            <div className="flex gap-2">
              <input
                type="number"
                min={10}
                value={intervalDraft}
                onChange={(e) =>
                  setIntervalDraft(Math.max(10, Number(e.target.value)))
                }
                className="input flex-1"
              />
              <button
                onClick={saveInterval}
                disabled={saving || intervalDraft === config?.scanIntervalSec}
                className="btn-primary inline-flex items-center gap-2 shrink-0 disabled:opacity-40"
              >
                {saving ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : null}
                应用
              </button>
            </div>
            <p className="mt-1 text-xs text-slate-500">
              1800 = 30 分钟，600 = 10 分钟。目录大小重命名固定每 3 分钟一次（按轮次节流）。
            </p>
          </div>

          <div className="flex items-center justify-between gap-4 pt-1 border-t border-slate-700/50">
            <div>
              <div className="text-sm text-slate-300">最近运行</div>
              <p className="text-xs text-slate-500 mt-0.5">
                {config?.lastRunAt
                  ? new Date(config.lastRunAt).toLocaleString()
                  : "尚未运行"}
                {config?.lastStatus ? ` · ${config.lastStatus}` : ""}
              </p>
            </div>
            <button
              onClick={trigger}
              disabled={triggering}
              className="btn-secondary inline-flex items-center gap-2 shrink-0"
            >
              {triggering ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <Play className="w-4 h-4" />
              )}
              立即执行
            </button>
          </div>
        </div>
      </section>

      {/* 主目录 */}
      <DirListSection
        title="主目录"
        description="执行 CAS 同步、目录大小重命名、HiveWeb 标签。扫描时会跳过同名的追更子目录。"
        dirs={mainDirs}
        onAdd={() => setPicker({ kind: "main" })}
        onDelete={(id) => handleDeleteDir(id, "main")}
      />

      {/* 追更目录 */}
      <DirListSection
        title="追更目录"
        description="执行 CAS 同步（支持 S01E01 / 纯数字格式）、纯 SxxExx 模板重命名、HiveWeb 标签。"
        dirs={chasingDirs}
        onAdd={() => setPicker({ kind: "chasing" })}
        onDelete={(id) => handleDeleteDir(id, "chasing")}
      />

      {picker && (
        <DirectoryPicker
          currentSelectedPath="/"
          onSelect={(path) => {
            void handleAddDir(path, picker.kind);
            setPicker(null);
          }}
          onClose={() => setPicker(null)}
        />
      )}
    </div>
  );
}

function DirListSection({
  title,
  description,
  dirs,
  onAdd,
  onDelete,
}: {
  title: string;
  description: string;
  dirs: MonitorDir[];
  onAdd: () => void;
  onDelete: (id: number) => void;
}) {
  return (
    <section className="bg-slate-800/50 rounded-xl border border-slate-700/50 overflow-hidden">
      <div className="flex items-center justify-between gap-2.5 px-6 py-4 border-b border-slate-700/50 bg-slate-800/80">
        <div className="flex items-center gap-2.5 min-w-0">
          <FolderPlus className="w-4 h-4 text-primary shrink-0" />
          <h2 className="text-sm font-semibold text-white tracking-wide uppercase truncate">
            {title}
          </h2>
        </div>
        <button
          onClick={onAdd}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-primary/10 text-primary hover:bg-primary/20 border border-primary/30 rounded-lg transition-colors shrink-0"
        >
          <Plus className="w-3.5 h-3.5" />
          添加目录
        </button>
      </div>

      <div className="p-6">
        <p className="text-xs text-slate-500 mb-4">{description}</p>

        {dirs.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-8 text-slate-500">
            <Folder className="w-8 h-8 mb-2 text-slate-600" />
            <span className="text-sm">暂无目录，点击「添加目录」</span>
          </div>
        ) : (
          <ul className="space-y-1.5">
            {dirs.map((d) => (
              <li
                key={d.id}
                className="flex items-center gap-3 px-3 py-2.5 rounded-lg bg-slate-900/50 border border-slate-700/50"
              >
                <Folder className="w-4 h-4 text-slate-500 shrink-0" />
                <span className="flex-1 truncate text-sm text-slate-200">
                  {d.path}
                </span>
                <button
                  onClick={() => onDelete(d.id)}
                  className="shrink-0 p-1.5 text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 rounded transition-colors"
                  title="删除"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
