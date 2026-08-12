# CloudSentinel 企业 GitOps 交付计划

## 决策基线

- Docker Compose MVP 保留为本地开发、演示和回归基线，不再作为生产部署方式。
- 生产运行 CloudSentinel API 与 Worker；MySQL、Redis 分别使用集群外托管 RDS、Tair，不提供数据库 StatefulSet 备选路径。
- 应用源码仓库负责代码、镜像构建与 CI；独立 GitOps 配置仓库负责环境期望状态，Argo CD 只读取配置仓库。
- GitHub Actions 只构建、扫描、推送不可变镜像并创建 GitOps 变更 PR；不得直接持有集群管理员凭证或执行 `kubectl apply`。
- Staging 在 GitOps PR 合并后自动同步；Production 通过显式手工确认、独立晋级 PR 和 Argo CD 手工同步完成门禁。GitHub Free 私有仓库不依赖付费 Environment 审批。
- 数据库 Migration 是 Argo CD PreSync Hook，成功后才能更新 API/Worker。
- Secret 只通过 ExternalSecret 契约引用；Git 中不保存 Secret、云访问密钥、Registry 密码或完整数据库 URL。

## Phase G0 - 边界、计划与事实来源

- 状态：PASS
- 交付：本计划、Agent 范围更新、仓库边界与生产依赖决策。
- 完成条件：README、架构、路线图和 GitOps 手册采用同一套结论。

## Phase G1 - 多副本运行时准备

- 状态：PASS
- 交付：Worker 存活/就绪端点；Bootstrap 并发启动幂等保护；对应单元测试。
- 验证：`gofmt`、`go vet ./...`、`go test ./...`、API/Worker 构建。

## Phase G2 - Kustomize 工作负载基线

- 状态：PASS
- 交付：API/Worker Deployment、Service、PDB、资源限制、安全上下文、探针、NetworkPolicy、Migration Job、Staging/Production Overlay。
- 完成条件：所有工作负载使用不可变镜像 digest；不包含 MySQL/Redis 工作负载；生产入口、命名空间和副本策略明确。

## Phase G3 - Secret 与外部依赖契约

- 状态：PASS
- 交付：ExternalSecret 契约、RDS/Tair 连接键、镜像拉取凭证契约、密钥轮换说明。
- 完成条件：静态扫描不存在真实 Secret；占位符在部署前检查中会阻断发布。

## Phase G4 - Argo CD 控制面

- 状态：PASS
- 交付：受限 AppProject、ApplicationSet、Staging 自动同步、Production 受控同步、漂移修复与清理策略。
- 完成条件：源仓库、目标集群、命名空间和允许资源有明确边界。

## Phase G5 - GitHub Actions 供应链与晋级

- 状态：PASS
- 交付：Go/清单 CI，北京 ACR 个人版镜像发布，BuildKit SBOM/Provenance，Staging GitOps PR，Production 晋级 PR。
- 完成条件：工作流不使用云 AccessKey；个人版固定 Registry 凭证仅存在于 GitHub Repository Secrets；不直接连接 Kubernetes API；镜像按 digest 晋级。

## Phase G6 - 企业运行文档

- 状态：PASS
- 交付：中文架构、GitOps 引导、生产部署、密钥与网络、回滚/恢复、故障排查、职责边界与日常操作手册。
- 完成条件：文档命令默认只读或生成变更；破坏性步骤包含前置检查、影响和回退路径。

## Phase G7 - 验证与交付审计

- 状态：PARTIAL（本地静态与代码验证 PASS；真实基础设施 NOT VERIFIED）
- 验证：Go 全套检查、Docker 构建、Kustomize Render、YAML/占位符/Secret 扫描、Git Diff/Status 审计。
- 集群、Argo CD、RDS、Tair、ACR 与 GitHub 组织策略验证必须在真实环境执行；当前无外部授权时统一记录为 `NOT VERIFIED`。

## 最终验证记录（2026-08-12）

- PASS：`gofmt` 无差异、`go mod verify`、`go vet ./...`、`go test -count=1 ./...`。
- PASS：API 与 Worker 使用 `-buildvcs=false -trimpath` 完成本地构建。
- PASS：Node 内置测试完成 digest 更新/晋级脚本的成功与非法 digest 拒绝场景。
- PASS：Staging、Production、两个 Secret Overlay 与 Argo CD Bootstrap 均可通过 kubectl 内置 Kustomize v5.8.1 渲染；应用渲染不包含 `Secret` 或 `StatefulSet`。
- PASS：`docker compose config --quiet`；仅出现当前用户 Docker 配置文件不可读警告，不影响 Compose 语义验证。
- PASS：actionlint v1.7.8 检查源码仓库与 GitOps 种子中的全部 GitHub Actions 工作流。
- NOT VERIFIED：Docker 守护进程未运行，API/Worker/Migration 镜像未做本轮实际构建。
- NOT VERIFIED：没有连接真实 Kubernetes、Argo CD、GitHub 组织、ACR、RDS、Tair、DNS、Ingress、cert-manager 或 External Secrets 后端，未产生任何实际部署结果。

## ACR 个人版修订（2026-08-12）

- 企业版 OIDC/临时 Token 方案因成本调整为北京 ACR 个人版固定 Registry 凭证，实施与验收标准见 `002-acr-personal-edition-migration.md`。
- 个人版无 SLA、共享限流和固定凭证风险是已知边界，不改变 digest 晋级、GitOps 分仓或 Argo CD 拉取模型。
- GitHub Free 私有仓库不支持 Environment Secrets、Required Reviewers 或受保护分支；零额外套餐成本基线使用 Repository Secrets、显式 `PROMOTE` 确认、PR 留痕和 Production Argo CD 手工同步。升级 GitHub Pro 后可再启用私有分支保护，但私有环境 Required Reviewers 仍不作为当前基线。
