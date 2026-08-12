# CloudSentinel 恢复总计划

## 不变量

- 只有一个 Go Module；API 与 Worker 是相互独立的操作系统进程。
- MySQL 是事实来源；Redis Streams 使用 at-least-once 投递语义。
- 禁止使用 AutoMigrate、提交 Secret、创建无界 Worker Goroutine、由 Worker 创建 Incident，或实现 MVP 范围外的投机性模块。
- 每个 Phase 必须在进入下一阶段前完成实现、测试、修复、文档同步和结果报告。

## Phase 0 - 项目上下文与文档

- 状态：PASS
- 已实现：Git 仓库与 `main` 分支；Agent 指南；需求、架构、数据库、API、监控、路线图和计划文档。
- 测试：必需文件清单、必需规范标题、Git `main` 状态和文档一致性检查均已通过。
- 问题：无。
- 修复：无。
- 剩余：无。

## Phase 1 - Go API 骨架

- 状态：PASS
- 已实现：类型化启动配置、slog Factory、Gin Router、Request ID/Access Log/Recovery Middleware、统一响应、Health/Readiness Handler、显式 HTTP Server、信号退出、Makefile、Dockerfile、环境变量示例和基础测试。
- 测试：`go mod tidy`、`go mod verify`、`go fmt ./...`、`go vet ./...`、`go test ./...` 和 API 构建均已通过。
- 问题：最初放置统一响应代码的位置造成 health/router 和 middleware/router Import Cycle；执行环境中的官方 Go Proxy 超时。
- 修复：将响应辅助函数移动到叶子子包，并通过已配置的区域 Go Proxy 下载经过 Checksum 校验的 Module。
- 剩余：无。

## Phase 2 - MySQL、Redis 与 Docker Compose

- 状态：PASS
- 已实现：GORM/MySQL 单连接池生命周期、注入式 Redis Client、依赖 Readiness Checker、带版本的平台 Migration、启用 AOF 的 Redis、持久化 MySQL/Redis Volume，以及四服务 Compose 启动顺序。
- 测试：Go fmt/vet/test/build、Compose config/build/up/ps/logs、Migration 成功、基础 Readiness、MySQL 停机/恢复和 Redis 停机/恢复均已通过。
- 问题：无法访问 Docker Hub；Redis Healthcheck 最初阻止了 Shell 变量展开；一次未提权的故障测试没有真正停止 MySQL，结果已丢弃。
- 修复：仅通过当前可用镜像源填充并重新标记固定版本的本地镜像，修正 Redis Healthcheck 引号，然后在确认容器状态后重新执行真实故障测试。
- 剩余：无。

## Phase 3 - 身份认证

- 状态：PASS
- 已实现：User/Audit Migration、bcrypt 凭证、严格的 HS256 JWT Manager 与 Middleware、Bootstrap 用户、登录与当前用户 Service/Handler、安全公共 DTO，以及事务化最后登录审计。
- 测试：凭证成功/失败/缺失/禁用场景，正常/错误 Secret/篡改/过期/alg-none JWT，无 Token，`/users/me` 数据库刷新，Bootstrap 校验/幂等，Go 检查/构建，Migration，真实登录，以及 API 重启后使用原密码登录均已通过。
- 问题：JWT/Bootstrap Compose 变量最初错误地配置在 Redis 上；一次无权限的重启操作没有真正重启 Docker，结果已丢弃。
- 修复：将变量移动到 API，重新创建容器，并再次执行已认证运行时检查和有权限的重启检查。
- 剩余：无。

## Phase 4 - 资产管理

- 状态：PASS
- 已实现：Host/Service Migration、经过校验的 Model 与 Target、Repository、支持 Context 的 Service、JWT Handler、分页、软禁用冲突和事务化 Audit Write。
- 测试：地址/HTTP/TCP/分页单元测试，Go 检查/构建，Migration，真实 Host 与 HTTP/TCP Service 创建，重复/非法输入，分页，Active Child 冲突，软禁用，JWT 保护和 Audit Row 校验均已通过。
- 问题：首次运行时脚本使用了 PowerShell 保留变量 `$Host`，导致返回的 ID 丢失，该次尝试中的依赖请求无效。
- 修复：通过 API 重新读取已创建的 Host，并成功重跑所有依赖运行时检查。
- 剩余：无。

## Phase 5 - 探测系统

