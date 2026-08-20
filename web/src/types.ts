export type Status = "active" | "disabled";

export interface Pagination {
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
}

export interface Page<T> {
  items: T[];
  pagination: Pagination;
}

export interface User {
  id: number;
  username: string;
  status: Status;
  last_login_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface LoginResult {
  access_token: string;
  token_type: string;
  expires_in: number;
  user: User;
}

export interface Host {
  id: number;
  name: string;
  address: string;
  description: string;
  status: Status;
  created_by: number;
  updated_by: number;
  created_at: string;
  updated_at: string;
}

export interface HostInput {
  name: string;
  address: string;
  description: string;
}

export interface MonitoredService {
  id: number;
  host_id: number;
  name: string;
  type: "http" | "tcp";
  target: string;
  description: string;
  status: Status;
  created_by: number;
  updated_by: number;
  created_at: string;
  updated_at: string;
}

export interface ServiceInput {
  host_id: number;
  name: string;
  type: "http" | "tcp";
  target: string;
  description: string;
}

export interface ProbeTask {
  id: number;
  service_id: number;
  name: string;
  interval_seconds: number;
  timeout_milliseconds: number;
  max_retries: number;
  retry_base_delay_milliseconds: number;
  status: Status;
  next_run_at: string;
  last_scheduled_at?: string | null;
  created_at: string;
  updated_at: string;
}

export type ProbeTaskInput = Pick<
  ProbeTask,
  | "service_id"
  | "name"
  | "interval_seconds"
  | "timeout_milliseconds"
  | "max_retries"
  | "retry_base_delay_milliseconds"
>;

export interface ProbeResult {
  id: number;
  execution_id: string;
  task_id: number;
  service_id: number;
  probe_type: "http" | "tcp";
  target_snapshot: string;
  scheduled_at: string;
  started_at?: string | null;
  finished_at?: string | null;
  duration_milliseconds?: number | null;
  status: "queued" | "running" | "succeeded" | "failed";
  success?: boolean | null;
  attempt_count: number;
  http_status_code?: number | null;
  error_code?: string | null;
  error_message?: string | null;
  worker_consumer?: string | null;
  created_at: string;
  updated_at: string;
}

export interface Incident {
  id: number;
  event_key: string;
  alert_name: string;
  service_id: number;
  task_id: number;
  probe_type: "http" | "tcp";
  severity: string;
  status: "firing" | "acknowledged" | "processing" | "resolved" | "closed";
  summary: string;
  description: string;
  fired_at: string;
  last_seen_at: string;
  resolved_at?: string | null;
  closed_at?: string | null;
  acknowledged_at?: string | null;
  processing_at?: string | null;
  occurrence_count: number;
  created_at: string;
  updated_at: string;
}

export interface HealthResult {
  status: string;
  dependencies?: Record<string, string>;
}
