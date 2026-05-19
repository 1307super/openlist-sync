import { useEffect } from "react";
import { Folder, ChevronRight, ArrowUp, Loader2, Check } from "lucide-react";
import { useBrowse } from "../hooks/useBrowse";

interface DirectoryPickerProps {
  currentSelectedPath: string;
  onSelect: (path: string) => void;
  onClose: () => void;
}

export default function DirectoryPicker({
  currentSelectedPath,
  onSelect,
  onClose,
}: DirectoryPickerProps) {
  const { currentPath, entries, loading, navigate, navigateInto, navigateUp } =
    useBrowse();

  useEffect(() => {
    navigate(currentSelectedPath || "/");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const segments = currentPath.split("/").filter(Boolean);

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="w-full max-w-lg mx-4 bg-slate-900 border border-slate-700 rounded-xl shadow-2xl overflow-hidden flex flex-col max-h-[80vh]">
        <div className="flex items-center justify-between px-5 py-4 border-b border-slate-800">
          <h3 className="text-base font-semibold text-white">
            选择目录
          </h3>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-white transition-colors text-sm"
          >
            取消
          </button>
        </div>

        <div className="flex items-center gap-1 px-5 py-3 border-b border-slate-800 overflow-x-auto text-sm">
          <button
            onClick={() => navigate("/")}
            className="text-slate-400 hover:text-white transition-colors shrink-0"
          >
            /
          </button>
          {segments.map((seg, i) => {
            const path = "/" + segments.slice(0, i + 1).join("/");
            return (
              <span key={path} className="flex items-center gap-1 shrink-0">
                <ChevronRight className="w-3.5 h-3.5 text-slate-600" />
                <button
                  onClick={() => navigate(path)}
                  className={`transition-colors ${
                    i === segments.length - 1
                      ? "text-white font-medium"
                      : "text-slate-400 hover:text-white"
                  }`}
                >
                  {seg}
                </button>
              </span>
            );
          })}
        </div>

        <div className="px-5 pt-3 flex items-center gap-3">
          <button
            onClick={navigateUp}
            disabled={currentPath === "/"}
            className="flex items-center gap-1.5 text-sm text-slate-400 hover:text-white disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            <ArrowUp className="w-4 h-4" />
            上级
          </button>
          <button
            onClick={() => onSelect(currentPath)}
            className="ml-auto flex items-center gap-1.5 text-sm text-primary hover:text-primary-hover transition-colors"
          >
            <Check className="w-4 h-4" />
            选择当前目录
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-2 py-2 min-h-[200px]">
          {loading ? (
            <div className="flex items-center justify-center py-12 text-slate-500">
              <Loader2 className="w-5 h-5 animate-spin" />
              <span className="ml-2 text-sm">加载中...</span>
            </div>
          ) : entries.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-10 text-slate-500">
              <Folder className="w-8 h-8 mb-2 text-slate-600" />
              <span className="text-sm mb-3">当前目录下无子目录</span>
              <button
                onClick={() => onSelect(currentPath)}
                className="px-4 py-1.5 text-sm font-medium bg-primary hover:bg-primary-hover text-white rounded-lg transition-colors"
              >
                选择 {currentPath}
              </button>
            </div>
          ) : (
            <ul className="space-y-0.5">
              {entries.map((entry) => {
                const entryPath =
                  currentPath === "/"
                    ? `/${entry.name}`
                    : `${currentPath}/${entry.name}`;
                return (
                  <li key={entry.name}>
                    <button
                      className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-slate-300 hover:bg-slate-800 hover:text-white transition-colors text-left group"
                    >
                      <Folder className="w-4 h-4 text-slate-500 shrink-0" />
                      <span
                        className="truncate flex-1"
                        onClick={() => navigateInto(entry.name)}
                      >
                        {entry.name}
                      </span>
                      <button
                        onClick={() => onSelect(entryPath)}
                        className="shrink-0 px-2 py-1 text-xs text-slate-400 hover:text-primary border border-slate-700 rounded"
                      >
                        选择
                      </button>
                      <ChevronRight
                        className="w-4 h-4 text-slate-600 shrink-0 cursor-pointer"
                        onClick={() => navigateInto(entry.name)}
                      />
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        <div className="px-5 py-4 border-t border-slate-800 flex items-center gap-3">
          <span className="text-xs text-slate-500 truncate flex-1">
            当前: {currentPath}
          </span>
          <button
            onClick={() => onSelect(currentPath)}
            className="px-4 py-2 text-sm font-medium bg-primary hover:bg-primary-hover text-white rounded-lg transition-colors"
          >
            确认选择
          </button>
        </div>
      </div>
    </div>
  );
}
