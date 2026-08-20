import { Navigate, Outlet, Route, Routes, useLocation } from "react-router-dom";
import { useAuth } from "./auth/AuthContext";
import { AppShell } from "./components/AppShell";
import { LoadingState } from "./components/ui";
import { DashboardPage } from "./pages/DashboardPage";
import { HostsPage } from "./pages/HostsPage";
import { IncidentsPage } from "./pages/IncidentsPage";
import { LoginPage } from "./pages/LoginPage";
import { ProfilePage } from "./pages/ProfilePage";
import { ResultsPage } from "./pages/ResultsPage";
import { ServicesPage } from "./pages/ServicesPage";
import { TasksPage } from "./pages/TasksPage";

function ProtectedRoute() {
  const { user, loading } = useAuth();
  const location = useLocation();
  if (loading) return <div className="boot-screen"><LoadingState label="正在恢复安全会话" /></div>;
  return user ? <Outlet /> : <Navigate to="/login" replace state={{ from: location.pathname }} />;
}

export default function App() {
  return <Routes>
    <Route path="/login" element={<LoginPage />} />
    <Route element={<ProtectedRoute />}><Route element={<AppShell />}><Route index element={<DashboardPage />} /><Route path="hosts" element={<HostsPage />} /><Route path="services" element={<ServicesPage />} /><Route path="probe-tasks" element={<TasksPage />} /><Route path="probe-results" element={<ResultsPage />} /><Route path="incidents" element={<IncidentsPage />} /><Route path="profile" element={<ProfilePage />} /></Route></Route>
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes>;
}
