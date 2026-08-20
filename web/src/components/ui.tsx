import { AlertTriangle, CheckCircle2, ChevronLeft, ChevronRight, LoaderCircle, X } from "lucide-react";
import { useEffect, type ButtonHTMLAttributes, type ReactNode } from "react";
import { statusLabel, statusTone } from "../lib/format";

export function Button({ className = "", variant = "primary", ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary" | "danger" | "ghost" }) {
  return <button className={`button button--${variant} ${className}`} {...props} />;
}

export function Badge({ status, children }: { status?: string; children?: ReactNode }) {
  return <span className={`badge badge--${statusTone(status)}`}><span className="badge__dot" />{children ?? statusLabel(status)}</span>;
}

export function PageHeader({ eyebrow, title, description, actions }: { eyebrow?: string; title: string; description: string; actions?: ReactNode }) {
  return <header className="page-header">
    <div>{eyebrow && <div className="eyebrow">{eyebrow}</div>}<h1>{title}</h1><p>{description}</p></div>
    {actions && <div className="page-header__actions">{actions}</div>}
  </header>;
}

export function Panel({ title, description, action, children, className = "" }: { title?: string; description?: string; action?: ReactNode; children: ReactNode; className?: string }) {
  return <section className={`panel ${className}`}>
    {(title || action) && <div className="panel__header"><div><h2>{title}</h2>{description && <p>{description}</p>}</div>{action}</div>}
    {children}
  </section>;
}

export function Modal({ open, title, description, onClose, children }: { open: boolean; title: string; description?: string; onClose: () => void; children: ReactNode }) {
  useEffect(() => {
    if (!open) return;
    const handler = (event: KeyboardEvent) => event.key === "Escape" && onClose();
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);
  if (!open) return null;
  return <div className="modal-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
    <section className="modal" role="dialog" aria-modal="true" aria-labelledby="modal-title">
      <header className="modal__header"><div><h2 id="modal-title">{title}</h2>{description && <p>{description}</p>}</div><button className="icon-button" onClick={onClose} aria-label="关闭"><X size={18} /></button></header>
      {children}
    </section>
  </div>;
}

export function FormField({ label, hint, error, children }: { label: string; hint?: string; error?: string; children: ReactNode }) {
  return <label className="form-field"><span className="form-field__label">{label}</span>{children}{hint && <span className="form-field__hint">{hint}</span>}{error && <span className="form-field__error">{error}</span>}</label>;
}

export function LoadingState({ label = "正在加载数据" }: { label?: string }) {
  return <div className="state-box"><LoaderCircle className="spin" size={24} /><strong>{label}</strong><span>请稍候，正在与 CloudSentinel 服务同步。</span></div>;
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return <div className="state-box state-box--error"><AlertTriangle size={24} /><strong>数据加载失败</strong><span>{message}</span>{onRetry && <Button variant="secondary" onClick={onRetry}>重新加载</Button>}</div>;
}

export function EmptyState({ title, description, action }: { title: string; description: string; action?: ReactNode }) {
  return <div className="state-box"><CheckCircle2 size={24} /><strong>{title}</strong><span>{description}</span>{action}</div>;
}

export function Pagination({ page, totalPages, total, onChange }: { page: number; totalPages: number; total: number; onChange: (page: number) => void }) {
  return <div className="pagination"><span>共 {total} 条，第 {page} / {Math.max(totalPages, 1)} 页</span><div><button className="icon-button" disabled={page <= 1} onClick={() => onChange(page - 1)} aria-label="上一页"><ChevronLeft size={18} /></button><button className="icon-button" disabled={page >= totalPages} onClick={() => onChange(page + 1)} aria-label="下一页"><ChevronRight size={18} /></button></div></div>;
}

export function ConfirmDialog({ open, title, message, confirmLabel = "确认停用", onConfirm, onClose, busy }: { open: boolean; title: string; message: string; confirmLabel?: string; onConfirm: () => void; onClose: () => void; busy?: boolean }) {
  return <Modal open={open} title={title} description="此操作会改变运行状态，请确认影响范围。" onClose={onClose}><div className="confirm-body"><AlertTriangle size={22} /><p>{message}</p></div><div className="modal__actions"><Button variant="secondary" onClick={onClose}>取消</Button><Button variant="danger" disabled={busy} onClick={onConfirm}>{busy ? "处理中…" : confirmLabel}</Button></div></Modal>;
}
