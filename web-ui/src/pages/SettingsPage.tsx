import { useState, useEffect } from "react";
import { useSettings } from "../hooks/useSettings";
import { Loader2, CheckCircle2, XCircle, Server, Bot, Save, Shield } from "lucide-react";

export default function SettingsPage() {
  const {
    settings,
    loading,
    saving,
    testResult,
    testing,
    updateSettings,
    testConnection,
  } = useSettings();

  const [form, setForm] = useState({
    openlist_base_url: "",
    openlist_token: "",
    tg_bot_token: "",
    tg_chat_id: "",
    auth_username: "",
    auth_password: "",
  });

  useEffect(() => {
    if (settings) {
      setForm({
        openlist_base_url: settings.openlist_base_url ?? "",
        openlist_token: settings.openlist_token ?? "",
        tg_bot_token: settings.tg_bot_token ?? "",
        tg_chat_id: settings.tg_chat_id ?? "",
        auth_username: settings.auth_username ?? "",
        auth_password: "",
      });
    }
  }, [settings]);

  const handleSave = () => {
    const data: Record<string, string> = { ...form };
    if (!data.auth_password) delete data.auth_password;
    updateSettings(data);
  };
  const handleTest = () => {
    const { auth_username, auth_password, ...connSettings } = form;
    testConnection(connSettings);
  };

  const set = (key: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((prev) => ({ ...prev, [key]: e.target.value }));

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto p-6 md:p-10 space-y-8">
      <div>
        <h1 className="text-2xl font-semibold text-white tracking-tight">
          设置
        </h1>
        <p className="mt-1 text-sm text-slate-400">
          配置 OpenList 连接和集成。
        </p>
      </div>

      <section className="bg-slate-800/50 rounded-xl border border-slate-700/50 overflow-hidden">
        <div className="flex items-center gap-2.5 px-6 py-4 border-b border-slate-700/50 bg-slate-800/80">
          <Server className="w-4 h-4 text-primary" />
          <h2 className="text-sm font-semibold text-white tracking-wide uppercase">
            OpenList 连接
          </h2>
        </div>

        <div className="p-6 space-y-5">
          <Field label="服务器地址" htmlFor="baseUrl">
            <input
              id="baseUrl"
              type="url"
              className="input"
              placeholder="http://192.168.1.100:5244"
              value={form.openlist_base_url}
              onChange={set("openlist_base_url")}
            />
          </Field>

          <Field label="OpenList 令牌" htmlFor="token">
            <input
              id="token"
              type="password"
              className="input"
              placeholder="alist-xxxxxxxx"
              value={form.openlist_token}
              onChange={set("openlist_token")}
            />
          </Field>

          <div className="flex flex-col sm:flex-row items-start sm:items-center gap-3 pt-1">
            <button
              onClick={handleTest}
              disabled={testing || saving}
              className="btn-secondary inline-flex items-center gap-2"
            >
              {testing ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
              )}
              测试连接
            </button>

            {testResult && (
              <div
                className={`flex items-center gap-1.5 text-sm font-medium animate-in ${
                  testResult.success ? "text-emerald-400" : "text-rose-400"
                }`}
              >
                {testResult.success ? (
                  <CheckCircle2 className="w-4 h-4" />
                ) : (
                  <XCircle className="w-4 h-4" />
                )}
                {testResult.success ? "连接成功" : testResult.message}
              </div>
            )}
          </div>
        </div>
      </section>

      <section className="bg-slate-800/50 rounded-xl border border-slate-700/50 overflow-hidden">
        <div className="flex items-center gap-2.5 px-6 py-4 border-b border-slate-700/50 bg-slate-800/80">
          <Bot className="w-4 h-4 text-primary" />
          <h2 className="text-sm font-semibold text-white tracking-wide uppercase">
            Telegram 机器人
          </h2>
        </div>

        <div className="p-6 space-y-5">
          <Field label="机器人 Token" htmlFor="botToken">
            <input
              id="botToken"
              type="password"
              className="input"
              placeholder="123456:ABC-DEF..."
              value={form.tg_bot_token}
              onChange={set("tg_bot_token")}
            />
          </Field>

          <Field label="Chat ID" htmlFor="chatId">
            <input
              id="chatId"
              type="text"
              className="input"
              placeholder="123456789"
              value={form.tg_chat_id}
              onChange={set("tg_chat_id")}
            />
          </Field>
          <p className="text-xs text-slate-500 -mt-2">
            仅该 Chat ID 可操作机器人，留空则不限制。保存后自动生效。
          </p>
        </div>
      </section>

      <section className="bg-slate-800/50 rounded-xl border border-slate-700/50 overflow-hidden">
        <div className="flex items-center gap-2.5 px-6 py-4 border-b border-slate-700/50 bg-slate-800/80">
          <Shield className="w-4 h-4 text-primary" />
          <h2 className="text-sm font-semibold text-white tracking-wide uppercase">
            账户安全
          </h2>
        </div>

        <div className="p-6 space-y-5">
          <Field label="用户名" htmlFor="authUsername">
            <input
              id="authUsername"
              type="text"
              className="input"
              value={form.auth_username}
              onChange={set("auth_username")}
            />
          </Field>

          <Field label="新密码" htmlFor="authPassword">
            <input
              id="authPassword"
              type="password"
              className="input"
              placeholder="留空则不修改"
              value={form.auth_password}
              onChange={set("auth_password")}
            />
          </Field>
          <p className="text-xs text-slate-500 -mt-2">
            修改密码后需重新登录。默认用户名和密码均为 admin。
          </p>
        </div>
      </section>

      <div className="flex justify-end pt-2">
        <button
          onClick={handleSave}
          disabled={saving || testing}
          className="btn-primary inline-flex items-center gap-2 min-w-[140px] justify-center"
        >
          {saving ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : (
            <Save className="w-4 h-4" />
          )}
          {saving ? "保存中..." : "保存设置"}
        </button>
      </div>
    </div>
  );
}

function Field({
  label,
  htmlFor,
  children,
}: {
  label: string;
  htmlFor: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1.5">
      <label
        htmlFor={htmlFor}
        className="block text-sm font-medium text-slate-300"
      >
        {label}
      </label>
      {children}
    </div>
  );
}
