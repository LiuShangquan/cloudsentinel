import { CheckCircle2, CircleDashed, Eye, TimerReset, XCircle } from "lucide-react";
import { useCallback, useState } from "react";
import { Badge, Button, EmptyState, ErrorState, LoadingState, Modal, PageHeader, Pagination, Panel } from "../components/ui";
import { usePagedResource } from "../hooks/usePagedResource";
import { api } from "../lib/api";
import { formatDate, formatDuration } from "../lib/format";
import type { ProbeResult } from "../types";

export function ResultsPage() {
  const loader = useCallback((page: number, size: number) => api.listResults(page, size), []);
  const state = usePagedResource(loader);
  const [selected, setSelected] = useState<ProbeResult | null>(null);
  return <>
    <PageHeader eyebrow="探测中心" title="执行结果" description="追踪每次 HTTP/TCP 探测的状态、耗时、响应码和错误原因。" actions={<Button variant="secondary" onClick={state.reload}><TimerReset size={17} />刷新结果</Button>} />
    <Panel>{state.loading ? <LoadingState /> : state.error ? <ErrorState message={state.error} onRetry={state.reload} /> : !state.data?.items.length ? <EmptyState title="暂无执行结果" description="启用探测任务并等待首次调度后，执行证据会出现在这里。" /> : <><div className="table-wrap"><table><thead><tr><th>执行</th><th>类型与目标</th><th>状态</th><th>响应</th><th>耗时</th><th>执行时间</th><th className="table-actions">操作</th></tr></thead><tbody>{state.data.items.map((result) => <tr key={result.id}><td><div className="entity-cell"><span className={`entity-icon ${result.success ? "entity-icon--success" : result.status === "running" ? "entity-icon--info" : "entity-icon--danger"}`}>{result.success ? <CheckCircle2 size={18} /> : result.status === "running" ? <CircleDashed size={18} /> : <XCircle size={18} />}</span><div><strong>{result.execution_id.slice(0, 12)}</strong><small>任务 #{result.task_id} · 第 {result.attempt_count} 次</small></div></div></td><td><div className="cell-stack"><strong>{result.probe_type.toUpperCase()}</strong><small className="ellipsis" title={result.target_snapshot}>{result.target_snapshot}</small></div></td><td><Badge status={result.status} /></td><td>{result.http_status_code ?? result.error_code ?? "—"}</td><td>{formatDuration(result.duration_milliseconds)}</td><td>{formatDate(result.finished_at ?? result.scheduled_at)}</td><td className="table-actions"><button className="row-action" onClick={() => setSelected(result)}><Eye size={16} />详情</button></td></tr>)}</tbody></table></div><Pagination page={state.page} totalPages={Number(state.data.pagination.total_pages)} total={Number(state.data.pagination.total)} onChange={state.setPage} /></>}</Panel>
    <ResultDetail result={selected} onClose={() => setSelected(null)} />
  </>;
}

function ResultDetail({ result, onClose }: { result: ProbeResult | null; onClose: () => void }) {
  return <Modal open={Boolean(result)} title="执行结果详情" description="该记录是 Worker 持久化的单次探测证据。" onClose={onClose}>{result && <div className="modal__body detail-grid"><Detail label="执行 ID" value={result.execution_id} mono /><Detail label="状态" value={<Badge status={result.status} />} /><Detail label="探测类型" value={result.probe_type.toUpperCase()} /><Detail label="目标快照" value={result.target_snapshot} mono wide /><Detail label="计划时间" value={formatDate(result.scheduled_at)} /><Detail label="开始时间" value={formatDate(result.started_at)} /><Detail label="完成时间" value={formatDate(result.finished_at)} /><Detail label="总耗时" value={formatDuration(result.duration_milliseconds)} /><Detail label="HTTP 状态码" value={result.http_status_code ?? "—"} /><Detail label="尝试次数" value={result.attempt_count} /><Detail label="错误代码" value={result.error_code ?? "—"} /><Detail label="错误信息" value={result.error_message ?? "—"} wide /><Detail label="Worker" value={result.worker_consumer ?? "—"} mono wide /></div>}<div className="modal__actions"><Button variant="secondary" onClick={onClose}>关闭</Button></div></Modal>;
}

function Detail({ label, value, mono, wide }: { label: string; value: React.ReactNode; mono?: boolean; wide?: boolean }) { return <div className={`detail-item ${wide ? "detail-item--wide" : ""}`}><span>{label}</span><strong className={mono ? "mono" : ""}>{value}</strong></div>; }
