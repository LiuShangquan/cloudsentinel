# HTTP API 说明

成功响应统一使用 `{"code":0,"message":"success","data":...}`。领域错误使用稳定的非零错误码和适当的 HTTP 状态码；未预期错误使用错误码 `50000`。受保护的 Endpoint 必须携带 `Authorization: Bearer <JWT>`。

## 公开 Endpoint 与机器 Endpoint

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`
- `POST /api/v1/auth/login`
- `POST /api/v1/alerts/webhook`（使用 Alertmanager Machine Token，不使用 JWT）

## JWT 接口

- `GET /api/v1/users/me`
- `/api/v1/hosts` 的 CRUD/软禁用操作
- `/api/v1/services` 的 CRUD/软禁用操作
- `/api/v1/probe-tasks` 的 CRUD/软禁用操作
- `/api/v1/probe-results` 只读查询
- `/api/v1/incidents` 的列表、详情以及 Acknowledge/Process/Close 操作

列表 Endpoint 使用 `page`（默认 1）和 `page_size`（默认 20，最大 100），并返回 `items` 及 `{page,page_size,total,total_pages}` 分页元数据。创建操作返回 201；读取和状态迁移返回 200。非法输入返回 400，资源不存在返回 404，依赖或状态冲突返回 409，用户或机器认证无效返回 401；未预期错误返回 500，且不泄露内部细节。

Probe Task 输入为 `{service_id,name,interval_seconds,timeout_milliseconds,max_retries,retry_base_delay_milliseconds}`。有效范围分别为 10..86400 秒、100..60000 毫秒、0..5 次重试，以及 100..30000 毫秒基础延迟。Host/Service/Task 的 DELETE Endpoint 执行逻辑禁用。

Incident 状态只能通过三个专用 Action Endpoint 修改。合法用户状态迁移为 `firing -> acknowledged`、`firing|acknowledged -> processing` 和 `resolved -> closed`；Webhook Resolution 会把任意 Active 状态迁移为 `resolved`。
