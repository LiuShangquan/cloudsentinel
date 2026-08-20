import { Activity, AlertTriangle, ArrowRight, CheckCircle2, Clock3, Radar, Server, Siren, XCircle } from "lucide-react";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Badge, ErrorState, LoadingState, PageHeader, Panel } from "../components/ui";
import { api } from "../lib/api";
import { formatDate, formatDuration } from "../lib/format";
import type { Host, Incident, MonitoredService, ProbeResult, ProbeTask } from "../types";

interface Snapshot { hosts: Host[]; services: MonitoredService[]; tasks: ProbeTask[]; results: ProbeResult[]; incidents: Incident[]; ready: boolean }

export function DashboardPage() {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [error, setError] = useState("");
  const load = async () => {
    setError("");
    try {
      const [hosts, services, tasks, results, incidents, readiness] = await Promise.all([
        api.listHosts(1, 100), api.listServices(1, 100), api.listTasks(1, 100), api.listResults(1, 8), api.listIncidents(1, 8), api.readiness(),
      ]);
      setSnapshot({ hosts: hosts.items, services: services.items, tasks: tasks.items, results: results.items, incidents: incidents.items, ready: readiness.status === "ready" });
    } catch (reason) { setError(reason instanceof Error ? reason.message : "加载失败"); }
  };
  useEffect(() => void load(), []);
  if (error) return <ErrorState message={error} onRetry={load} />;
  if (!snapshot) return <LoadingState label="正在汇总运行状态" />;

  const activeHosts = snapshot.hosts.filter((item) => item.status === "active").length;
  const activeTasks = snapshot.tasks.filter((item) => item.status === "active").length;
  const failures = snapshot.results.filter((item) => item.status === "failed").length;
  const openIncidents = snapshot.incidents.filter((item) => !["resolved", "closed"].includes(item.status)).length;
  const successCount = snapshot.results.filter((item) => item.success).length;
  const successRate = snapshot.results.length ? Math.round((successCount / snapshot.results.length) * 100) : 100;

  return <>
    <PageHeader eyebrow="控制台" title="运行总览" description="集中查看资产规模、探测执行和当前故障态势。" />
    <div className={`readiness-banner ${snapshot.ready ? "readiness-banner--ready" : "readiness-banner--warning"}`}><span>{snapshot.ready ? <CheckCircle2 size={19} /> : <AlertTriangle size={19} />}</span><div><strong>{snapshot.ready ? "平台服务运行正常" : "平台依赖尚未就绪"}</strong><small>{snapshot.ready ? "API、MySQL 与 Redis 依赖均通过就绪检查" : "请检查就绪状态后再执行管理操作"}</small></div><Badge status={snapshot.ready ? "active" : "firing"}>{snapshot.ready ? "Ready" : "Degraded"}</Badge></div>
    <div className="metric-grid">
      <MetricCard icon={<Server />} label="活跃主机" value={activeHosts} detail={`共登记 ${snapshot.hosts.length} 台`} tone="blue" />
      <MetricCard icon={<Radar />} label="活跃探测任务" value={activeTasks} detail={`覆盖 ${snapshot.services.length} 项服务`} tone="teal" />
      <MetricCard icon={<Activity />} label="近期成功率" value={`${successRate}%`} detail={`${snapshot.results.length} 条近期执行结果`} tone="green" />
      <MetricCard icon={<Siren />} label="待处置事件" value={openIncidents} detail={failures ? `近期发现 ${failures} 次失败` : "近期无失败执行"} tone={openIncidents ? "red" : "violet"} />
    </div>
    <div className="dashboard-grid">
      <Panel title="最近探测结果" description="展示最近的任务执行与响应状态" action={<Link className="text-link" to="/probe-results">查看全部 <ArrowRight size={15} /></Link>}>
        <div className="activity-list">{snapshot.results.length ? snapshot.results.map((item) => <div className="activity-row" key={item.id}><span className={`result-icon ${item.success ? "result-icon--success" : item.status === "running" ? "result-icon--running" : "result-icon--danger"}`}>{item.success ? <CheckCircle2 size={17} /> : item.status === "running" ? <Clock3 size={17} /> : <XCircle size={17} />}</span><div className="activity-row__main"><strong>{item.probe_type.toUpperCase()} · {item.target_snapshot}</strong><small>任务 #{item.task_id} · {formatDate(item.finished_at ?? item.scheduled_at)}</small></div><div className="activity-row__meta"><Badge status={item.status} /><span>{formatDuration(item.duration_milliseconds)}</span></div></div>) : <div className="compact-empty">暂无探测结果</div>}</div>
      </Panel>
      <Panel title="故障事件" description="需要关注的告警与处置进度" action={<Link className="text-link" to="/incidents">进入事件中心 <ArrowRight size={15} /></Link>}>
        <div className="activity-list">{snapshot.incidents.length ? snapshot.incidents.slice(0, 6).map((item) => <div className="incident-row" key={item.id}><div className={`severity-line severity-line--${item.severity || "warning"}`} /><div className="activity-row__main"><strong>{item.summary || item.alert_name}</strong><small>服务 #{item.service_id} · 发生 {item.occurrence_count} 次</small></div><Badge status={item.status} /></div>) : <div className="compact-empty compact-empty--success"><CheckCircle2 size={20} /> 当前没有故障事件</div>}</div>
      </Panel>
    </div>
  </>;
}

function MetricCard({ icon, label, value, detail, tone }: { icon: React.ReactNode; label: string; value: number | string; detail: string; tone: string }) {
  return <div className="metric-card"><div className={`metric-card__icon metric-card__icon--${tone}`}>{icon}</div><div><span>{label}</span><strong>{value}</strong><small>{detail}</small></div></div>;
}
