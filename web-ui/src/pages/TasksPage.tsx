import { useState, useEffect, useRef, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { Plus, Loader2, FolderSync } from "lucide-react";
import { useTasks } from "../hooks/useTasks";
import { useOpenListCopyTasks } from "../hooks/useOpenListCopyTasks";
import TaskCard from "../components/TaskCard";
import TaskForm from "../components/TaskForm";
import GlobalProgress from "../components/GlobalProgress";
import type { SyncTask } from "../types";

export default function TasksPage() {
  const navigate = useNavigate();
  const {
    tasks,
    loading,
    fetchTasks,
    createTask,
    updateTask,
    deleteTask,
    startTask,
    stopTask,
    triggerTask,
  } = useTasks();

  const [showForm, setShowForm] = useState(false);
  const [editingTask, setEditingTask] = useState<SyncTask | null>(null);
  const [keyword, setKeyword] = useState("");
  const [toast, setToast] = useState<string | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

  const showToast = useCallback((msg: string) => {
    setToast(msg);
    clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToast(null), 2500);
  }, []);

  const hasRunning = tasks.some((t) => t.status === "running");
  const copyTasks = useOpenListCopyTasks();

  const filteredTasks = keyword.trim()
    ? tasks.filter((t) => {
        const kw = keyword.toLowerCase();
        return (
          t.name.toLowerCase().includes(kw) ||
          t.sourcePath.toLowerCase().includes(kw) ||
          t.destPath.toLowerCase().includes(kw)
        );
      })
    : tasks;

  useEffect(() => {
    if (!hasRunning) return;
    const id = setInterval(fetchTasks, 10000);
    return () => clearInterval(id);
  }, [hasRunning, fetchTasks]);

  const handleCreate = async (data: Partial<SyncTask>) => {
    const task = await createTask(data);
    setShowForm(false);
    if (task.enabled) {
      triggerTask(task.id);
      showToast("任务已创建，同步已触发");
    }
  };

  const handleEdit = async (data: Partial<SyncTask>) => {
    if (editingTask) {
      await updateTask(editingTask.id, data);
      setEditingTask(null);
    }
  };

  const handleStart = async (id: number) => {
    await startTask(id);
    fetchTasks();
  };

  const handleStop = async (id: number) => {
    await stopTask(id);
    fetchTasks();
  };

  const handleDelete = async (id: number) => {
    if (confirm("确定要删除此任务吗？此操作不可撤销。")) {
      await deleteTask(id);
    }
  };

  const handleTrigger = async (id: number) => {
    await triggerTask(id);
    showToast("同步已触发");
    fetchTasks();
  };

  return (
    <div className="p-4 md:p-6 lg:p-8 max-w-7xl mx-auto">
      {(showForm || editingTask) && (
        <TaskForm
          mode={editingTask ? "edit" : "create"}
          task={editingTask ?? undefined}
          onSubmit={editingTask ? handleEdit : handleCreate}
          onCancel={() => {
            setShowForm(false);
            setEditingTask(null);
          }}
        />
      )}

      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl md:text-2xl font-semibold text-white">
          同步任务
        </h1>
        <button
          onClick={() => setShowForm(true)}
          className="flex items-center gap-2 px-4 py-2 text-sm font-medium bg-primary hover:bg-primary-hover text-white rounded-lg transition-colors"
        >
          <Plus className="w-4 h-4" />
          创建任务
        </button>
      </div>

      {tasks.length > 0 && (
        <div className="mb-4">
          <input
            type="text"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="搜索任务名称或路径..."
            className="input"
          />
        </div>
      )}

      {copyTasks.length > 0 && <GlobalProgress tasks={copyTasks} />}

      {loading ? (
        <div className="flex items-center justify-center py-20 text-slate-500">
          <Loader2 className="w-6 h-6 animate-spin" />
          <span className="ml-3">加载任务中...</span>
        </div>
      ) : filteredTasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-slate-500">
          {keyword.trim() ? (
            <>
              <p className="text-sm">没有匹配「{keyword.trim()}」的任务</p>
              <button
                onClick={() => setKeyword("")}
                className="mt-3 text-sm text-primary hover:underline"
              >
                清除筛选
              </button>
            </>
          ) : (
            <>
              <FolderSync className="w-12 h-12 mb-4 text-slate-600" />
              <p className="text-sm mb-4">暂无同步任务</p>
              <button
                onClick={() => setShowForm(true)}
                className="flex items-center gap-2 px-4 py-2 text-sm font-medium bg-primary hover:bg-primary-hover text-white rounded-lg transition-colors"
              >
                <Plus className="w-4 h-4" />
                创建第一个任务
              </button>
            </>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {filteredTasks.map((task) => (
            <TaskCard
              key={task.id}
              task={task}
              onStart={handleStart}
              onStop={handleStop}
              onTrigger={handleTrigger}
              onEdit={setEditingTask}
              onDelete={handleDelete}
              onOpen={(id) => navigate(`/tasks/${id}`)}
            />
          ))}
        </div>
      )}

      {toast && (
        <div className="fixed bottom-24 md:bottom-6 left-1/2 -translate-x-1/2 z-50 px-4 py-2.5 bg-slate-800 border border-slate-700 rounded-lg text-sm text-white shadow-xl animate-[fadeInUp_0.2s_ease-out]">
          {toast}
        </div>
      )}
    </div>
  );
}
