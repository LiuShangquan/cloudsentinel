import { Edit3, Plus, Power, Server } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Badge, Button, ConfirmDialog, EmptyState, ErrorState, FormField, LoadingState, Modal, PageHeader, Pagination, Panel } from "../components/ui";
import { usePagedResource } from "../hooks/usePagedResource";
import { api } from "../lib/api";
import { formatDate } from "../lib/format";
import type { Host, HostInput } from "../types";

const emptyInput: HostInput = { name: "", address: "", description: "" };

export function HostsPage() {
  const loader = useCallback((page: number, size: number) => api.listHosts(page, size), []);
  const state = usePagedResource(loader);
  const [editing, setEditing] = useState<Host | "new" | null>(null);
  const [disabling, setDisabling] = useState<Host | null>(null);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState("");

  const disable = async () => {
    if (!disabling) return;
    setBusy(true); setActionError("");
    try { await api.disableHost(disabling.id); setDisabling(null); await state.reload(); }
    catch (reason) { setActionError(reason instanceof Error ? reason.message : "停用失败"); }
    finally { setBusy(false); }
  };

  return <>
    <PageHeader eyebrow="资产管理" title="主机资产" description="登记被监控主机及网络地址，作为服务探测的资产归属。" actions={<Button onClick={() => setEditing("new")}><Plus size={17} />登记主机</Button>} />
    {actionError && <div className="inline-alert">{actionError}</div>}
    <Panel>{state.loading ? <LoadingState /> : state.error ? <ErrorState message={state.error} onRetry={state.reload} /> : !state.data?.items.length ? <EmptyState title="还没有主机资产" description="登记第一台主机后，即可在其下创建监控服务。" action={<Button onClick={() => setEditing("new")}><Plus size={17} />登记主机</Button>} /> : <>
      <div className="table-wrap"><table><thead><tr><th>主机</th><th>网络地址</th><th>状态</th><th>说明</th><th>更新时间</th><th className="table-actions">操作</th></tr></thead><tbody>{state.data.items.map((host) => <tr key={host.id}><td><div className="entity-cell"><span className="entity-icon"><Server size={18} /></span><div><strong>{host.name}</strong><small>资产 ID #{host.id}</small></div></div></td><td><code>{host.address}</code></td><td><Badge status={host.status} /></td><td className="muted-cell">{host.description || "—"}</td><td>{formatDate(host.updated_at)}</td><td className="table-actions"><button className="row-action" onClick={() => setEditing(host)} disabled={host.status === "disabled"}><Edit3 size={16} />编辑</button><button className="row-action row-action--danger" onClick={() => setDisabling(host)} disabled={host.status === "disabled"}><Power size={16} />停用</button></td></tr>)}</tbody></table></div><Pagination page={state.page} totalPages={Number(state.data.pagination.total_pages)} total={Number(state.data.pagination.total)} onChange={state.setPage} /></>}
    </Panel>
    <HostForm target={editing} onClose={() => setEditing(null)} onSaved={async () => { setEditing(null); await state.reload(); }} />
    <ConfirmDialog open={Boolean(disabling)} title={`停用主机「${disabling?.name ?? ""}」`} message="主机下存在启用状态的监控服务时，平台会拒绝停用。请先处理关联服务。" onClose={() => setDisabling(null)} onConfirm={disable} busy={busy} />
  </>;
}

function HostForm({ target, onClose, onSaved }: { target: Host | "new" | null; onClose: () => void; onSaved: () => void }) {
  const [input, setInput] = useState<HostInput>(emptyInput);
  const [error, setError] = useState(""); const [busy, setBusy] = useState(false);
  useEffect(() => {
    setInput(target && target !== "new" ? { name: target.name, address: target.address, description: target.description } : { ...emptyInput });
    setError("");
  }, [target]);
  const submit = async (event: FormEvent) => { event.preventDefault(); if (!input.name.trim() || !input.address.trim()) return setError("主机名称和网络地址不能为空"); setBusy(true); setError(""); try { if (target && target !== "new") await api.updateHost(target.id, input); else await api.createHost(input); onSaved(); } catch (reason) { setError(reason instanceof Error ? reason.message : "保存失败"); } finally { setBusy(false); } };
  return <Modal open={Boolean(target)} title={target === "new" ? "登记主机" : "编辑主机"} description="地址可以是 IP、域名或主机标识，请确保平台工作节点能够访问。" onClose={onClose}><form onSubmit={submit}><div className="modal__body form-grid"><FormField label="主机名称"><input value={input.name} maxLength={100} onChange={(e) => setInput({ ...input, name: e.target.value })} placeholder="例如：payment-api-01" /></FormField><FormField label="网络地址"><input value={input.address} onChange={(e) => setInput({ ...input, address: e.target.value })} placeholder="例如：10.0.2.15 或 api.example.com" /></FormField><FormField label="说明"><textarea value={input.description} onChange={(e) => setInput({ ...input, description: e.target.value })} placeholder="记录环境、用途或责任团队" rows={3} /></FormField>{error && <div className="form-alert">{error}</div>}</div><div className="modal__actions"><Button type="button" variant="secondary" onClick={onClose}>取消</Button><Button type="submit" disabled={busy}>{busy ? "保存中…" : "保存主机"}</Button></div></form></Modal>;
}
