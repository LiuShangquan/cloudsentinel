# 企业 GitOps 部署手册

## 1. 最终模型

CloudSentinel 采用“源码仓库 + 独立 GitOps 仓库 + Argo CD 拉取”的模型。CI 生成工件和变更提案，CD 控制器从 Git 拉取期望状态；GitHub Actions 不接触 Kubeconfig，也不执行 `kubectl apply`。

```text
Source main -> OIDC -> ACR digest -> Staging PR -> Argo CD 自动同步
                                               -> 观测与验收
                                               -> Production 环境审批
                                               -> Production PR
                                               -> 人工 Argo CD Sync
```

仓库中的 `deploy/gitops-repository` 是初始化种子，启用时必须复制到独立私有仓库。复制完成后，发布脚本继续来自源码仓库，Kustomize 配置以独立仓库为事实来源。

## 2. 平台前置条件

平台团队应先在目标集群提供：Argo CD/ApplicationSet、ingress-nginx、cert-manager、External Secrets Operator、Metrics Server 与 Prometheus。CRD 和 ClusterRole 属于平台仓库，CloudSentinel AppProject 无权安装它们。

还需准备：

- ACR Enterprise 三个私有 Repository：`cloudsentinel-api`、`cloudsentinel-worker`、`cloudsentinel-migrate`；
- RDS MySQL 与 Tair Redis 的私网 Endpoint、独立应用账户、备份和多可用区策略；
- GitHub OIDC Provider 与仅允许目标仓库/分支的 RAM Role；
- 只安装在 GitOps 仓库的 GitHub App，权限限定为 Contents 读写和 Pull Requests 读写；
- GitHub Environments：`registry-release` 和带 Required Reviewers 的 `production-approval`；
- 短生命周期、组织管理的 Release Runner，预装固定版本 Docker/Buildx、Alibaba Cloud CLI、jq、gh、Node.js。

`registry-release` 还需配置 `RELEASE_ALLOWED_OWNER` 与 `RELEASE_ALLOWED_REPOSITORY`；工作流在取得云身份前核对 Owner、Repository 和 `main` Ref，避免仓库转移或复用时沿用发布权限。

Release RAM Role 最小权限包括 `cr:GetAuthorizationToken`，以及三条指定 ACR Repository 的 `cr:PushRepository`/`cr:PullRepository`。OIDC 信任条件必须绑定组织、源码仓库、`main` 分支和 GitHub Environment，不能允许任意 Repository。

## 3. 初始化 GitOps 仓库

以下步骤会创建新的远端仓库状态，必须由仓库管理员在确认组织名、目标仓库和分支保护后手工执行。当前项目不会代替操作者执行。

1. 创建私有 `cloudsentinel-gitops`，把 `deploy/gitops-repository` 的内容作为仓库根目录。
2. 替换所有 `REPLACE_*`、域名、ACR 地址、ExternalSecret 远端键名、CODEOWNERS 团队和 Argo CD Repo URL。
3. 先运行 `rg -n "REPLACE_" .`，结果必须为空；再渲染两个 Overlay。
4. 启用 `main` 分支保护：禁止 Direct Push，要求 `validate-gitops`、对话解决、线性历史和 CODEOWNERS。
5. Production 目录要求平台和应用双重审批；GitHub App 不拥有绕过分支保护的权限。
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

Migration 账户拥有 Schema DDL 权限；运行时账户只拥有业务 DML 权限。`MIGRATION_DATABASE_URL` 需要 URL 编码凭证并设置 MySQL TLS 参数。GitOps ConfigMap 默认启用 `MYSQL_TLS_ENABLED` 与 `REDIS_TLS_ENABLED`，代码保持证书与主机名校验，不提供跳过验证选项。Registry 远端对象提供 `.dockerconfigjson`。密钥后端和 `ClusterSecretStore` 由平台决定；应用仓库不绑定已经处于不维护状态的云 Provider 实现。

## 5. 正常发布

1. Source PR 通过格式化、Vet、单测、构建和 Kustomize Render。
2. 合并 `main` 触发 `release-images-and-stage`；OIDC 获取短期 STS，ACR 临时 Token 登录，BuildKit 推送三个带 SBOM/Provenance 的镜像。
3. 工作流取得每个镜像 digest，创建 Staging GitOps PR。审核并合并后 Argo CD 自动运行 Migration，再滚动 API/Worker。
4. 验证 Staging Migration、Pod Ready、`/readyz`、Probe 结果、Incident Webhook、错误率和延迟。
5. 触发 `promote-production`。Required Reviewers 批准后，工作流把 Staging 的完全相同 digest 复制到 Production 并创建 PR。
6. 在变更窗口合并 Production PR；发布负责人确认 Argo Diff 和 RDS 备份点后执行 Argo CD Sync。

不要重新构建 Production 镜像；晋级的是已经测试的 digest。

## 6. 首次同步检查

同步前确认 `ExternalSecret` 为 Ready、目标 Secret 键完整、RDS/Tair 私网连通、Ingress 证书可签发、ACR Pull Secret 有效、RDS 当前备份点成功。PreSync Job 失败时 Argo CD 不会更新应用；先检查 Job 退出码与 RDS 权限，不要跳过 Hook 手工滚动 Deployment。

集群与云环境尚未在本地仓库验证，因此真实 Argo CD、RDS、Tair、ACR、Ingress、证书与 ExternalSecret 结果均为 `NOT VERIFIED`。
