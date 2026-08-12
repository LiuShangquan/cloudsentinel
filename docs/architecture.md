# 架构设计

## 业务架构

CloudSentinel 是一个 Go Module 和模块化单体，生产部署为 API 与 Worker 两类无状态进程。MySQL 是事实来源；Redis Streams 提供 at-least-once 消息投递。Worker 不创建 Incident，Prometheus 判断故障，Alertmanager 路由，API 的 Incident Service 处理 Webhook。

```text
客户端 -> Ingress -> API Deployment -> RDS MySQL
                                  \-> Tair Redis（连接与就绪检查）

RDS 到期任务 -> Worker Scheduler -> Tair Stream -> 有界 Worker Pool
                                             -> HTTP/TCP Target -> RDS Result

API /metrics ----\
Worker /metrics --> Prometheus -> Alertmanager -> API Machine Webhook -> Incident
```

多个 Worker 副本通过 MySQL `FOR UPDATE SKIP LOCKED`、条件状态迁移、唯一 `execution_id` 和 Redis Consumer Group 协作。调度器先提交 Queued Execution，再发布 Stream；双写中断由 Recovery Dispatcher 补发。Consumer 只有在最终状态落库后 ACK。

## 生产交付架构

源码和期望状态分仓：

| 仓库 | 职责 | 禁止事项 |
|---|---|---|
| `cloudsentinel` | Go、Dockerfile、Migration、测试、发布工作流 | 不保存环境 Secret，不直接部署集群 |
| `cloudsentinel-gitops` | Kustomize Base/Overlay、Argo CD 项目与应用 | 不构建镜像，不保存应用源码或明文 Secret |

发布过程为单向供应链：源码合并后使用 GitHub OIDC 获取短期阿里云身份，推送 API/Worker/Migration 镜像到 ACR Enterprise；工作流用 GitHub App 短期 Token 创建 Staging PR。Staging 通过后，受保护的 Production 工作流复制完全相同的 digest 创建晋级 PR。Argo CD 是唯一向集群收敛期望状态的组件。

## Kubernetes 映射

- API：3 个生产副本、ClusterIP、Ingress、PDB、CPU HPA。
- Worker：3 个生产副本、PDB、固定并发；先以 Redis backlog 与探测延迟做容量规划，再设计基于自定义指标的弹性。
- Migration：独立一次性 PreSync Job，失败会阻断应用同步。
- RDS/Tair：集群外托管服务，不创建 StatefulSet 或 PVC。
- Secret：ExternalSecret 生成运行时 Secret 与 Registry Pull Secret；应用 ServiceAccount 不挂载 API Token。

Base 使用受限安全上下文、资源请求/限制、探针、拓扑分散和默认入口隔离。动态探测目标使 Worker 出站策略具有业务依赖，最终边界必须同时由应用 SSRF 校验、CNI/出口网关和 VPC 防火墙实现，不能只依赖一个静态 NetworkPolicy。

## 可用性与一致性边界

- API/Worker 多副本不代表 RDS/Tair 自动满足 SLA；必须独立配置多可用区、备份与恢复演练。
- Rolling Update 通过 `maxUnavailable: 0`、PDB 与 Readiness 降低中断，但 Migration 必须采用 Expand/Contract，避免新旧版本并存时不兼容。
- 回滚镜像不会自动回滚数据库。Down Migration 仅在经过数据影响评估和独立备份后执行。
- Staging 自动同步；Production 默认不启用 Argo CD 自动同步，受保护 PR 合并后由发布负责人在变更窗口手动 Sync。

## 安全边界

用户 JWT 与 Alertmanager Machine Token 严格隔离。镜像发布不用长期阿里云 AK；GitOps 写入不用个人 PAT。应用 Pod 为非 Root、只读根文件系统、移除 Linux Capabilities，且无 Kubernetes API 凭证。生产密钥在平台密钥系统中轮换；Git 只记录远端键名。
