import { AlarmClockCheck, CheckCheck, Eye, Siren, Wrench } from "lucide-react";
import { useCallback, useState } from "react";
import { Badge, Button, EmptyState, ErrorState, LoadingState, Modal, PageHeader, Pagination, Panel } from "../components/ui";
import { usePagedResource } from "../hooks/usePagedResource";
import { api } from "../lib/api";
import { formatDate } from "../lib/format";
import type { Incident } from "../types";

export function IncidentsPage() {
  const loader = useCallback((page: number, size: number) => api.listIncidents(page, size), []);
  const state = usePagedResource(loader);
  const [selected, setSelected] = useState<Incident | null>(null);
  const [busy, setBusy] = useState(false); const [error, setError] = useState("");
  const act = async (action: "acknowledge" | "process" | "close") => { if (!selected) return; setBusy(true); setError(""); try { const updated = await api.incidentAction(selected.id, action); setSelected(updated); await state.reload(); } catch (reason) { setError(reason instanceof Error ? reason.message : "操作失败"); } finally { setBusy(false); } };
  return <>
    <PageHeader eyebrow="事件管理" title="故障事件" description="接收 Alertmanager 告警并推进确认、处理、恢复和关闭生命周期。" />
    <Panel>{state.loading ? <LoadingState /> : state.error ? <ErrorState message={state.error} onRetry={state.reload} /> : !state.data?.items.length ? <EmptyState title="当前没有故障事件" description="Prometheus 触发告警并经 Alertmanager 路由后，事件会自动进入这里。" /> : <><div className="table-wrap"><table><thead><tr><th>事件</th><th>级别</th><th>关联对象</th><th>发生次数</th><th>状态</th><th>最后发现</th><th className="table-actions">操作</th></tr></thead><tbody>{state.data.items.map((incident) => <tr key={incident.id}><td><div className="entity-cell"><span className="entity-icon entity-icon--danger"><Siren size={18} /></span><div><strong>{incident.summary || incident.alert_name}</strong><small>{incident.alert_name} · 事件 #{incident.id}</small></div></div></td><td><span className={`severity-tag severity-tag--${incident.severity || "warning"}`}>{incident.severity || "warning"}</span></td><td><div className="cell-stack"><strong>服务 #{incident.service_id}</strong><small>任务 #{incident.task_id} · {incident.probe_type.toUpperCase()}</small></div></td><td>{incident.occurrence_count}</td><td><Badge status={incident.status} /></td><td>{formatDate(incident.last_seen_at)}</td><td className="table-actions"><button className="row-action" onClick={() => setSelected(incident)}><Eye size={16} />处置</button></td></tr>)}</tbody></table></div><Pagination page={state.page} totalPages={Number(state.data.pagination.total_pages)} total={Number(state.data.pagination.total)} onChange={state.setPage} /></>}</Panel>
    <Modal open={Boolean(selected)} title={selected?.summary || selected?.alert_name || "事件详情"} description={`事件 #${selected?.id ?? ""} · ${selected?.alert_name ?? ""}`} onClose={() => setSelected(null)}>{selected && <><div className="modal__body"><div className="incident-detail__status"><Badge status={selected.status} /><span className={`severity-tag severity-tag--${selected.severity}`}>{selected.severity}</span></div><p className="incident-description">{selected.description || "暂无详细说明"}</p><div className="detail-grid"><Info label="首次发生" value={formatDate(selected.fired_at)} /><Info label="最后发现" value={formatDate(selected.last_seen_at)} /><Info label="累计次数" value={`${selected.occurrence_count} 次`} /><Info label="关联服务 / 任务" value={`#${selected.service_id} / #${selected.task_id}`} /><Info label="确认时间" value={formatDate(selected.acknowledged_at)} /><Info label="处理时间" value={formatDate(selected.processing_at)} /></div>{error && <div className="form-alert">{error}</div>}</div><div className="modal__actions"><Button variant="secondary" onClick={() => setSelected(null)}>关闭</Button>{selected.status === "firing" && <Button disabled={busy} onClick={() => act("acknowledge")}><AlarmClockCheck size={17} />确认事件</Button>}{["firing", "acknowledged"].includes(selected.status) && <Button disabled={busy} onClick={() => act("process")}><Wrench size={17} />开始处理</Button>}{selected.status === "resolved" && <Button disabled={busy} onClick={() => act("close")}><CheckCheck size={17} />关闭事件</Button>}</div></>}</Modal>
  </>;
}

function Info({ label, value }: { label: string; value: string }) { return <div className="detail-item"><span>{label}</span><strong>{value}</strong></div>; }
