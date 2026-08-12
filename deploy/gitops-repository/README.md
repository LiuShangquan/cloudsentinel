# CloudSentinel GitOps 配置仓库种子

此目录是一套可复制为独立私有仓库 `cloudsentinel-gitops` 的完整种子。正式启用后，源码仓库继续保留模板和发布脚本，Argo CD 只读取独立配置仓库；不要让 Argo CD 直接跟踪应用源码仓库。

## 初始化前必须替换

- `REPLACE_DOMAIN` 与环境域名；ACR Registry/Namespace 已固定为北京个人版实例和 `cloudsentinel0306`；
- 三个镜像的真实 SHA-256 digest；
- Staging/Production 的外部密钥对象路径；
- Argo CD `repoURL` 与 CODEOWNERS 团队。

运行 `rg -n "REPLACE_" .` 的结果必须为空，随后渲染两个应用 Overlay 和两个 Secret Overlay。`apps/` 只保存应用期望状态；`bootstrap/` 由平台管理员一次性接入 Argo CD；`platform/cloudsentinel-secrets` 由独立 Argo Application 管理 Secret 物化，使 PreSync Migration 开始前运行时 Secret 已经存在；其余集群控制器不归应用项目管理。

应用镜像必须使用 `crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com` 拉取；公网端点只用于 GitHub Actions 推送。Registry 远端 Secret 的 `.dockerconfigjson` 必须将该 VPC 域名作为认证服务器。ACR 个人版无生产 SLA并可能限流，滚动并发和节点缓存应由平台运行手册约束。

## 分支保护

GitHub Free 私有仓库无法强制分支保护或 CODEOWNERS 审批，因此当前基线要求操作者自律使用 PR、检查 `validate-gitops`，并禁止把 GitHub App 配置为绕过流程。CODEOWNERS 只用于责任标记，不等价于强制审批。Production 晋级工作流只复制已经在 Staging 使用的 digest；合并后仍必须由操作者在 Argo CD 手工 Sync。若升级 GitHub Pro，应立即为 `main` 启用分支保护、必需状态检查、对话解决和线性历史。
