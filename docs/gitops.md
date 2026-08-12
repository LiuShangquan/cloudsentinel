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

平台团队应先在目标集群提供：Argo CD/ApplicationSet、ingress-nginx、cert-manager、External Secrets Operator、Metrics Server 与 Prometheus。CRD 和 ClusterRole 属于平台仓库，CloudSentinel AppProject 无权安装它们。

还需准备：

- 北京 ACR 个人版实例 `crpi-1s64ln3ptbvgkqof`，命名空间 `cloudsentinel0306`，三个私有 Repository：`cloudsentinel-api`、`cloudsentinel-worker`、`cloudsentinel-migrate`；
- RDS MySQL 与 Tair Redis 的私网 Endpoint、独立应用账户、备份和多可用区策略；
- 只安装在 GitOps 仓库的 GitHub App，权限限定为 Contents 读写和 Pull Requests 读写；
- GitHub Repository Secrets：`ACR_USERNAME`、`ACR_PASSWORD`、`GITOPS_APP_PRIVATE_KEY`；
- GitHub 托管 `ubuntu-24.04` Runner；发布工作流不依赖阿里云 AccessKey、OIDC、Kubeconfig 或自托管 Runner。

源码仓库需要配置 Repository Secrets `ACR_USERNAME`、`ACR_PASSWORD`，并配置 Repository Variables `RELEASE_ALLOWED_OWNER=LiuShangquan`、`RELEASE_ALLOWED_REPOSITORY=cloudsentinel`；工作流在登录 Registry 前核对 Owner、Repository 和 `main` Ref，避免仓库转移或复用时沿用发布权限。

ACR 个人版不提供企业版临时登录流程。固定 Registry 密码不得放在 Repository Variable、源码、GitOps 或日志中；必须使用 Repository Secret 并建立轮换记录。GitHub Free 私有仓库不提供 Environment Secrets、Required Reviewers 或受保护分支，因此当前零成本门禁是：仅从 `main` 手工触发、输入精确确认词 `PROMOTE`、GitOps PR 留痕、CODEOWNERS 责任标记以及 Production Argo CD 手工同步。它可以阻止一部分误操作，但不等价于强制双人审批。个人版无生产 SLA且共享带宽，正式上线前必须验证真实节点 VPC 拉取和滚动发布；不满足要求时迁移到 ACR 企业版或企业维护的 Harbor。

## 3. 初始化 GitOps 仓库

以下步骤会创建新的远端仓库状态，必须由仓库管理员在确认组织名、目标仓库和分支保护后手工执行。当前项目不会代替操作者执行。

1. 创建私有 `cloudsentinel-gitops`，把 `deploy/gitops-repository` 的内容作为仓库根目录。
2. 替换所有 `REPLACE_*`、域名、ExternalSecret 远端键名和首批镜像 digest；ACR 地址、命名空间、CODEOWNERS 与 Argo CD Repo URL 已固定为本项目当前资源。
3. 先运行 `rg -n "REPLACE_" .`，结果必须为空；再渲染两个 Overlay。
4. GitHub Free 私有仓库无法启用分支保护；团队约定禁止 Direct Push，所有变更均使用 PR，并在合并前人工确认 `validate-gitops` 成功。升级 GitHub Pro 后立即启用 `main` 分支保护、必需状态检查、对话解决、线性历史和 CODEOWNERS。
5. Production 目录保留平台和应用双重审阅要求，但在 GitHub Free 私有仓库中这是流程约束而非平台强制；GitHub App 只创建分支和 PR，不直接更新 `main`。
6. 将独立仓库以只读 Deploy Key 或 GitHub App 接入 Argo CD，然后由平台管理员应用 `bootstrap/argocd`。

Argo CD 项目只允许两个 CloudSentinel Namespace 和清单所需资源种类。独立 Secret ApplicationSet 先物化 ExternalSecret；Staging 应用开启 Prune/Self-Heal；Production 应用保留自动漂移检测，但默认需要人工 Sync。

