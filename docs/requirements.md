# CloudSentinel MVP 需求

## 目标

将 CloudSentinel 恢复为一个 Docker Compose MVP：能够登记 Host 与 Service、调度 HTTP/TCP 探测、持久记录结果、暴露 Prometheus 指标、通过 Alertmanager 路由告警，并管理 Incident 从产生到恢复和关闭的完整生命周期。

## 功能范围

1. 通过 JWT 认证的用户可以管理 Host、Monitored Service 和 Probe Task。
2. Worker 将到期任务调度到 Redis Streams，并使用有界 Worker Pool 消费。
3. HTTP/TCP Probe 遵守重试、超时、SSRF、TLS 和幂等规则。
4. 探测执行生命周期和结果持久化到 MySQL。
5. Prometheus 抓取 API 与 Worker 指标，并由自动预置的 Grafana 展示。
6. Alertmanager 发送经过认证的 Firing/Resolved Webhook，用于创建和更新 Incident。
7. Health 只表示进程存活；Readiness 检查 MySQL 和 Redis。
8. 正式业务管理门户覆盖登录、资产、监控服务、探测任务、探测结果、故障事件和个人身份查看；Grafana 继续承担指标分析，不在业务门户中重复建设。

## 质量约束

- 采用模块化单体分层，并独立部署两个进程。
- 只允许 SQL Migration；禁止使用 GORM `AutoMigrate`。
- Redis Streams 使用 at-least-once 投递，通过数据库保证幂等。
- 支持优雅退出和有界并发。
- 源码、日志、审计数据和指标中不得出现 Secret。
- 必须具备自动化测试以及 Compose E2E 与故障验证。

## 排除项

自动修复、Automation Runner、Kafka、多租户、完整 RBAC、Refresh Token、OAuth/OIDC、通用 Kubernetes/Argo CD 管理界面、替代 Grafana 的复杂分析前端，以及 Prometheus/Grafana/Alertmanager 之外的可观测性系统，均不在当前业务范围内。Kubernetes GitOps 与业务管理门户属于已经批准的交付扩展，具体边界分别以 `docs/gitops.md` 和 `docs/frontend.md` 为准。
