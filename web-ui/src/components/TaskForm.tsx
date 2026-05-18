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
  const [scanIntervalSec, setScanIntervalSec] = useState(
    task?.scanIntervalSec ?? 300
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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    onSubmit({
      name: name.trim(),
      sourcePath: sourcePath.trim(),
      destPath: destPath.trim(),
      completionRule,
      replaceRule,
      scanIntervalSec,
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
        className="fixed inset-0 z-40 flex items-center justify-center bg-black/60 backdrop-blur-sm"
        onClick={(e) => {
          if (e.target === e.currentTarget) onCancel();
        }}
      >
        <div className="w-full max-w-lg mx-4 bg-slate-900 border border-slate-700 rounded-xl shadow-2xl overflow-hidden">
          <div className="px-6 py-4 border-b border-slate-800">
            <h3 className="text-base font-semibold text-white">
              {mode === "create" ? "创建同步任务" : "编辑同步任务"}
            </h3>
          </div>

          <form onSubmit={handleSubmit} className="p-6 space-y-5">
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

            <div className="flex items-center justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={onCancel}
                className="px-4 py-2 text-sm font-medium text-slate-400 hover:text-white transition-colors"
              >
                取消
              </button>
              <button
                type="submit"
                className="px-5 py-2 text-sm font-medium bg-primary hover:bg-primary-hover text-white rounded-lg transition-colors"
              >
                {mode === "create" ? "创建任务" : "保存修改"}
              </button>
            </div>
          </form>
        </div>
      </div>
    </>
  );
}