## 4. ExternalSecret 数据契约

`cloudsentinel-runtime` 的远端对象必须包含：

| 键 | 用途 |
|---|---|
| `MYSQL_HOST/PORT/DATABASE/USER/PASSWORD` | RDS 应用连接，Host 必须使用证书覆盖的 DNS Endpoint |
| `REDIS_HOST/PORT/PASSWORD` | Tair TLS 连接，Host 必须使用证书覆盖的 DNS Endpoint |
| `JWT_SECRET` | HS256 签名，至少 32 字节随机值 |
| `ALERTMANAGER_WEBHOOK_TOKEN` | 与 JWT 完全不同的机器 Token |
| `BOOTSTRAP_USER_USERNAME/PASSWORD` | 首次初始化；不会自动轮换已有用户密码 |
| `MIGRATION_DATABASE_URL` | URL 编码后的独立 Migration 账户连接串 |

Migration 账户拥有 Schema DDL 权限；运行时账户只拥有业务 DML 权限。`MIGRATION_DATABASE_URL` 需要 URL 编码凭证并设置 MySQL TLS 参数。GitOps ConfigMap 默认启用 `MYSQL_TLS_ENABLED` 与 `REDIS_TLS_ENABLED`，代码保持证书与主机名校验，不提供跳过验证选项。

Registry 远端对象提供完整 `.dockerconfigjson`，其中认证服务器必须是集群拉取所用的 VPC 域名：

```json
{"auths":{"crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com":{"username":"nick9198860113","password":"<从密钥后端注入>","auth":"<base64(username:password)>"}}}
```

该 JSON 是结构示例，真实密码和 `auth` 不得写入 Git。密钥后端和 `ClusterSecretStore` 由平台决定；应用仓库不绑定已经处于不维护状态的云 Provider 实现。

## 5. 正常发布

1. Source PR 通过格式化、Vet、单测、构建和 Kustomize Render。
2. 合并 `main` 触发 `release-images-and-stage`；GitHub 托管 Runner 使用 Repository Secrets 中的固定凭证登录 ACR 个人版公网端点，BuildKit 推送三个带 SBOM/Provenance 的镜像。同一分支重复触发时只保留最新运行，旧的排队或执行中发布会被取消；镜像标签不可变，只有三个构建全部成功后才创建 GitOps PR。
3. 工作流取得每个镜像 digest，把镜像名称转换为同实例的北京 VPC 端点并创建 Staging GitOps PR。审核并合并后 Argo CD 自动运行 Migration，再滚动 API/Worker。
4. 验证 Staging Migration、Pod Ready、`/readyz`、Probe 结果、Incident Webhook、错误率和延迟。
5. 从源码仓库 `main` 手工触发 `promote-production` 并输入 `PROMOTE`。工作流把 Staging 的完全相同 digest 复制到 Production 并创建 PR；至少由另一名负责人审阅，但 GitHub Free 私有仓库不会强制该审批。
6. 在变更窗口合并 Production PR；发布负责人确认 Argo Diff 和 RDS 备份点后执行 Argo CD Sync。

不要重新构建 Production 镜像；晋级的是已经测试的 digest。

## 6. 首次同步检查

同步前确认集群和 ACR 均位于 `cn-beijing`，`ExternalSecret` 为 Ready、目标 Secret 键完整、RDS/Tair 私网连通、Ingress 证书可签发、ACR VPC Pull Secret 有效、RDS 当前备份点成功。PreSync Job 失败时 Argo CD 不会更新应用；先检查 Job 退出码与 RDS 权限，不要跳过 Hook 手工滚动 Deployment。

集群与云环境尚未在本地仓库验证，因此真实 Argo CD、RDS、Tair、ACR、Ingress、证书与 ExternalSecret 结果均为 `NOT VERIFIED`。
