# CloudSentinel 平台控制器引导计划

## 目标

在已经通过基础设施验收的 Kubernetes `v1.35.7` 学生集群中，按可审计、可回退的顺序引入 External Secrets Operator、Argo CD 与后续平台组件。平台镜像必须先由 GitHub Actions 从固定上游版本同步到北京 ACR 个人版，集群只使用 ACR VPC 端点和不可变 digest。

本计划不把任何密码、Token、Docker Config、Kubeconfig 或 GitHub 私钥写入 Git；所有会修改集群的命令仍由操作者确认 Context、目标节点、文件校验值与风险后手工执行。

## 已确认基线

- 3 个 Control Plane、4 个 Kubernetes Worker 均为 `Ready`，Calico、CoreDNS、三成员 stacked etcd 与私网 NLB 已完成验收。
- Kubernetes 固定为 `v1.35.7`，节点架构统一为 `linux/amd64`。
- `worker-data-01` 的根文件系统为 40 GiB ext4，准备前可用约 32 GiB；Local PV 容量门禁已通过。
- `/var/lib/cloudsentinel/mysql`、`redis` 与 `backups` 已在 `worker-data-01` 建立，目录准备结果为 `LOCAL_PV_DIRECTORY_PREPARE=PASS`。
- 集群节点不能稳定访问 GHCR/`registry.k8s.io`，平台镜像必须经 ACR 个人版同步。
- External Secrets Operator 固定为 `v2.8.0`。上游兼容矩阵覆盖 Kubernetes `1.35-1.36`；官方 `external-secrets.yaml` SHA256 固定为 `7d1533a7ace1f0ee9a21ac2215665e8f5856690ca7b224961df2011ff735d572`。

## Phase P0 - External Secrets 供应链

1. 在 ACR 命名空间 `cloudsentinel0306` 手工创建私有仓库 `cloudsentinel-external-secrets`，关闭自动创建仓库。
2. 从源码仓库 `main` 手工运行 `mirror-external-secrets-image`。
3. 工作流只复制 `ghcr.io/external-secrets/external-secrets:v2.8.0` 的 `linux/amd64` 镜像，记录 ACR Push 返回的 digest。
4. 工作流下载官方发布清单并核对固定 SHA256，生成仅使用 ACR VPC digest 的 Kustomize 安装包，作为短期 Actions Artifact 交给操作者。
5. 安装包为审计工件，不包含 Registry 凭据、Kubeconfig 或运行时 Secret。

## Phase P1 - External Secrets 安装

1. 在受控运维终端生成权限为 `0600` 的 ACR Docker Config，不在命令行参数或历史中暴露密码。
2. 创建 `external-secrets` Namespace 与 `platform-acr-registry` Pull Secret，只检查 Secret 类型和键名，不输出内容。
3. 把工作流 Artifact 上传到 `master-01`，核对文件校验值并审计渲染结果。
4. 手工运行 `install-external-secrets.sh --apply`；脚本使用 Server-Side Apply，等待三个 Deployment 可用，并确认它们只调度到 `node-role=app` 节点。
5. 验证 `external-secrets.io/v1` CRD 与 Webhook，再应用 GitOps 仓库的 `bootstrap/secret-store`。

## Phase P2 - Secret 来源

1. 在 `cloudsentinel-secret-source` 中创建四个受控 Secret：数据凭据、Staging 应用凭据、Production 应用凭据与 ACR Pull Secret。
2. 只允许 `cloudsentinel-secret-reader` ServiceAccount 读取明确列出的四个 Secret。
3. 确认 `ClusterSecretStore/cloudsentinel-secret-store` 为 `Ready=True`，然后再允许数据层 Application 同步。
4. 临时凭据文件仅在确认集群 Secret 已创建后，从精确绝对路径安全移除。

## Phase P3 - Argo CD 与数据层

1. 固定 Argo CD/ApplicationSet 为 `v3.5.0`，校验官方非 HA 清单 SHA256，并把 Argo CD、Dex 和控制面 Redis 三类 `linux/amd64` 镜像同步到 ACR。学生集群使用非 HA 拓扑；正式生产必须改用 HA 基线。
2. 以只读凭据接入 `cloudsentinel-gitops`，Production 保持人工 Sync。
3. 先同步 Secret，再同步 `cloudsentinel-data-lab`；确认 3 个 Local PV/PVC 绑定、MySQL/Redis Ready 和 Bootstrap Job 成功。
4. 数据层验收后再同步 Lab Staging；Production 只在 Staging 通过后手工执行。

## 退出与回退边界

- External Secrets 安装失败时停止在平台层，不创建业务 Secret，不同步数据 Application。
- 不删除 CRD、Namespace、PVC、PV 或 Local PV 目录作为故障排查手段。
- ACR digest、上游清单 SHA、Actions Run 与服务器安装日志共同构成变更证据。
- Local PV `Retain` 不等于备份；数据层上线后必须把逻辑备份复制到 `ops-storage`。

## 当前状态

- Phase P0-P2 已在真实集群完成：External Secrets `v2.8.0` 三个 Deployment 可用，Webhook DNS 已修复并固化，`ClusterSecretStore/cloudsentinel-secret-store` 为 `Ready=True`，四个受控源 Secret 已创建且最小权限读取验证通过。
- Phase P3 的 Argo CD `v3.5.0` 镜像同步工作流、校验构建器、离线安装脚本和操作文档已在工作区实现；真实 Actions 镜像同步与集群安装仍为 `NOT VERIFIED`。
- Argo CD 安装成功前，不配置 GitOps 仓库凭据，不应用 `bootstrap/argocd`，也不创建 PV/PVC、MySQL/Redis StatefulSet 或 CloudSentinel 工作负载。
