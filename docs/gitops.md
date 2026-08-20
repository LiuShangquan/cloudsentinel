# 企业 GitOps 部署手册

## 1. 最终模型

CloudSentinel 采用“源码仓库 + 独立 GitOps 仓库 + Argo CD 拉取”的模型。CI 生成工件和变更提案，CD 控制器从 Git 拉取期望状态；GitHub Actions 不接触 Kubeconfig，也不执行 `kubectl apply`。

```text
Source main -> Repository Secrets 固定凭证 -> ACR 个人版公网端点
                                             -> 镜像 digest
                                             -> Staging PR
                                             -> Argo CD 自动同步
                                             -> 观测与验收
                                             -> 手工输入 PROMOTE
                                             -> Production PR
                                             -> 人工 Argo CD Sync
```

仓库中的 `deploy/gitops-repository` 是初始化种子，启用时必须复制到独立私有仓库。复制完成后，发布脚本继续来自源码仓库，Kustomize 配置以独立仓库为事实来源。

## 2. 平台前置条件

平台团队应先在目标集群提供：Argo CD/ApplicationSet、ingress-nginx、cert-manager、External Secrets Operator、Metrics Server 与 Prometheus。CRD 和 ClusterRole 属于平台仓库，CloudSentinel 业务 AppProject 无权安装它们；当前学生集群使用独立的 `cloudsentinel-platform` AppProject 管理受限的平台资源。

当前学生集群先安装 External Secrets Operator `v2.8.0`；官方兼容矩阵覆盖 Kubernetes `1.35-1.36`，与集群 `v1.35.7` 对齐。由于北京节点不能稳定访问 GHCR，必须先运行 `mirror-external-secrets-image`，把唯一的 `linux/amd64` 上游镜像同步到私有仓库 `cloudsentinel-external-secrets`。工作流会校验官方发布清单 SHA256，并生成按 ACR VPC digest 固定的离线安装 Artifact；不得让节点直接套用上游公网安装命令。

Argo CD 固定为 `v3.5.0`，官方测试矩阵覆盖 Kubernetes `v1.35`。当前学生集群采用官方非 HA `install.yaml`，包含单副本 Application Controller、ApplicationSet、Repo Server、API/UI、Dex、Notifications 和 Argo CD 控制面 Redis；这适合练习与演示，不是生产高可用方案。`mirror-argocd-images` 会把 Argo CD、Dex 和 Redis 三类镜像同步到 ACR，并以官方清单 SHA256 与三个 ACR digest 生成离线 Artifact。安装脚本先通过现有 `ClusterSecretStore` 物化 `argocd/platform-acr-registry`，再执行 Server-Side Dry Run 和安装；它不会接入 Git 仓库或自动部署业务资源。

还需准备：

- 北京 ACR 个人版实例 `crpi-1s64ln3ptbvgkqof`，命名空间 `cloudsentinel0306`，至少包含业务、数据和平台使用的私有 Repository：`cloudsentinel-api`、`cloudsentinel-worker`、`cloudsentinel-migrate`、`cloudsentinel-mysql`、`cloudsentinel-redis`、`cloudsentinel-external-secrets`、`cloudsentinel-prometheus`、`cloudsentinel-alertmanager`、`cloudsentinel-grafana`、`cloudsentinel-metrics-server`；
- 当前学生集群使用 `worker-data-01` 上的单副本 MySQL/Redis StatefulSet；正式企业生产切换为 RDS/Tair 后再准备私网 Endpoint、多可用区和托管备份；
- 只安装在 GitOps 仓库的 GitHub App，权限限定为 Contents 读写、Pull Requests 读写和 Workflows 读写；Workflows 权限仅用于在监控平台 PR 中同步 GitOps 校验工作流，GitHub App 仍不允许绕过 PR 或直接合并 `main`；
- GitHub Repository Secrets：`ACR_USERNAME`、`ACR_PASSWORD`、`GITOPS_APP_PRIVATE_KEY`；
- GitHub 托管 `ubuntu-24.04` Runner；发布工作流不依赖阿里云 AccessKey、OIDC、Kubeconfig 或自托管 Runner。

源码仓库需要配置 Repository Secrets `ACR_USERNAME`、`ACR_PASSWORD`，并配置 Repository Variables `RELEASE_ALLOWED_OWNER=LiuShangquan`、`RELEASE_ALLOWED_REPOSITORY=cloudsentinel`；工作流在登录 Registry 前核对 Owner、Repository 和 `main` Ref，避免仓库转移或复用时沿用发布权限。

ACR 个人版不提供企业版临时登录流程。固定 Registry 密码不得放在 Repository Variable、源码、GitOps 或日志中；必须使用 Repository Secret 并建立轮换记录。GitHub Free 私有仓库不提供 Environment Secrets、Required Reviewers 或受保护分支，因此当前零成本门禁是：仅从 `main` 手工触发、输入精确确认词 `PROMOTE`、GitOps PR 留痕、CODEOWNERS 责任标记以及 Production Argo CD 手工同步。它可以阻止一部分误操作，但不等价于强制双人审批。个人版无生产 SLA且共享带宽，正式上线前必须验证真实节点 VPC 拉取和滚动发布；不满足要求时迁移到 ACR 企业版或企业维护的 Harbor。

