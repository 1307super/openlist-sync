import { useState, useEffect, useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { tasksApi } from "../api/client";
import type { SyncTask, CopyJob } from "../types";
import LogViewer from "../components/LogViewer";
import TaskForm from "../components/TaskForm";
import {
  ArrowLeft,
  Play,
  Square,
  RefreshCw,
  Pencil,
  Trash2,
  FolderInput,
  FolderOutput,
  Timer,
  ToggleLeft,
  ToggleRight,
  Clock,
  AlertTriangle,
  CheckCircle2,
  XCircle,
  Copy,
  Loader2,
  CircleDashed,
} from "lucide-react";

const statusBadge: Record<string, string> = {
  idle: "bg-slate-500/20 text-slate-400 border border-slate-500/30",
  running: "bg-blue-500/20 text-blue-400 border border-blue-500/30",
  paused: "bg-yellow-500/20 text-yellow-400 border border-yellow-500/30",
  error: "bg-red-500/20 text-red-400 border border-red-500/30",
};

const statusDot: Record<string, string> = {
  idle: "bg-slate-400",
  running: "bg-blue-400",
  paused: "bg-yellow-400",
  error: "bg-red-400",
};

const jobStatusBadge: Record<string, string> = {
  pending: "bg-slate-500/20 text-slate-400",
  copying: "bg-blue-500/20 text-blue-400",
  completed: "bg-emerald-500/20 text-emerald-400",
  failed: "bg-red-500/20 text-red-400",
};

const statusLabelMap: Record<string, string> = {
  idle: "空闲",
  running: "运行中",
  paused: "已暂停",
  error: "错误",
};

const jobStatusLabelMap: Record<string, string> = {
  pending: "等待中",
  copying: "复制中",
  completed: "已完成",
  failed: "失败",
};

export default function TaskDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const taskId = Number(id);

  const [task, setTask] = useState<SyncTask | null>(null);
  const [jobs, setJobs] = useState<CopyJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [triggering, setTriggering] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [deletingJobId, setDeletingJobId] = useState<number | null>(null);
  const [showEditForm, setShowEditForm] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchTask = useCallback(async () => {
    try {
      const [t, j] = await Promise.all([
        tasksApi.get(taskId),
        tasksApi.jobs(taskId),
      ]);
      setTask(t);
      setJobs(j);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "加载任务失败");
    } finally {
      setLoading(false);
    }
  }, [taskId]);

  useEffect(() => {
    fetchTask();
  }, [fetchTask]);

  const isRunning = task?.status === "running";
  const isActive = task ? task.enabled && task.status !== "paused" : false;
  const hasActiveJobs = jobs.some(
    (j) => j.status === "copying" || j.status === "pending"
  );

  useEffect(() => {
    if (!isRunning && !hasActiveJobs) return;
    const interval = setInterval(fetchTask, 5000);
    return () => clearInterval(interval);
  }, [isRunning, hasActiveJobs, fetchTask]);

  const handleStartStop = async () => {
    if (!task) return;
    try {
      if (isActive) {
        const updated = await tasksApi.stop(taskId);
        setTask(updated);
      } else {
        const updated = await tasksApi.start(taskId);
        setTask(updated);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作失败");
    }
  };

  const handleTrigger = async () => {
    setTriggering(true);
    try {
      await tasksApi.trigger(taskId);
      await fetchTask();
    } catch (e) {
      setError(e instanceof Error ? e.message : "触发失败");
    } finally {
      setTriggering(false);
    }
  };

  const handleDelete = async () => {
    try {
      await tasksApi.delete(taskId);
      navigate("/");
    } catch (e) {
      setError(e instanceof Error ? e.message : "删除失败");
    }
  };

  const handleDeleteJob = async (job: CopyJob) => {
    const ok = window.confirm(
      `删除本地复制记录「${job.fileName}」？这不会删除 OpenList 中的任务，只会释放本项目的 pending 占用。`
    );
    if (!ok) return;

    setDeletingJobId(job.id);
    try {
      await tasksApi.deleteJob(taskId, job.id);
      await fetchTask();
    } catch (e) {
      setError(e instanceof Error ? e.message : "删除复制记录失败");
    } finally {
      setDeletingJobId(null);
    }
  };

  const handleEdit = async (data: Partial<SyncTask>) => {
    try {
      const updated = await tasksApi.update(taskId, data);
      setTask(updated);
      setShowEditForm(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "更新失败");
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Loader2 className="w-8 h-8 text-primary animate-spin" />
      </div>
    );
  }

  if (!task) {
    return (
      <div className="p-6">
        <button
          onClick={() => navigate("/")}
          className="flex items-center gap-2 text-slate-400 hover:text-white transition-colors mb-6"
        >
          <ArrowLeft className="w-4 h-4" />
          返回任务列表
        </button>
        <div className="text-red-400">{error || "任务未找到"}</div>
      </div>
    );
  }

  const formatRelative = (iso: string | null) => {
    if (!iso) return "—";
    const d = new Date(iso);
    const now = new Date();
    const diffMs = now.getTime() - d.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 1) return "刚刚";
    if (diffMin < 60) return `${diffMin}分钟前`;
    const diffHr = Math.floor(diffMin / 60);
    if (diffHr < 24) return `${diffHr}小时前`;
    return d.toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  return (
    <div className="p-6 space-y-6 max-w-6xl mx-auto">
      {error && (
        <div className="flex items-center gap-2 px-4 py-3 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm">
          <AlertTriangle className="w-4 h-4 shrink-0" />
          {error}
        </div>
      )}

      <div className="flex items-center gap-4">
        <button
          onClick={() => navigate("/")}
          className="p-2 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
        >
          <ArrowLeft className="w-5 h-5" />
        </button>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-3 flex-wrap">
            <h1 className="text-2xl font-semibold text-white truncate">
              {task.name}
            </h1>
            <span
              className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold uppercase tracking-wide ${
                statusBadge[task.status]
              }`}
            >
              <span
                className={`w-1.5 h-1.5 rounded-full ${
                  task.status === "running" ? "animate-pulse" : ""
                } ${statusDot[task.status]}`}
              />
              {statusLabelMap[task.status] || task.status}
            </span>
          </div>
        </div>
      </div>

      {showEditForm && task && (
        <TaskForm
          mode="edit"
          task={task}
          onSubmit={handleEdit}
          onCancel={() => setShowEditForm(false)}
        />
      )}

      <div className="flex items-center gap-2 flex-wrap">
        <button
          onClick={handleStartStop}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            isActive
              ? "bg-yellow-500/10 text-yellow-400 hover:bg-yellow-500/20 border border-yellow-500/20"
              : "bg-emerald-500/10 text-emerald-400 hover:bg-emerald-500/20 border border-emerald-500/20"
          }`}
        >
          {isActive ? (
            <>
              <Square className="w-4 h-4" />
              <span className="hidden sm:inline">暂停</span>
            </>
          ) : (
            <>
              <Play className="w-4 h-4" />
              <span className="hidden sm:inline">启动</span>
            </>
          )}
        </button>

        <button
          onClick={handleTrigger}
          disabled={triggering}
          className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium bg-primary/10 text-primary hover:bg-primary/20 border border-primary/20 transition-colors disabled:opacity-50"
        >
          <RefreshCw
            className={`w-4 h-4 ${triggering ? "animate-spin" : ""}`}
          />
          <span className="hidden sm:inline">
            {triggering ? "同步中..." : "立即同步"}
          </span>
        </button>

        <button
          onClick={() => setShowEditForm(true)}
          className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium bg-slate-800 text-slate-300 hover:bg-slate-700 border border-slate-700 transition-colors"
        >
          <Pencil className="w-4 h-4" />
          <span className="hidden sm:inline">编辑</span>
        </button>

        <button
          onClick={() => setShowDeleteConfirm(true)}
          className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium bg-red-500/10 text-red-400 hover:bg-red-500/20 border border-red-500/20 transition-colors"
        >
          <Trash2 className="w-4 h-4" />
          <span className="hidden sm:inline">删除</span>
        </button>
      </div>

      {showDeleteConfirm && (
        <div className="flex items-center gap-3 px-4 py-3 rounded-lg bg-red-500/10 border border-red-500/20">
          <p className="text-sm text-red-300 flex-1">
            确定要删除任务{" "}
            <span className="font-semibold">{task.name}</span> 吗？此操作不可撤销。
          </p>
          <button
            onClick={handleDelete}
            className="px-3 py-1.5 rounded-md text-sm font-medium bg-red-500 text-white hover:bg-red-600 transition-colors"
          >
            删除
          </button>
          <button
            onClick={() => setShowDeleteConfirm(false)}
            className="px-3 py-1.5 rounded-md text-sm font-medium bg-slate-700 text-slate-300 hover:bg-slate-600 transition-colors"
          >
            取消
          </button>
        </div>
      )}

      <div className="bg-slate-800/50 rounded-xl border border-slate-700/50 p-5">
        <h2 className="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-4">
          配置信息
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-4">
          <ConfigItem
            icon={<FolderInput className="w-4 h-4" />}
            label="源路径"
            value={task.sourcePath}
          />
          <ConfigItem
            icon={<FolderOutput className="w-4 h-4" />}
            label="目标路径"
            value={task.destPath}
          />
          <ConfigItem
            icon={
              task.completionRule === "delete_source" ? (
                <Trash2 className="w-4 h-4" />
              ) : (
                <CheckCircle2 className="w-4 h-4" />
              )
            }
            label="完成规则"
            value={
              task.completionRule === "delete_source"
                ? "删除源文件"
                : "保留文件"
            }
          />
          <ConfigItem
            icon={
              task.replaceRule === "overwrite" ? (
                <Copy className="w-4 h-4" />
              ) : (
                <CircleDashed className="w-4 h-4" />
              )
            }
            label="替换规则"
            value={
              task.replaceRule === "overwrite" ? "覆盖已存在" : "跳过已存在"
            }
          />
          <ConfigItem
            icon={<Timer className="w-4 h-4" />}
            label="扫描间隔"
            value={`${task.scanIntervalSec}秒`}
          />
          <ConfigItem
            icon={
              task.enabled ? (
                <ToggleRight className="w-4 h-4 text-emerald-400" />
              ) : (
                <ToggleLeft className="w-4 h-4 text-slate-500" />
              )
            }
            label="启用状态"
            value={task.enabled ? "已启用" : "已禁用"}
          />
          <ConfigItem
            icon={<Clock className="w-4 h-4" />}
            label="上次同步"
            value={formatRelative(task.lastSyncAt)}
          />
          {task.error && (
            <ConfigItem
              icon={<AlertTriangle className="w-4 h-4 text-red-400" />}
              label="错误信息"
              value={task.error}
              valueClass="text-red-400"
            />
          )}
        </div>
      </div>

      <div className="space-y-3">
        <div className="flex items-center gap-3 flex-wrap">
          <h2 className="text-lg font-semibold text-white">复制记录</h2>
          <span className="text-xs font-medium text-slate-500 bg-slate-800 px-2 py-0.5 rounded-full">
            {jobs.length}
          </span>
          {jobs.some((job) => job.status === "pending") && (
            <span className="text-xs font-medium text-amber-300 bg-amber-500/10 border border-amber-500/20 px-2 py-0.5 rounded-full">
              pending {jobs.filter((job) => job.status === "pending").length}
            </span>
          )}
        </div>

        {jobs.length === 0 ? (
          <div className="flex items-center justify-center py-12 bg-slate-800/30 rounded-xl border border-slate-800 text-slate-500 text-sm">
            暂无复制记录
          </div>
        ) : (
          <div className="overflow-x-auto rounded-xl border border-slate-700/50">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-slate-800/80 text-slate-400 text-left">
                  <th className="px-4 py-3 font-medium">文件名</th>
                  <th className="px-4 py-3 font-medium">状态</th>
                  <th className="px-4 py-3 font-medium">重试</th>
                  <th className="px-4 py-3 font-medium">错误</th>
                  <th className="px-4 py-3 font-medium">创建时间</th>
                  <th className="px-4 py-3 font-medium">完成时间</th>
                  <th className="px-4 py-3 font-medium text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/50">
                {jobs.map((job, i) => (
                  <tr
                    key={job.id}
                    className={
                      i % 2 === 1
                        ? "bg-slate-800/20"
                        : "bg-transparent"
                    }
                  >
                    <td className="px-4 py-3">
                      <span
                        className="text-slate-200 max-w-[200px] truncate block"
                        title={job.fileName}
                      >
                        {job.fileName}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium ${
                          jobStatusBadge[job.status]
                        }`}
                      >
                        {job.status === "copying" && (
                          <span className="w-1.5 h-1.5 rounded-full bg-blue-400 animate-pulse" />
                        )}
                        {jobStatusLabelMap[job.status] || job.status}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-slate-400">
                      {job.retryCount}
                    </td>
                    <td className="px-4 py-3">
                      {job.error ? (
                        <span
                          className="text-red-400 truncate block max-w-[200px]"
                          title={job.error}
                        >
                          {job.error}
                        </span>
                      ) : (
                        <span className="text-slate-600">—</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-slate-400 whitespace-nowrap">
                      {formatRelative(job.createdAt)}
                    </td>
                    <td className="px-4 py-3 text-slate-400 whitespace-nowrap">
                      {formatRelative(job.completedAt)}
                    </td>
                    <td className="px-4 py-3 text-right">
                      {(job.status === "pending" || job.status === "failed") ? (
                        <button
                          onClick={() => handleDeleteJob(job)}
                          disabled={deletingJobId === job.id}
                          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-red-500/10 text-red-400 hover:bg-red-500/20 border border-red-500/20 transition-colors disabled:opacity-50"
                          title="删除本地复制记录，释放 pending 占用"
                        >
                          {deletingJobId === job.id ? (
                            <Loader2 className="w-3.5 h-3.5 animate-spin" />
                          ) : (
                            <Trash2 className="w-3.5 h-3.5" />
                          )}
                          删除
                        </button>
                      ) : (
                        <span className="text-slate-600">—</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="space-y-3">
        <h2 className="text-lg font-semibold text-white">日志</h2>
        <LogViewer taskId={taskId} isRunning={isRunning} />
      </div>
    </div>
  );
}

function ConfigItem({
  icon,
  label,
  value,
  valueClass = "text-slate-200",
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  valueClass?: string;
}) {
  return (
    <div className="flex items-start gap-3">
      <span className="text-slate-500 mt-0.5 shrink-0">{icon}</span>
      <div className="min-w-0">
        <div className="text-xs text-slate-500 mb-0.5">{label}</div>
        <div
          className={`text-sm font-medium break-all ${valueClass}`}
        >
          {value}
        </div>
      </div>
    </div>
  );
}
