import { Clock3, Edit3, Plus, Power, Radar } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Badge, Button, ConfirmDialog, EmptyState, ErrorState, FormField, LoadingState, Modal, PageHeader, Pagination, Panel } from "../components/ui";
import { usePagedResource } from "../hooks/usePagedResource";
import { api } from "../lib/api";
import { formatDate, formatInterval } from "../lib/format";
import type { MonitoredService, ProbeTask, ProbeTaskInput } from "../types";

const emptyInput: ProbeTaskInput = { service_id: 0, name: "", interval_seconds: 60, timeout_milliseconds: 5000, max_retries: 1, retry_base_delay_milliseconds: 500 };

export function TasksPage() {
  const loader = useCallback((page: number, size: number) => api.listTasks(page, size), []);
  const state = usePagedResource(loader);
  const [services, setServices] = useState<MonitoredService[]>([]);
  const [editing, setEditing] = useState<ProbeTask | "new" | null>(null);
  const [disabling, setDisabling] = useState<ProbeTask | null>(null);
  const [busy, setBusy] = useState(false); const [actionError, setActionError] = useState("");
  useEffect(() => { api.listServices(1, 100).then((page) => setServices(page.items.filter((item) => item.status === "active"))).catch(() => undefined); }, []);
  const names = new Map(services.map((item) => [item.id, item.name]));
  const disable = async () => { if (!disabling) return; setBusy(true); setActionError(""); try { await api.disableTask(disabling.id); setDisabling(null); await state.reload(); } catch (reason) { setActionError(reason instanceof Error ? reason.message : "停用失败"); } finally { setBusy(false); } };
  return <>
    <PageHeader eyebrow="探测中心" title="探测任务" description="配置执行周期、超时与重试边界，由 Worker 持续调度执行。" actions={<Button onClick={() => setEditing("new")} disabled={!services.length}><Plus size={17} />创建任务</Button>} />
    {!services.length && <div className="inline-alert inline-alert--info">请先创建至少一项启用状态的监控服务。</div>}{actionError && <div className="inline-alert">{actionError}</div>}
    <Panel>{state.loading ? <LoadingState /> : state.error ? <ErrorState message={state.error} onRetry={state.reload} /> : !state.data?.items.length ? <EmptyState title="还没有探测任务" description="创建任务后，调度器会按周期投递到 Redis Streams，由 Worker 有界执行。" /> : <><div className="table-wrap"><table><thead><tr><th>任务</th><th>监控服务</th><th>执行周期</th><th>超时 / 重试</th><th>下次执行</th><th>状态</th><th className="table-actions">操作</th></tr></thead><tbody>{state.data.items.map((task) => <tr key={task.id}><td><div className="entity-cell"><span className="entity-icon"><Radar size={18} /></span><div><strong>{task.name}</strong><small>任务 ID #{task.id}</small></div></div></td><td>{names.get(task.service_id) ?? `服务 #${task.service_id}`}</td><td><div className="cell-stack"><strong>{formatInterval(task.interval_seconds)}</strong><small><Clock3 size={13} /> 周期执行</small></div></td><td><div className="cell-stack"><strong>{task.timeout_milliseconds} ms</strong><small>最多重试 {task.max_retries} 次</small></div></td><td>{formatDate(task.next_run_at)}</td><td><Badge status={task.status} /></td><td className="table-actions"><button className="row-action" onClick={() => setEditing(task)} disabled={task.status === "disabled"}><Edit3 size={16} />编辑</button><button className="row-action row-action--danger" onClick={() => setDisabling(task)} disabled={task.status === "disabled"}><Power size={16} />停用</button></td></tr>)}</tbody></table></div><Pagination page={state.page} totalPages={Number(state.data.pagination.total_pages)} total={Number(state.data.pagination.total)} onChange={state.setPage} /></>}</Panel>
    <TaskForm target={editing} services={services} onClose={() => setEditing(null)} onSaved={async () => { setEditing(null); await state.reload(); }} />
    <ConfirmDialog open={Boolean(disabling)} title={`停用任务「${disabling?.name ?? ""}」`} message="停用后调度器不会再创建新的执行，但既有结果和故障记录会保留。" onClose={() => setDisabling(null)} onConfirm={disable} busy={busy} />
  </>;
}

function TaskForm({ target, services, onClose, onSaved }: { target: ProbeTask | "new" | null; services: MonitoredService[]; onClose: () => void; onSaved: () => void }) {
  const [input, setInput] = useState<ProbeTaskInput>(emptyInput); const [error, setError] = useState(""); const [busy, setBusy] = useState(false);
  useEffect(() => {
    setInput(target && target !== "new" ? { service_id: target.service_id, name: target.name, interval_seconds: target.interval_seconds, timeout_milliseconds: target.timeout_milliseconds, max_retries: target.max_retries, retry_base_delay_milliseconds: target.retry_base_delay_milliseconds } : { ...emptyInput, service_id: services[0]?.id ?? 0 });
    setError("");
  }, [target, services]);
  const submit = async (event: FormEvent) => { event.preventDefault(); if (!input.service_id || !input.name.trim()) return setError("监控服务和任务名称不能为空"); if (input.interval_seconds < 10 || input.timeout_milliseconds < 100) return setError("周期至少 10 秒，超时至少 100 毫秒"); setBusy(true); setError(""); try { if (target && target !== "new") await api.updateTask(target.id, input); else await api.createTask(input); onSaved(); } catch (reason) { setError(reason instanceof Error ? reason.message : "保存失败"); } finally { setBusy(false); } };
  return <Modal open={Boolean(target)} title={target === "new" ? "创建探测任务" : "编辑探测任务"} description="所有时间边界均受后端校验，避免无界探测占用资源。" onClose={onClose}><form onSubmit={submit}><div className="modal__body form-grid form-grid--two"><FormField label="监控服务"><select value={input.service_id} onChange={(e) => setInput({ ...input, service_id: Number(e.target.value) })}>{services.map((service) => <option key={service.id} value={service.id}>{service.name} · {service.type.toUpperCase()}</option>)}</select></FormField><FormField label="任务名称"><input value={input.name} maxLength={100} onChange={(e) => setInput({ ...input, name: e.target.value })} placeholder="例如：核心接口健康检查" /></FormField><FormField label="执行周期（秒）" hint="10–86400 秒"><input type="number" min={10} max={86400} value={input.interval_seconds} onChange={(e) => setInput({ ...input, interval_seconds: Number(e.target.value) })} /></FormField><FormField label="超时时间（毫秒）" hint="100–60000 毫秒"><input type="number" min={100} max={60000} value={input.timeout_milliseconds} onChange={(e) => setInput({ ...input, timeout_milliseconds: Number(e.target.value) })} /></FormField><FormField label="最大重试次数" hint="0–5 次"><input type="number" min={0} max={5} value={input.max_retries} onChange={(e) => setInput({ ...input, max_retries: Number(e.target.value) })} /></FormField><FormField label="重试基础延迟（毫秒）" hint="100–30000 毫秒"><input type="number" min={100} max={30000} value={input.retry_base_delay_milliseconds} onChange={(e) => setInput({ ...input, retry_base_delay_milliseconds: Number(e.target.value) })} /></FormField>{error && <div className="form-alert form-grid__full">{error}</div>}</div><div className="modal__actions"><Button type="button" variant="secondary" onClick={onClose}>取消</Button><Button type="submit" disabled={busy}>{busy ? "保存中…" : "保存任务"}</Button></div></form></Modal>;
}
