import type {
  HealthResult,
  Host,
  HostInput,
  Incident,
  LoginResult,
  MonitoredService,
  Page,
  ProbeResult,
  ProbeTask,
  ProbeTaskInput,
  ServiceInput,
  User,
} from "../types";

const TOKEN_KEY = "cloudsentinel.access_token";

interface Envelope<T> {
  code: number;
  message: string;
  data: T;
}

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public code?: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export const session = {
  getToken: () => sessionStorage.getItem(TOKEN_KEY),
  setToken: (token: string) => sessionStorage.setItem(TOKEN_KEY, token),
  clear: () => sessionStorage.removeItem(TOKEN_KEY),
};

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), 15_000);
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body) headers.set("Content-Type", "application/json");
  const token = session.getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);

  try {
    const response = await fetch(path, { ...init, headers, signal: controller.signal });
    const payload = (await response.json().catch(() => null)) as Envelope<T> | null;
    if (!response.ok || !payload || payload.code !== 0) {
      if (response.status === 401) session.clear();
      throw new ApiError(payload?.message || `请求失败（${response.status}）`, response.status, payload?.code);
    }
    return payload.data;
  } catch (error) {
    if (error instanceof ApiError) throw error;
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new ApiError("请求超时，请检查服务状态后重试", 408);
    }
    throw new ApiError("暂时无法连接 CloudSentinel 服务", 0);
  } finally {
    window.clearTimeout(timeout);
  }
}

const json = (value: unknown) => JSON.stringify(value);
const pageQuery = (page: number, pageSize = 20) => `?page=${page}&page_size=${pageSize}`;

export const api = {
  login: (username: string, password: string) =>
    request<LoginResult>("/api/v1/auth/login", { method: "POST", body: json({ username, password }) }),
  me: () => request<User>("/api/v1/users/me"),
  health: () => request<HealthResult>("/healthz"),
  readiness: () => request<HealthResult>("/readyz"),

  listHosts: (page = 1, size = 20) => request<Page<Host>>(`/api/v1/hosts${pageQuery(page, size)}`),
  createHost: (input: HostInput) => request<Host>("/api/v1/hosts", { method: "POST", body: json(input) }),
  updateHost: (id: number, input: HostInput) => request<Host>(`/api/v1/hosts/${id}`, { method: "PUT", body: json(input) }),
  disableHost: (id: number) => request<{ id: number; status: string }>(`/api/v1/hosts/${id}`, { method: "DELETE" }),

  listServices: (page = 1, size = 20) => request<Page<MonitoredService>>(`/api/v1/services${pageQuery(page, size)}`),
  createService: (input: ServiceInput) => request<MonitoredService>("/api/v1/services", { method: "POST", body: json(input) }),
  updateService: (id: number, input: ServiceInput) => request<MonitoredService>(`/api/v1/services/${id}`, { method: "PUT", body: json(input) }),
  disableService: (id: number) => request<{ id: number; status: string }>(`/api/v1/services/${id}`, { method: "DELETE" }),

  listTasks: (page = 1, size = 20) => request<Page<ProbeTask>>(`/api/v1/probe-tasks${pageQuery(page, size)}`),
  createTask: (input: ProbeTaskInput) => request<ProbeTask>("/api/v1/probe-tasks", { method: "POST", body: json(input) }),
  updateTask: (id: number, input: ProbeTaskInput) => request<ProbeTask>(`/api/v1/probe-tasks/${id}`, { method: "PUT", body: json(input) }),
  disableTask: (id: number) => request<{ id: number; status: string }>(`/api/v1/probe-tasks/${id}`, { method: "DELETE" }),

  listResults: (page = 1, size = 20) => request<Page<ProbeResult>>(`/api/v1/probe-results${pageQuery(page, size)}`),
  getResult: (id: number) => request<ProbeResult>(`/api/v1/probe-results/${id}`),

  listIncidents: (page = 1, size = 20) => request<Page<Incident>>(`/api/v1/incidents${pageQuery(page, size)}`),
  getIncident: (id: number) => request<Incident>(`/api/v1/incidents/${id}`),
  incidentAction: (id: number, action: "acknowledge" | "process" | "close") =>
    request<Incident>(`/api/v1/incidents/${id}/${action}`, { method: "POST" }),
};
