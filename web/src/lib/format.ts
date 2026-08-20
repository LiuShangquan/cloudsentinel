export function formatDate(value?: string | null): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(date);
}

export function formatDuration(milliseconds?: number | null): string {
  if (milliseconds === null || milliseconds === undefined) return "—";
  if (milliseconds < 1000) return `${milliseconds} ms`;
  return `${(milliseconds / 1000).toFixed(2)} s`;
}

export function formatInterval(seconds: number): string {
  if (seconds < 60) return `${seconds} 秒`;
  if (seconds < 3600) return `${Math.round(seconds / 60)} 分钟`;
  return `${Math.round(seconds / 3600)} 小时`;
}

export function statusLabel(status?: string): string {
  const labels: Record<string, string> = {
    active: "启用",
    disabled: "停用",
    queued: "排队中",
    running: "执行中",
    succeeded: "成功",
    failed: "失败",
    firing: "告警中",
    acknowledged: "已确认",
    processing: "处理中",
    resolved: "已恢复",
    closed: "已关闭",
  };
  return status ? labels[status] ?? status : "未知";
}

export function statusTone(status?: string): "success" | "danger" | "warning" | "info" | "neutral" {
  if (["active", "succeeded", "resolved", "closed"].includes(status ?? "")) return "success";
  if (["failed", "firing"].includes(status ?? "")) return "danger";
  if (["queued", "acknowledged"].includes(status ?? "")) return "warning";
  if (["running", "processing"].includes(status ?? "")) return "info";
  return "neutral";
}
