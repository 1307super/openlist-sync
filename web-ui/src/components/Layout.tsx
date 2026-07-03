import { NavLink, Outlet } from "react-router-dom";
import { FolderSync, ListTodo, Settings, LogOut, Wand2 } from "lucide-react";

const navItems = [
  { to: "/", icon: ListTodo, label: "任务管理" },
  { to: "/monitor", icon: Wand2, label: "监控处理" },
  { to: "/settings", icon: Settings, label: "设置" },
];

function Sidebar({ onLogout }: { onLogout: () => void }) {
  return (
    <aside className="hidden md:flex md:flex-col md:w-64 md:shrink-0 bg-slate-900 border-r border-slate-800">
      <div className="flex items-center gap-3 px-6 py-5 border-b border-slate-800">
        <FolderSync className="w-7 h-7 text-primary" />
        <h1 className="text-lg font-semibold tracking-tight text-white">
          OpenList 同步
        </h1>
      </div>
      <nav className="flex flex-col gap-1 p-3">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            className={({ isActive }) =>
              `flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                isActive
                  ? "bg-slate-800 text-white"
                  : "text-slate-400 hover:bg-slate-800/50 hover:text-slate-200"
              }`
            }
          >
            <Icon className="w-5 h-5" />
            {label}
          </NavLink>
        ))}
      </nav>
      <div className="mt-auto p-3 border-t border-slate-800">
        <button
          onClick={onLogout}
          className="flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm font-medium text-slate-500 hover:bg-slate-800/50 hover:text-slate-300 transition-colors w-full"
        >
          <LogOut className="w-5 h-5" />
          退出登录
        </button>
      </div>
    </aside>
  );
}

function BottomNav() {
  return (
    <nav className="md:hidden fixed bottom-0 inset-x-0 z-50 bg-slate-900 border-t border-slate-800">
      <div className="flex justify-around py-2">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            className={({ isActive }) =>
              `flex flex-col items-center gap-1 px-4 py-1.5 text-xs font-medium transition-colors ${
                isActive
                  ? "text-primary"
                  : "text-slate-500 hover:text-slate-300"
              }`
            }
          >
            <Icon className="w-5 h-5" />
            {label}
          </NavLink>
        ))}
      </div>
    </nav>
  );
}

export default function Layout({ onLogout }: { onLogout: () => void }) {
  return (
    <div className="flex h-screen bg-slate-950 text-white">
      <Sidebar onLogout={onLogout} />
      <main className="flex-1 overflow-y-auto pb-20 md:pb-0">
        <Outlet />
      </main>
      <BottomNav />
    </div>
  );
}
