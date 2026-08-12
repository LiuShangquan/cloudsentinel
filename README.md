# CloudSentinel

CloudSentinel 是一个基于 Go 的基础设施探测与故障事件平台。系统登记 Host/Service，调度 HTTP/TCP 探测，通过 Prometheus 与 Alertmanager 判断故障，并维护 Incident 生命周期。

## 当前交付形态

- 本地开发与回归：Docker Compose MVP（MySQL、Redis、API、Worker、Prometheus、Alertmanager、Grafana）。
- 企业生产：Kubernetes + Kustomize + Argo CD，MySQL/Redis 统一使用集群外阿里云 RDS/Tair。
- 学生练习集群：保持 3 Control Plane + 4 Worker 拓扑，MySQL/Redis 以单副本 StatefulSet 固定运行在 `worker-data-01`，使用 Retain 本地 PV 和定时逻辑备份；这是低成本单点方案，不具备生产高可用性。
- CI/CD：GitHub Actions 构建三个不可变镜像，推送到北京 ACR 个人版并创建独立 GitOps 仓库 PR；工作流不直接操作 Kubernetes。
- 数据库变更：版本化 SQL Migration；生产由 Argo CD PreSync Job 在应用滚动更新前执行。
- 密钥：Git 只保存 ExternalSecret 引用，实际值由平台密钥控制面提供。

```text
开发者 -> 源码 PR -> CI -> main
                         |
                         v
             ACR 固定凭证 -> ACR 个人版公网端点
                    （API/Worker/Migration digest）
                         |
                         v
                  Staging GitOps PR
                         |
                         v
               Argo CD -> Kubernetes -> RDS/Tair（企业）
                                      -> StatefulSet（学生实验）
                         |
              手工 PROMOTE + Production PR
```

业务运行链路保持不变：

```text
客户端 -> API -> MySQL
MySQL -> 调度器 -> Redis Stream -> Worker Pool -> HTTP/TCP 目标
API /metrics + Worker /metrics -> Prometheus -> Alertmanager -> API Webhook -> Incident
```

## 仓库导航

- `cmd/api`、`cmd/worker`：两个生产进程。
- `internal/auth|asset|probe|incident`：领域模块。
- `migrations`：唯一 Schema 事实来源。
- `deploy/gitops-repository`：需要拆分成独立私有 GitOps 仓库的种子。
- `.github/workflows`：CI、镜像发布、Staging 与 Production 晋级流程。
- `docs/gitops.md`：企业 GitOps 架构、接入与日常发布。
- `docs/operations.md`：运行、回滚、Migration 和故障处置。
- `docs/kubernetes`：自建 Kubernetes 集群与网络手册。

## 本地启动

将 `.env.example` 复制为 `.env`，仅在本地使用开发占位凭证：

```powershell
docker compose up -d --build
docker compose ps
$login = Invoke-RestMethod -Method Post -ContentType 'application/json' `
  -Body '{"username":"admin","password":"development-admin-password"}' `
  http://127.0.0.1:8080/api/v1/auth/login
```

API、Prometheus、Alertmanager、Grafana 分别位于 `8080`、`9090`、`9093`、`3000` 端口。完整演示参见 [docs/demo.md](docs/demo.md)。停止但保留数据使用：

```powershell
docker compose --profile demo down
```

警告：`docker compose down -v` 会永久删除本地 Named Volume 数据。

## 开发检查

```powershell
go mod verify
go fmt ./...
go vet ./...
go test ./...
go build ./cmd/api
go build ./cmd/worker
docker compose config
```

GitOps 模板故意包含 `REPLACE_*`，复制为正式配置仓库后必须替换并通过该仓库的 `validate-gitops` 检查。不得把 `.env`、JWT、数据库 URL、Webhook Token、ACR 密码或 Kubeconfig 提交到 Git。

当前低成本镜像基线使用北京 ACR 个人版：GitHub Actions 从公网端点推送，Kubernetes 从同实例 VPC 端点按 digest 拉取。个人版无生产 SLA且可能共享限流；正式上线前必须在真实节点验证拉取、滚动发布和凭证轮换，容量或可用性不足时迁移到 ACR 企业版或组织维护的 Harbor。

当前 GitHub Free 私有仓库基线不依赖 Environment Secrets、Required Reviewers 或私有分支保护：固定 ACR 凭证存放在 Repository Secrets；生产晋级要求手工输入 `PROMOTE`、创建 GitOps PR，并在合并后由操作者手工执行 Argo CD Sync。该方案保留审计链和误操作阻断，但不等价于强制双人审批；升级 GitHub Pro 后应立即启用私有分支保护。

## 生产边界

企业生产 Overlay 不包含 MySQL/Redis StatefulSet。学生实验 Overlay 仅用于本次低成本自建集群，单个数据节点或系统盘故障会同时中断数据库、消息流和本地备份。正式上线时必须迁回 RDS/Tair 或经验证的高可用数据平台。当前仍不实现自动修复、多租户、完整业务 RBAC、OAuth/OIDC、复杂前端或 HTTP/TCP 以外的 Probe。
