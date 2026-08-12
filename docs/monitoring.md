# 监控与告警

API 和 Worker 分别创建独立的 Prometheus Registry。API 指标覆盖请求数量和耗时，Label 仅包含有界的 `method`、路由模板和状态码。Worker 指标覆盖探测数量、探测耗时、基于数据库的最新状态、Active Worker 数和进程内 Queue Length。所有耗时统一使用秒。

Compose 环境通过 Compose DNS 抓取自身、API 和 Worker。Kubernetes 环境的 Service/Pod 带有受控 Prometheus Scrape 注解，由平台监控系统发现；规则和 Alertmanager 路由应由平台监控仓库复用并维护现有契约。预置规则包括带有非零 `for` 时长及 Incident Label 的 `CloudSentinelProbeFailure`，以及设置了 `cloudsentinel_incident=false` 的 `CloudSentinelWorkerDown`。

Alertmanager 只把 `cloudsentinel_incident=true` 的告警路由到经过机器认证的 API Webhook，并发送 Resolved 通知。Grafana 自动预置 Prometheus Datasource 和 CloudSentinel Overview Dashboard，无需手工配置。

禁止在指标 Label 中使用 Secret 或无界/请求级数据，例如 Request ID、Execution ID、用户身份、客户端 IP、目标地址/URL、错误消息、时间戳和 Worker Consumer Name。

实际导出的 Metric Family 如下：

- API：`cloudsentinel_http_requests_total`、`cloudsentinel_http_request_duration_seconds`，Label 为 `method`、路由模板和状态码。
- Worker：`cloudsentinel_probe_total`、`cloudsentinel_probe_duration_seconds`、`cloudsentinel_probe_status`、`cloudsentinel_worker_active` 和 `cloudsentinel_worker_queue_length`。
- Incident：`cloudsentinel_active_incidents`，从共享 MySQL 状态重建。

Prometheus 对 `CloudSentinelProbeFailure` 持续评估 30 秒后才进入 Firing，并且只把带 `cloudsentinel_incident="true"` Label 的告警路由到 Incident Webhook。`CloudSentinelWorkerDown` 只用于运维告警，明确不创建 Incident。Alertmanager 按 Alert、Service、Task 和 Probe Type 分组；等待 5 秒后发送首次通知，每 1 分钟重复，并发送 Resolved 通知。
