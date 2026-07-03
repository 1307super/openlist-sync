import { BrowserRouter, Routes, Route } from "react-router-dom";
import { useAuth } from "./hooks/useAuth";
import { setOnUnauthorized } from "./api/client";
import { useEffect } from "react";
import Layout from "./components/Layout";
import LoginPage from "./pages/LoginPage";
import SettingsPage from "./pages/SettingsPage";
import TasksPage from "./pages/TasksPage";
import TaskDetailPage from "./pages/TaskDetailPage";
import MonitorPage from "./pages/MonitorPage";

export default function App() {
  const { token, isAuthenticated, login, logout } = useAuth();

  useEffect(() => {
    setOnUnauthorized(logout);
  }, [logout]);

  if (!isAuthenticated) {
    return <LoginPage onLogin={login} />;
  }

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout onLogout={logout} />}>
          <Route path="/" element={<TasksPage />} />
          <Route path="/monitor" element={<MonitorPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/tasks/:id" element={<TaskDetailPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
