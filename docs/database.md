# 数据库设计

MySQL Schema 只由有序的 `migrations/*.up.sql` 和可回滚的 `.down.sql` 文件管理。应用启动时绝不执行 Schema Migration。

生产数据库固定使用集群外阿里云 RDS，不创建 MySQL StatefulSet。Argo CD 在每次应用同步前运行独立 PreSync Migration Job；Job 使用启用 TLS、且凭证经过 URL 编码的 `MIGRATION_DATABASE_URL` 和独立 DDL 账户，成功后才允许 API/Worker 滚动。运行时账户只授予业务 DML 权限。应用连接默认使用带证书和主机名校验的 TLS。生产变更采用 Expand/Contract；镜像回滚不会自动回滚 Schema，Down Migration 或 RDS PITR 必须走独立数据库恢复流程。

已实现的数据表：

- `users`：用户凭证、Active/Disabled 状态和最后登录时间。
- `audit_logs`：安全与变更审计轨迹；绝不存储 Secret。
- `hosts`：经过地址校验并支持软禁用的逻辑 Host。
- `services`：HTTP/TCP Monitored Service，在同一个 Host 内保持唯一。
- `probe_tasks`：调度周期以及重试/超时策略。
- `probe_results`：每个唯一 `execution_id` 对应一条持久化执行生命周期。
- `incidents`：由稳定 `event_key` 标识的唯一告警事件实例。

所有数据表均使用 UTC 时间戳。面向用户的删除操作是状态迁移，而不是物理删除。每个进程内的 GORM Repository 共享同一个底层 `sql.DB` 连接池。资产和 Incident 变更与对应 Audit Log 在同一个事务中写入。

关键唯一约束包括：`users.username`、`hosts.name`、`(services.host_id, services.name)`、`(probe_tasks.service_id, probe_tasks.name)`、`probe_results.execution_id` 和 `incidents.event_key`。

有序 Migration 集合如下：平台元数据（`000001`）、用户/审计（`000002`）、资产（`000003`）、探测系统（`000004`）和 Incident（`000005`）。外键保护用户、Host、Service 和 Task 的关联关系。调度器以原子方式推进 `next_run_at` 并创建 Queued Execution；Worker 使用条件状态更新，确保重复 Stream 投递不会产生多条最终结果。
