import { useState } from "react";
import { FolderInput, FolderOutput } from "lucide-react";
import type { SyncTask } from "../types";
import DirectoryPicker from "./DirectoryPicker";

interface TaskFormProps {
  mode: "create" | "edit";
  task?: SyncTask;
  onSubmit: (data: Partial<SyncTask>) => void;
  onCancel: () => void;
}

export default function TaskForm({
  mode,
  task,
  onSubmit,
  onCancel,
}: TaskFormProps) {
  const [name, setName] = useState(task?.name ?? "");
  const [sourcePath, setSourcePath] = useState(task?.sourcePath ?? "");
  const [destPath, setDestPath] = useState(task?.destPath ?? "");
  const [completionRule, setCompletionRule] = useState<
    SyncTask["completionRule"]
  >(task?.completionRule ?? "keep");
  const [replaceRule, setReplaceRule] = useState<SyncTask["replaceRule"]>(
    task?.replaceRule ?? "skip"
  );
  const [matchMode, setMatchMode] = useState<SyncTask["matchMode"]>(
    task?.matchMode ?? "exact"
  );
  const [scanIntervalSec, setScanIntervalSec] = useState(
    task?.scanIntervalSec ?? 300
  );
  const [deleteEmptyDirs, setDeleteEmptyDirs] = useState(
    task?.deleteEmptyDirs ?? false
  );
  const [pickerTarget, setPickerTarget] = useState<
    "source" | "dest" | null
  >(null);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const validate = () => {
    const e: Record<string, string> = {};
    if (!name.trim()) e.name = "请输入任务名称";
    if (!sourcePath.trim()) e.sourcePath = "请输入源路径";
    if (!destPath.trim()) e.destPath = "请输入目标路径";
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleSubmit = () => {
    if (!validate()) return;
    onSubmit({
      name: name.trim(),
      sourcePath: sourcePath.trim(),
      destPath: destPath.trim(),
      completionRule,
      replaceRule,
      matchMode,
      scanIntervalSec,
      deleteEmptyDirs,
      enabled: task?.enabled ?? true,
    });
  };

  const handlePathSelect = (path: string) => {
    if (pickerTarget === "source") setSourcePath(path);
    else if (pickerTarget === "dest") setDestPath(path);
    setPickerTarget(null);
  };

  const inputClass =
    "w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-sm text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary transition-colors";

  const radioCard =
    "flex items-center gap-2 px-3 py-2.5 rounded-lg border cursor-pointer transition-colors text-sm";

  return (
    <>
      {pickerTarget && (
        <DirectoryPicker
          currentSelectedPath={
            pickerTarget === "source" ? sourcePath : destPath
          }
          onSelect={handlePathSelect}
          onClose={() => setPickerTarget(null)}
        />
      )}

      <div
        className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
        onClick={(e) => {
          if (e.target === e.currentTarget) onCancel();
        }}
      >
        <div className="w-full max-w-lg bg-slate-900 border border-slate-700 rounded-xl shadow-2xl flex flex-col max-h-[calc(100vh-2rem)]">
          <div className="px-6 py-4 border-b border-slate-800 shrink-0">
            <h3 className="text-base font-semibold text-white">
              {mode === "create" ? "创建同步任务" : "编辑同步任务"}
            </h3>
          </div>

          <div className="overflow-y-auto flex-1 min-h-0 p-6 space-y-5">
            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1.5">
                任务名称
              </label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="例如：电影同步"
                className={inputClass}
              />
              {errors.name && (
                <p className="mt-1 text-xs text-red-400">{errors.name}</p>
              )}
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1.5">
                源路径
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={sourcePath}
                  onChange={(e) => setSourcePath(e.target.value)}
                  placeholder="/源目录"
                  className={`${inputClass} flex-1`}
                />
                <button
                  type="button"
                  onClick={() => setPickerTarget("source")}
                  className="px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-400 hover:text-white hover:border-slate-600 transition-colors"
                  title="浏览源路径"
                >
                  <FolderInput className="w-4 h-4" />
                </button>
              </div>
              {errors.sourcePath && (
                <p className="mt-1 text-xs text-red-400">{errors.sourcePath}</p>
              )}
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1.5">
                目标路径
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={destPath}
                  onChange={(e) => setDestPath(e.target.value)}
                  placeholder="/目标目录"
                  className={`${inputClass} flex-1`}
                />
                <button
                  type="button"
                  onClick={() => setPickerTarget("dest")}
                  className="px-3 py-2 bg-slate-800 border border-slate-700 rounded-lg text-slate-400 hover:text-white hover:border-slate-600 transition-colors"
                  title="浏览目标路径"
                >
                  <FolderOutput className="w-4 h-4" />
                </button>
              </div>
              {errors.destPath && (
                <p className="mt-1 text-xs text-red-400">{errors.destPath}</p>
              )}
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-300 mb-2">
                完成规则
              </label>
              <div className="grid grid-cols-2 gap-2">
                <label
                  className={`${radioCard} ${
                    completionRule === "keep"
                      ? "border-primary bg-primary/10 text-white"
                      : "border-slate-700 text-slate-400 hover:border-slate-600"
                  }`}
                >
                  <input
                    type="radio"
                    name="completionRule"
                    value="keep"
                    checked={completionRule === "keep"}
                    onChange={() => setCompletionRule("keep")}
                    className="sr-only"
                  />
                  保留文件
                </label>
                <label
                  className={`${radioCard} ${
                    completionRule === "delete_source"
                      ? "border-primary bg-primary/10 text-white"
                      : "border-slate-700 text-slate-400 hover:border-slate-600"
                  }`}
                >
                  <input
                    type="radio"
                    name="completionRule"
                    value="delete_source"
                    checked={completionRule === "delete_source"}
                    onChange={() => setCompletionRule("delete_source")}
                    className="sr-only"
                  />
                  删除源文件
                </label>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-300 mb-2">
                替换规则
              </label>
              <div className="grid grid-cols-2 gap-2">
                <label
                  className={`${radioCard} ${
                    replaceRule === "skip"
                      ? "border-primary bg-primary/10 text-white"
                      : "border-slate-700 text-slate-400 hover:border-slate-600"
                  }`}
                >
                  <input
                    type="radio"
                    name="replaceRule"
                    value="skip"
                    checked={replaceRule === "skip"}
                    onChange={() => setReplaceRule("skip")}
                    className="sr-only"
                  />
                  跳过已存在文件
                </label>
                <label
                  className={`${radioCard} ${
                    replaceRule === "overwrite"
                      ? "border-primary bg-primary/10 text-white"
                      : "border-slate-700 text-slate-400 hover:border-slate-600"
                  }`}
                >
                  <input
                    type="radio"
                    name="replaceRule"
                    value="overwrite"
                    checked={replaceRule === "overwrite"}
                    onChange={() => setReplaceRule("overwrite")}
                    className="sr-only"
                  />
                  覆盖已存在文件
                </label>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-slate-300 mb-2">
                匹配模式
              </label>
              <div className="grid grid-cols-2 gap-2">
                <label
                  className={`${radioCard} ${
                    matchMode === "exact"
                      ? "border-primary bg-primary/10 text-white"
                      : "border-slate-700 text-slate-400 hover:border-slate-600"
                  }`}
                >
                  <input
                    type="radio"
                    name="matchMode"
                    value="exact"
                    checked={matchMode === "exact"}
                    onChange={() => setMatchMode("exact")}
                    className="sr-only"
                  />
                  精确匹配
                </label>
                <label
                  className={`${radioCard} ${
                    matchMode === "smart"
                      ? "border-primary bg-primary/10 text-white"
                      : "border-slate-700 text-slate-400 hover:border-slate-600"
                  }`}
                >
                  <input
                    type="radio"
                    name="matchMode"
                    value="smart"
                    checked={matchMode === "smart"}
                    onChange={() => setMatchMode("smart")}
                    className="sr-only"
                  />
                  追更匹配
                </label>
              </div>
              <p className="mt-1.5 text-xs text-slate-500">
                {matchMode === "smart"
                  ? "源文件名为 S01E01.mkv 格式时，通过剧集编号匹配目标（如 adb.S01E01.1080P.mkv）"
                  : "仅通过完整文件名匹配"}
              </p>
            </div>

            <label className="flex items-center gap-3 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={deleteEmptyDirs}
                onChange={(e) => setDeleteEmptyDirs(e.target.checked)}
                className="w-4 h-4 rounded border-slate-600 bg-slate-800 text-primary focus:ring-primary/50 focus:ring-offset-0"
              />
              <div>
                <span className="text-sm text-slate-300">删除空目录</span>
                <p className="text-xs text-slate-500 mt-0.5">同步后递归删除源目录下的空子目录；追更模式下源目录本身为空时也会删除，精确模式下不删除源目录本身</p>
              </div>
            </label>

            <div>
              <label className="block text-sm font-medium text-slate-300 mb-1.5">
                扫描间隔（秒）
              </label>
              <input
                type="number"
                value={scanIntervalSec}
                onChange={(e) =>
                  setScanIntervalSec(Math.max(10, Number(e.target.value)))
                }
                min={10}
                className={inputClass}
              />
              <p className="mt-1 text-xs text-slate-500">
                300 = 5分钟，60 = 1分钟
              </p>
            </div>
          </div>

          <div className="px-6 py-4 border-t border-slate-800 shrink-0 flex items-center justify-end gap-3">
            <button
              type="button"
              onClick={onCancel}
              className="px-4 py-2 text-sm font-medium text-slate-400 hover:text-white transition-colors"
            >
              取消
            </button>
            <button
              type="button"
              onClick={handleSubmit}
              className="px-5 py-2 text-sm font-medium bg-primary hover:bg-primary-hover text-white rounded-lg transition-colors"
            >
              {mode === "create" ? "创建任务" : "保存修改"}
            </button>
          </div>
        </div>
      </div>
    </>
  );
}