- 状态：PASS
- 已实现：Task/Result Migration 与 JWT API、到期任务原子调度、Queued Recovery Dispatcher、Redis Streams Consumer Group/AutoClaim、有界 Worker Pool、重试、具备 SSRF 防护的 HTTP/TCP Probe、条件执行权领取、持久最终状态，以及持久化成功后的 ACK。
- 测试：HTTP 状态/Redirect/Timeout/TLS，TCP Open/Closed/Cancel，网络策略，重试策略，有界 Backpressure，Go 检查/构建，Migration，真实 Schedule/Stream/Worker/Result 成功链路，Service/Task 冲突，重复消息幂等/ACK 和 Redis 中断后的 Queued Recovery 均已通过。
- 问题：Windows 的 Connection Refused 文本最初被归类为通用 Network Error。
- 修复：增加跨平台 `syscall.ECONNREFUSED` 分类并重新运行完整测试集。
- 剩余：无；Worker Crash/Pending Claim 已在 Phase 8 最终故障矩阵中完成验证。

## Phase 6 - Prometheus 与 Grafana

- 状态：PASS
- 已实现：独立 API/Worker Registry、使用有界 Label 的请求/Probe/Worker 指标、基于数据库的 Latest Probe 与 Active Incident Collector、Worker Metrics Server、Prometheus Scrape/Rule，以及完整预置的 Grafana Datasource/Dashboard。
- 测试：Go Collector 测试/构建、Label 审查、`promtool` Config/Rule、三个 Prometheus Target 全部 UP、Alertmanager Discovery、Grafana Health、Datasource 和 Dashboard Provisioning 均已通过。
- 问题：无法从 Docker Hub 获取固定版本的 Prometheus/Grafana 镜像。
- 修复：通过当前可用镜像源填充精确固定版本的官方镜像，同时在 Compose 中保留标准镜像名称。
- 剩余：无。

## Phase 7 - Alertmanager 与 Incident

- 状态：PASS
- 已实现：经过认证的 Alertmanager Webhook、稳定 Fingerprint/Event Key 幂等、Firing/Resolved 接收、Incident 状态机与 JWT API、事务化 Audit Write、基于数据库的 Active Metric、Alertmanager 路由、持久化和 Resolved 通知。
- 测试：Machine Token 隔离/缺失/错误/User JWT 场景、重复 Firing/Resolved 幂等、合法/非法状态迁移、审计行为、`amtool`，以及真实 Probe -> Prometheus Pending/Firing -> Alertmanager -> Incident Firing/Resolved 链路均已通过。
- 问题：一次手工 PowerShell Payload 替换误发送了额外 Firing 通知，而不是 Resolved 通知；该问题并非产品缺陷。
- 修复：发送显式 Resolved Payload，随后独立验证真实 Prometheus/Alertmanager Resolved 链路。
- 剩余：无。

## Phase 8 - Docker Compose MVP 验收

- 状态：PASS
- 已实现：固定版本的最终八服务技术栈、持久化 Volume、可选 `mvp-http-target` Demo Profile、README 和可重复演示手册。Demo Target 使用 Go 标准库实现，并运行在固定版本的非 Root Alpine Runtime 中。
- 测试：Tidy/Verify/Fmt/Vet/Unit/Build、Compose Config/Build/Up/Logs、真实 Login-to-Probe E2E、Demo 告警生命周期与 Close、MySQL/Redis/Worker/监控组件停机、Worker Hard Crash Pending Claim、API 与全栈重启、Volume 持久化、Runtime Image、Auth/Webhook、Secret/Cardinality 和最终架构审查均已通过。Race Test 标记为 `NOT VERIFIED`，因为 Windows 主机既未启用 CGO，也没有 C Compiler。
- 问题：首个纯 Alpine Demo Fixture 尝试使用固定镜像中不存在的 BusyBox `httpd` Applet；Grafana 整目录 Provisioning Mount 遮蔽了可选镜像目录，且 Grafana 13 尝试执行不必要的 Bundled Plugin 后台更新；Docker Hub 连接需要通过本地镜像源填充；当前环境不支持 Race Instrumentation。
- 修复：将 Fixture 替换为通过同一固定版本非 Root 多阶段镜像构建的微型 Go 标准库 HTTP 进程；将 Grafana Mount 缩小到 Datasource/Dashboard 子目录并禁用默认 Plugin Preinstallation；Compose 继续使用标准固定镜像名称；Race Test 如实记录为 `NOT VERIFIED`。
- 剩余：无。

