# CloudSentinel Agent 指南

## 使命与范围

CloudSentinel 是一个基于 Go 的平台，用于资产登记、定时 HTTP/TCP 探测、监控、告警路由和故障事件生命周期管理。`v0.1.0-compose-mvp` 继续作为本地开发与验收基线；当前批准的交付扩展是在不改变业务边界的前提下，增加面向 Kubernetes 的企业级 GitOps 发布能力。不得借此实现自动修复、多租户、完整 RBAC、OAuth/OIDC、复杂前端或额外的可观测性技术栈。

生产 GitOps 基线采用独立配置仓库、Kustomize、Argo CD 和 GitHub Actions。CloudSentinel API/Worker 运行在 Kubernetes；MySQL 与 Redis 统一使用集群外托管服务（阿里云 RDS 与 Tair），不得在生产清单中创建 MySQL/Redis StatefulSet。数据库变更由独立的 Argo CD PreSync Migration Job 执行。Secret 不得进入 Git，只能由集群密钥控制面注入。

## 架构与技术

本仓库是一个 Go Module（`cloudsentinel`），按模块化单体组织，并部署为两个操作系统进程：

- `cmd/api`：Gin HTTP API、JWT 身份认证、资产/探测任务管理、结果查询、Alertmanager Webhook、故障事件和 API 指标。
- `cmd/worker`：调度器、Redis Streams Consumer Group、有界 Worker Pool、HTTP/TCP 探测、重试、结果持久化和 Worker 指标。

两个进程共享 MySQL 和 Redis，只通过 MySQL 状态与 Redis Streams 通信。必须使用的技术包括：Go、Gin、GORM/MySQL、go-redis/Redis Streams、Docker Compose、Prometheus、Grafana、Alertmanager、`log/slog`、JWT HS256、bcrypt，以及 golang-migrate SQL Migration。

## 包结构与分层规则

代码按业务模块（`auth`、`asset`、`probe`、`incident`）及职责明确的平台包组织。不得创建全局 `handlers`、`services`、`repositories`、`models`，也不得创建包罗万象的 `utils`、`common`、`helpers` 或 `base` 包。

HTTP 调用链为 Router -> Middleware -> Handler -> Service -> Repository -> GORM -> MySQL。

- Router 只负责组合路由、中间件和 Handler。
- Middleware 负责 Request ID、日志、Recovery、JWT、指标和机器身份认证；不得包含业务规则。
- Handler 负责解析并校验 HTTP 输入、调用 Service、映射领域错误和写入统一响应；不得直接访问 GORM/MySQL/Redis。
- Service 接收 `context.Context`，执行业务规则并划定事务边界；不得依赖 Gin、HTTP Writer 或状态码。
- Repository 负责 GORM 持久化和查询；不得依赖 Gin、签发 JWT 或执行网络探测。

所有阻塞调用或外部调用都必须接收 Context。必须保留取消语义并使用有界超时。不得保存请求 Context，也不得把 Context 当作可选参数容器。

## 数据库与消息

- Schema 变更必须使用带版本的 SQL Migration。严禁调用 `AutoMigrate`。
- GORM 和 Readiness 必须使用同一个底层 `sql.DB`；每个进程只能有一套 MySQL 连接池。
- Redis Client 只构造一次，通过依赖注入复用并在退出时关闭；不得使用全局 Client 变量。
- 必须原子完成的资产、故障事件及审计变更应使用同一个数据库事务。
- Redis Streams 采用 at-least-once 投递语义。通过 `execution_id`、数据库唯一约束和条件状态迁移保证幂等。
- Worker 绝不直接创建 Incident。Prometheus 判断故障，Alertmanager 负责路由，Incident Service 处理 Webhook。

## 错误、日志与响应

定义稳定的领域错误，并且只在 Handler 中将其映射为 HTTP。包装错误时应提供运维上下文，但不得泄露 Secret。对外 500 响应固定为 `{"code":50000,"message":"internal server error","data":null}`。

使用 `log/slog`：开发环境输出 Text，其他环境输出 JSON。结构化日志应在相关场景包含 Request ID，但绝不能记录密码、Hash、JWT、Authorization Header、Webhook Token、数据库凭证或完整的含密载荷。Recovery 在内部记录 Panic，但不得向客户端返回 Stack Trace。

## 安全

- 密码使用 bcrypt；JWT 使用 HS256，并校验算法、Issuer 和过期时间。
- JWT 用户认证与 Alertmanager Machine Token 认证必须严格隔离。
- 探测网络策略必须校验 IP 字面量、DNS 解析结果以及每次 Redirect。拒绝元数据地址、Unspecified、Multicast、Link-local，以及未显式允许的 Loopback 地址。
- TLS 证书校验必须保持启用。
- Prometheus Label 必须有界，不得包含 Request/Execution/User ID、用户名、目标 URL、错误消息、时间戳、Token 或客户端 IP。
- Secret 只能来自启动配置，绝不能提交到仓库。`.env.example` 只包含开发占位值。

## 测试与交付

任何大型修改开始前都必须有书面计划。没有测试和真实验证的 Phase 不算完成。如果无法解释调用链，不得进入下一阶段。每个 Go Phase 后都应执行格式化、Vet、测试和构建；存在基础设施时还要执行 Compose、配置和运行时检查。任何无法实际执行的检查必须标记为 `NOT VERIFIED`。

使用显式 `http.Server` 和优雅退出。Worker 并发必须使用有界 Channel、WaitGroup、Context 取消和 Panic Recovery；在数据库完成持久化前不得 ACK。

Docker 镜像必须固定版本、使用多阶段构建和 `CGO_ENABLED=0`，并采用不含源码、`.git` 或 `.env` 的非 Root Runtime。

## Git 规则

允许执行：Status、Diff、Diff Stat、初始化和分支重命名。严禁自动执行 `git add`、Commit、Push、Tag、Hard Reset、Clean、Checkout All 或 Restore All。不得创建或连接 Remote。所有修改保留在工作区中，只能建议 Commit Message。

## 事实来源

保持 `docs/`、README、Migration、配置示例、监控定义和恢复计划与代码同步。Compose 恢复范围由 `.agent/plans/000-recovery-master-plan.md` 定义；企业 GitOps 扩展由 `.agent/plans/001-enterprise-gitops-plan.md` 定义。避免投机性模块和空目录脚手架。