## 3. 初始化 GitOps 仓库

以下步骤会创建新的远端仓库状态，必须由仓库管理员在确认组织名、目标仓库和分支保护后手工执行。当前项目不会代替操作者执行。

1. 创建私有 `cloudsentinel-gitops`，把 `deploy/gitops-repository` 的内容作为仓库根目录。
2. 替换当前启用路径中的 `REPLACE_*`、ExternalSecret 远端键名和首批镜像 digest；ACR 地址、命名空间、CODEOWNERS 与 Argo CD Repo URL 已固定为本项目当前资源。休眠的企业 RDS/Tair Overlay 可继续保留占位符。
3. 运行 GitOps 仓库的 `validate-gitops`，确认 `lab-*`、`bootstrap`、实验数据层及已启用 Secret Overlay 不含占位符，再渲染所有当前启用的 Overlay；不要用整个仓库必须零占位符的检查误判休眠模板。
4. GitHub Free 私有仓库无法启用分支保护；团队约定禁止 Direct Push，所有变更均使用 PR，并在合并前人工确认 `validate-gitops` 成功。升级 GitHub Pro 后立即启用 `main` 分支保护、必需状态检查、对话解决、线性历史和 CODEOWNERS。
5. Production 目录保留平台和应用双重审阅要求，但在 GitHub Free 私有仓库中这是流程约束而非平台强制；GitHub App 只创建分支和 PR，不直接更新 `main`。
6. 将独立仓库以只读 Deploy Key 或 GitHub App 接入 Argo CD，然后由平台管理员应用 `bootstrap/argocd`。使用 Deploy Key 时必须关闭写权限，并统一使用 SSH 地址 `git@github.com:LiuShangquan/cloudsentinel-gitops.git`；不得把私钥写入 Git。

Argo CD 项目允许 Staging、Production 和 `cloudsentinel-data` 三个 Namespace，以及数据层所需的 PV/StorageClass/StatefulSet/CronJob。独立 Secret ApplicationSet 先物化 ExternalSecret，并为生成的 Application 启用 `ServerSideDiff=true`，避免 External Secrets Webhook 默认化字段形成持续漂移；数据 Application 自动 Self-Heal 但禁用 Prune，也不配置资源级联删除 Finalizer；Staging 应用开启 Prune/Self-Heal；Production 应用默认需要人工 Sync。

学生集群保留上述同步语义，但受三个 2 GiB Control Plane 的容量约束，Argo CD Application Controller 状态处理器、操作处理器、`kubectl` 并发和 Repo Server 清单生成并发均固定为 `1`。这是容量限流而不是功能降级；扩大应用规模或迁移到企业生产集群时必须重新压测后调整。

## 4. ExternalSecret 数据契约

`cloudsentinel-runtime` 的远端对象必须包含：

| 键 | 用途 |
|---|---|
| `MYSQL_HOST/PORT/DATABASE/USER/PASSWORD` | 当前指向 `mysql.cloudsentinel-data.svc.cluster.local`；企业 Overlay 指向 RDS DNS Endpoint |
| `REDIS_HOST/PORT/PASSWORD` | 当前指向 `redis.cloudsentinel-data.svc.cluster.local`；企业 Overlay 指向 Tair DNS Endpoint |
| `JWT_SECRET` | HS256 签名，至少 32 字节随机值 |
| `ALERTMANAGER_WEBHOOK_TOKEN` | 与 JWT 完全不同的机器 Token |
| `BOOTSTRAP_USER_USERNAME/PASSWORD` | 首次初始化；不会自动轮换已有用户密码 |
| `MIGRATION_DATABASE_URL` | URL 编码后的独立 Migration 账户连接串 |

Migration 账户拥有 Schema DDL 权限；运行时账户只拥有业务 DML 权限。企业 Overlay 的 `MIGRATION_DATABASE_URL` 需要 URL 编码凭证并设置 MySQL TLS 参数。学生 `lab-*` Overlay 将数据密码限制为安全字符集并关闭集群内 MySQL/Redis TLS；该例外只适用于受控 VPC 与 NetworkPolicy 边界。

Registry 远端对象提供完整 `.dockerconfigjson`，其中认证服务器必须是集群拉取所用的 VPC 域名：

```json
{"auths":{"crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com":{"username":"nick9198860113","password":"<从密钥后端注入>","auth":"<base64(username:password)>"}}}
```

该 JSON 是结构示例，真实密码和 `auth` 不得写入 Git。密钥后端和 `ClusterSecretStore` 由平台决定；应用仓库不绑定已经处于不维护状态的云 Provider 实现。

