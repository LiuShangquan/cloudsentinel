# CloudSentinel ACR 个人版迁移计划

## 背景与决定

- 镜像仓库改为阿里云容器镜像服务 ACR 个人版，地域固定为 `cn-beijing`。
- GitHub Actions 通过个人版公网端点推送镜像，Kubernetes 通过同地域 VPC 端点拉取镜像。
- 命名空间固定为 `cloudsentinel0306`，仓库固定为 `cloudsentinel-api`、`cloudsentinel-worker`、`cloudsentinel-migrate`。
- 个人版不支持企业版 `GetAuthorizationToken` 临时登录流程。发布使用 GitHub Repository Secrets 加密保存的固定 Registry 凭证，集群拉取凭证继续由 External Secrets 物化。
- 继续按镜像 Digest 交付，GitHub Actions 不持有 Kubeconfig，也不直接操作 Kubernetes。

## 安全与运行边界

- Registry 密码不得写入源码、GitOps、日志、`.env` 或文档。
- GitHub Free 私有仓库无法使用 Environment Secrets，因此 `ACR_USERNAME`、`ACR_PASSWORD` 仅使用 Repository Secrets；生产部署前必须建立密码轮换记录。
- GitHub Free 私有仓库也不提供 Required Reviewers 或受保护分支。当前零成本门禁由手工触发时输入 `PROMOTE`、GitOps PR 留痕、CODEOWNERS 责任标记和 Production Argo CD 手工同步组成；它不是强制双人审批的等价替代。
- ACR 个人版无生产 SLA、共享带宽并可能限流。滚动发布必须控制并发，镜像使用 `IfNotPresent` 和节点缓存。
- 若实际并发、可用性或审计需求超过个人版能力，迁移到 ACR 企业版或企业维护的 Harbor；不得通过关闭 TLS 或使用匿名镜像规避认证。

## 实施步骤

1. 删除发布工作流中的阿里云 OIDC、RAM Role、企业版 Instance ID/API Endpoint 和自托管 Runner 依赖。
2. 使用 GitHub 托管 Runner，通过 `ACR_USERNAME`、`ACR_PASSWORD` 登录个人版公网端点并推送三个镜像。
3. 生成 Staging PR 时，把相同镜像 Digest 写入个人版 VPC 端点，供集群内拉取。
4. 更新 Kustomize、ExternalSecret 契约、README、架构、GitOps、运维与路线图文档。
5. 执行 Go、Node、GitHub Actions、Compose 和 Kustomize 静态/运行时可执行验证。

## 完成条件

- 工作流中不存在 `GetAuthorizationToken`、阿里云 OIDC、企业版 Instance ID/API Endpoint 或发布自托管 Runner。
- 发布地址为 `crpi-1s64ln3ptbvgkqof.cn-beijing.personal.cr.aliyuncs.com`。
- 部署地址为 `crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com`。
- GitOps 应用镜像均位于 `cloudsentinel0306` 命名空间且继续使用 `sha256` Digest。
- 任何仓库文件中不存在真实 Registry 密码。
- 本地检查通过；真实 GitHub Actions、ACR 推送和 Kubernetes 拉取在外部环境完成前标记为 `NOT VERIFIED`。

## 验证记录（2026-08-12）

- PASS：`gofmt`、`go mod verify`、`go vet ./...`、`go test -count=1 ./...`、API/Worker 构建。
- PASS：`actionlint` v1.7.8、GitOps 镜像脚本测试、`docker compose config --quiet`。
- PASS：Staging、Production 与 Argo CD Bootstrap Kustomize 渲染；两个应用 Overlay 均恰好包含三个北京 ACR VPC Digest 镜像引用。
- PASS：企业版 OIDC、企业版 API 参数、自托管发布 Runner 与硬编码 ACR 密码残留扫描。
- NOT VERIFIED：真实 GitHub 托管 Runner、ACR 登录/推送、集群 VPC 拉取及 Argo CD 同步。
