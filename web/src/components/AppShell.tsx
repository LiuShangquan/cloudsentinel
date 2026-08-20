import { Activity, BellRing, Boxes, ChevronDown, Gauge, LogOut, Menu, Radar, Server, ShieldCheck, UserRound, X } from "lucide-react";
import { useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";

const navigation = [
  { to: "/", label: "运行总览", icon: Gauge, end: true },
  { to: "/hosts", label: "主机资产", icon: Server },
  { to: "/services", label: "监控服务", icon: Boxes },
  { to: "/probe-tasks", label: "探测任务", icon: Radar },
  { to: "/probe-results", label: "执行结果", icon: Activity },
  { to: "/incidents", label: "故障事件", icon: BellRing },
];

const titles: Record<string, string> = {
  "/": "运行总览", "/hosts": "主机资产", "/services": "监控服务", "/probe-tasks": "探测任务", "/probe-results": "执行结果", "/incidents": "故障事件", "/profile": "账户信息",
};

export function AppShell() {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [accountOpen, setAccountOpen] = useState(false);
  const { user, logout } = useAuth();
  const location = useLocation();
  const title = titles[location.pathname] ?? "CloudSentinel";

  return <div className="app-shell">
    {mobileOpen && <div className="sidebar-scrim" onClick={() => setMobileOpen(false)} />}
    <aside className={`sidebar ${mobileOpen ? "sidebar--open" : ""}`}>
      <div className="brand"><div className="brand__mark"><ShieldCheck size={25} /></div><div><strong>CloudSentinel</strong><span>服务可靠性平台</span></div><button className="sidebar-close" onClick={() => setMobileOpen(false)}><X size={19} /></button></div>
      <div className="sidebar__label">工作台</div>
      <nav>{navigation.map(({ to, label, icon: Icon, end }) => <NavLink key={to} to={to} end={end} onClick={() => setMobileOpen(false)} className={({ isActive }) => isActive ? "nav-item nav-item--active" : "nav-item"}><Icon size={19} /><span>{label}</span></NavLink>)}</nav>
      <div className="sidebar__footer"><span className="system-dot" />系统已连接<div>Enterprise GitOps · v0.1</div></div>
    </aside>
    <div className="app-main">
      <header className="topbar"><div className="topbar__title"><button className="menu-button" onClick={() => setMobileOpen(true)} aria-label="打开菜单"><Menu size={20} /></button><div><span>CloudSentinel /</span><strong>{title}</strong></div></div><div className="account"><button className="account__trigger" onClick={() => setAccountOpen((value) => !value)}><span className="avatar">{user?.username.slice(0, 1).toUpperCase()}</span><span className="account__name"><strong>{user?.username}</strong><small>平台管理员</small></span><ChevronDown size={16} /></button>{accountOpen && <div className="account__menu"><NavLink to="/profile" onClick={() => setAccountOpen(false)}><UserRound size={16} />账户信息</NavLink><button onClick={logout}><LogOut size={16} />退出登录</button></div>}</div></header>
      <main className="page"><Outlet /></main>
    </div>
  </div>;
}
