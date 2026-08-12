# 路线图

## v0.1.0-compose-mvp（已完成）

- 两进程 Go 模块化单体、MySQL/Redis Streams、JWT、资产与任务 API。
- HTTP/TCP 探测、Prometheus/Grafana/Alertmanager、Incident 生命周期。
- Compose E2E、故障恢复、持久化与安全验收。

## 企业 GitOps 交付基线（当前）

- API/Worker 多副本运行准备与 Worker 健康端点。
- Kustomize Base、Staging/Production Overlay、Ingress、PDB、HPA、NetworkPolicy。
- 外部 RDS/Tair、ExternalSecret、PreSync Migration Job。
- Argo CD AppProject/ApplicationSet 和独立 GitOps 仓库。
- GitHub Actions 发布到北京 ACR 个人版、SBOM/Provenance、digest 晋级与环境审批。
- 中文生产操作、回滚、恢复、安全与故障排查文档。

## 后续演进

- 真实环境容量压测后调整副本、连接池、资源与 HPA。
- 接入组织统一策略引擎、镜像签名验证和集中 Secret Backend。
- 当个人版 SLA、限流或凭证隔离不再满足需求时，迁移到 ACR 企业版或企业维护的 Harbor。
- 基于 Redis backlog/探测延迟的 Worker 自定义指标扩缩容。
- 跨可用区监控栈、长期指标存储及正式 SLO/错误预算。

## 不在当前范围

自动修复、Automation Runner、多租户、完整业务 RBAC、OAuth/OIDC、复杂 UI、云资产同步和 HTTP/TCP 以外的 Probe。生产数据库不会回退为集群内 StatefulSet。
