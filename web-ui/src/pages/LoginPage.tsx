import { useState } from "react";
import { FolderSync } from "lucide-react";

export default function LoginPage({
  onLogin,
}: {
  onLogin: (token: string) => void;
}) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.message || "登录失败");
        return;
      }
      onLogin(data.token);
    } catch {
      setError("网络错误");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-slate-950 px-4">
      <div className="w-full max-w-sm">
        <div className="flex flex-col items-center mb-8">
          <FolderSync className="w-12 h-12 text-primary mb-3" />
          <h1 className="text-xl font-semibold text-white">OpenList 同步</h1>
          <p className="text-sm text-slate-500 mt-1">请登录以继续</p>
        </div>

        <form
          onSubmit={handleSubmit}
          className="bg-slate-900 rounded-lg border border-slate-800 p-6 space-y-4"
        >
          {error && (
            <div className="text-sm text-red-400 bg-red-500/10 rounded px-3 py-2">
              {error}
            </div>
          )}

          <div>
            <label className="block text-sm text-slate-400 mb-1.5">
              用户名
            </label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="input"
              placeholder="admin"
              autoFocus
              required
            />
          </div>

          <div>
            <label className="block text-sm text-slate-400 mb-1.5">密码</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="input"
              placeholder="••••••"
              required
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="btn-primary w-full py-2.5"
          >
            {loading ? "登录中..." : "登录"}
          </button>
        </form>
      </div>
    </div>
  );
}