学生集群的 Kubernetes Provider Secret Store 由 `bootstrap/secret-store` 提供。源 Namespace 只保存五个受控 Secret：`cloudsentinel-data-credentials`、`cloudsentinel-staging-app`、`cloudsentinel-production-app`、`cloudsentinel-registry`、`cloudsentinel-monitoring`。监控源 Secret 只包含独立 Grafana 管理凭证和从既有 Staging 源 Secret 安全复制的 Alertmanager Webhook Token。数据键名与启用顺序见[实验 StatefulSet 数据层](kubernetes/lab-stateful-data.md)，监控流程见[学生集群轻量监控平台](kubernetes/lab-monitoring-platform.md)。

## 5. 正常发布

首次业务发布前，先在 ACR 控制台创建 `cloudsentinel-mysql` 和 `cloudsentinel-redis` 两个私有仓库，再从源码仓库 `main` 手工运行 `mirror-lab-data-images`。它只同步当前 x86 ECS 需要的 `linux/amd64` 固定上游版本，直接采用 ACR Push 返回的 digest，并在该 digest 与 GitOps 不一致时创建数据镜像 PR。数据层健康后再开始下面的业务镜像发布。

1. Source PR 通过格式化、Vet、单测、构建和 Kustomize Render。
2. 合并 `main` 触发 `release-images-and-stage`；GitHub 托管 Runner 使用 Repository Secrets 中的固定凭证登录 ACR 个人版公网端点，BuildKit 推送三个固定 `linux/amd64` 镜像。同一分支重复触发时只保留最新运行，旧的排队或执行中发布会被取消；镜像标签不可变，只有三个构建全部成功后才创建 GitOps PR。当前 ACR 个人版实测拒绝 BuildKit 附着式 SBOM/Provenance 使用的 OCI Attestation Manifest（`application/vnd.oci.empty.v1+json`），因此发布工作流显式设置 `provenance: false` 与 `sbom: false`；镜像仍按 Registry 返回的不可变 Digest 晋级，但这不等价于完整供应链证明。SBOM/Provenance 必须在独立 Artifact 流程实现，或在迁移到支持 OCI Referrers/Attestations 的 Registry 后重新附着。
3. 工作流取得每个镜像 digest，把镜像名称转换为同实例的北京 VPC 端点并创建 Staging GitOps PR。审核并合并后 Argo CD 自动运行 Migration，再滚动 API/Worker。
4. 验证 Staging Migration、Pod Ready、`/readyz`、Probe 结果、Incident Webhook、错误率和延迟。
5. 从源码仓库 `main` 手工触发 `promote-production` 并输入 `PROMOTE`。工作流把 Staging 的完全相同 digest 复制到 Production 并创建 PR；至少由另一名负责人审阅，但 GitHub Free 私有仓库不会强制该审批。
6. 在变更窗口合并 Production PR；发布负责人确认 Argo Diff 和最近一次 MySQL/Redis 逻辑备份后执行 Argo CD Sync。

不要重新构建 Production 镜像；晋级的是已经测试的 digest。

## 6. 首次同步检查

同步前确认集群和 ACR 均位于 `cn-beijing`，数据节点目录已准备、所有 `ExternalSecret` Ready、ACR VPC Pull Secret 有效、两个 StatefulSet Ready、Bootstrap Job 成功且逻辑备份可写。当前 `lab-*` Overlay 不含 Ingress，因此不需要先准备域名。Migration 是 PreSync Hook，会早于应用的普通 ServiceAccount 创建；它因此使用 Namespace 自动生成的 `default` ServiceAccount、保持 `automountServiceAccountToken: false`，并只在 Pod 上显式引用 ACR Pull Secret。PreSync Job 失败时 Argo CD 不会更新应用；先检查 Job 退出码、MySQL 账户和数据层健康，不要跳过 Hook 手工滚动 Deployment。

真实 ACR VPC 镜像拉取、External Secrets `v2.8.0`、Kubernetes Provider `ClusterSecretStore`、受控源 Secret、Argo CD `v3.5.0` 安装和 GitOps 仓库只读接入已在 2026-08-15 验证。三个 Secret Application 已成功物化 ExternalSecret，并通过 Server-Side Diff 达到 `Synced/Healthy`。实验数据层的 MySQL/Redis StatefulSet、Local PV/PVC、认证连接和 Argo CD 收敛已在 2026-08-15 验证；备份 CronJob 已创建，但实际备份与恢复演练仍为 `NOT VERIFIED`。Staging 于 2026-08-16 完成 Migration、API/Worker 2 副本、内部健康检查、JWT 身份认证及真实 Host/Service/Probe Task 到 HTTP 200 Probe Result 的业务 E2E 验收，测试业务记录已禁用，临时 Namespace 已删除。轻量监控平台、Production、Ingress 和证书仍为 `NOT VERIFIED`，必须按顺序取得运行时证据后再更新状态。
