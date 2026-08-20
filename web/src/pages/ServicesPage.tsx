import { Edit3, Globe2, Network, Plus, Power } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Badge, Button, ConfirmDialog, EmptyState, ErrorState, FormField, LoadingState, Modal, PageHeader, Pagination, Panel } from "../components/ui";
import { usePagedResource } from "../hooks/usePagedResource";
import { api } from "../lib/api";
import { formatDate } from "../lib/format";
import type { Host, MonitoredService, ServiceInput } from "../types";

const emptyInput: ServiceInput = { host_id: 0, name: "", type: "http", target: "", description: "" };

export function ServicesPage() {
  const loader = useCallback((page: number, size: number) => api.listServices(page, size), []);
  const state = usePagedResource(loader);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [editing, setEditing] = useState<MonitoredService | "new" | null>(null);
  const [disabling, setDisabling] = useState<MonitoredService | null>(null);
  const [busy, setBusy] = useState(false); const [actionError, setActionError] = useState("");
  useEffect(() => { api.listHosts(1, 100).then((page) => setHosts(page.items.filter((item) => item.status === "active"))).catch(() => undefined); }, []);
  const hostNames = new Map(hosts.map((item) => [item.id, item.name]));
  const disable = async () => { if (!disabling) return; setBusy(true); setActionError(""); try { await api.disableService(disabling.id); setDisabling(null); await state.reload(); } catch (reason) { setActionError(reason instanceof Error ? reason.message : "停用失败"); } finally { setBusy(false); } };
  return <>
    <PageHeader eyebrow="资产管理" title="监控服务" description="为主机配置 HTTP 或 TCP 服务目标，统一维护探测入口。" actions={<Button onClick={() => setEditing("new")} disabled={!hosts.length}><Plus size={17} />创建服务</Button>} />
    {!hosts.length && <div className="inline-alert inline-alert--info">请先登记至少一台启用状态的主机，再创建监控服务。</div>}{actionError && <div className="inline-alert">{actionError}</div>}
    <Panel>{state.loading ? <LoadingState /> : state.error ? <ErrorState message={state.error} onRetry={state.reload} /> : !state.data?.items.length ? <EmptyState title="还没有监控服务" description="HTTP 服务用于接口可用性探测，TCP 服务用于端口连通性探测。" /> : <><div className="table-wrap"><table><thead><tr><th>服务</th><th>类型</th><th>探测目标</th><th>所属主机</th><th>状态</th><th>更新时间</th><th className="table-actions">操作</th></tr></thead><tbody>{state.data.items.map((service) => <tr key={service.id}><td><div className="entity-cell"><span className="entity-icon">{service.type === "http" ? <Globe2 size={18} /> : <Network size={18} />}</span><div><strong>{service.name}</strong><small>服务 ID #{service.id}</small></div></div></td><td><span className="type-chip">{service.type.toUpperCase()}</span></td><td className="target-cell"><code title={service.target}>{service.target}</code></td><td>{hostNames.get(service.host_id) ?? `主机 #${service.host_id}`}</td><td><Badge status={service.status} /></td><td>{formatDate(service.updated_at)}</td><td className="table-actions"><button className="row-action" onClick={() => setEditing(service)} disabled={service.status === "disabled"}><Edit3 size={16} />编辑</button><button className="row-action row-action--danger" onClick={() => setDisabling(service)} disabled={service.status === "disabled"}><Power size={16} />停用</button></td></tr>)}</tbody></table></div><Pagination page={state.page} totalPages={Number(state.data.pagination.total_pages)} total={Number(state.data.pagination.total)} onChange={state.setPage} /></>}</Panel>
    <ServiceForm target={editing} hosts={hosts} onClose={() => setEditing(null)} onSaved={async () => { setEditing(null); await state.reload(); }} />
    <ConfirmDialog open={Boolean(disabling)} title={`停用服务「${disabling?.name ?? ""}」`} message="服务下存在启用状态的探测任务时，平台会拒绝停用。请先停用关联任务。" onClose={() => setDisabling(null)} onConfirm={disable} busy={busy} />
  </>;
}

function ServiceForm({ target, hosts, onClose, onSaved }: { target: MonitoredService | "new" | null; hosts: Host[]; onClose: () => void; onSaved: () => void }) {
  const [input, setInput] = useState<ServiceInput>(emptyInput); const [error, setError] = useState(""); const [busy, setBusy] = useState(false);
  useEffect(() => {
    setInput(target && target !== "new" ? { host_id: target.host_id, name: target.name, type: target.type, target: target.target, description: target.description } : { ...emptyInput, host_id: hosts[0]?.id ?? 0 });
    setError("");
  }, [target, hosts]);
  const submit = async (event: FormEvent) => { event.preventDefault(); if (!input.host_id || !input.name.trim() || !input.target.trim()) return setError("所属主机、服务名称和探测目标不能为空"); setBusy(true); setError(""); try { if (target && target !== "new") await api.updateService(target.id, input); else await api.createService(input); onSaved(); } catch (reason) { setError(reason instanceof Error ? reason.message : "保存失败"); } finally { setBusy(false); } };
  return <Modal open={Boolean(target)} title={target === "new" ? "创建监控服务" : "编辑监控服务"} description="HTTP 目标需要完整 URL；TCP 目标使用 host:port。" onClose={onClose}><form onSubmit={submit}><div className="modal__body form-grid form-grid--two"><FormField label="所属主机"><select value={input.host_id} onChange={(e) => setInput({ ...input, host_id: Number(e.target.value) })}>{hosts.map((host) => <option key={host.id} value={host.id}>{host.name} · {host.address}</option>)}</select></FormField><FormField label="服务类型"><select value={input.type} onChange={(e) => setInput({ ...input, type: e.target.value as "http" | "tcp", target: "" })}><option value="http">HTTP / HTTPS</option><option value="tcp">TCP 端口</option></select></FormField><FormField label="服务名称"><input value={input.name} onChange={(e) => setInput({ ...input, name: e.target.value })} placeholder="例如：订单 API" /></FormField><FormField label="探测目标"><input value={input.target} onChange={(e) => setInput({ ...input, target: e.target.value })} placeholder={input.type === "http" ? "https://api.example.com/healthz" : "10.0.2.15:3306"} /></FormField><div className="form-grid__full"><FormField label="说明"><textarea value={input.description} onChange={(e) => setInput({ ...input, description: e.target.value })} placeholder="记录服务用途、SLA 或责任团队" rows={3} /></FormField></div>{error && <div className="form-alert form-grid__full">{error}</div>}</div><div className="modal__actions"><Button type="button" variant="secondary" onClick={onClose}>取消</Button><Button type="submit" disabled={busy}>{busy ? "保存中…" : "保存服务"}</Button></div></form></Modal>;
}
