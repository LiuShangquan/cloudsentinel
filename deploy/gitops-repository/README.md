# CloudSentinel GitOps 配置仓库种子

此目录是一套可复制为独立私有仓库 `cloudsentinel-gitops` 的完整种子。正式启用后，源码仓库继续保留模板和发布脚本，Argo CD 只读取独立配置仓库；不要让 Argo CD 直接跟踪应用源码仓库。

## 初始化前必须替换

- `REPLACE_ORG`、`REPLACE_DOMAIN`、ACR Registry/Namespace；
- 三个镜像的真实 SHA-256 digest；
- Staging/Production 的外部密钥对象路径；
- Argo CD `repoURL` 与 CODEOWNERS 团队。

运行 `rg -n "REPLACE_" .` 的结果必须为空，随后渲染两个应用 Overlay 和两个 Secret Overlay。`apps/` 只保存应用期望状态；`bootstrap/` 由平台管理员一次性接入 Argo CD；`platform/cloudsentinel-secrets` 由独立 Argo Application 管理 Secret 物化，使 PreSync Migration 开始前运行时 Secret 已经存在；其余集群控制器不归应用项目管理。

## 分支保护

`main` 禁止直接 Push，要求 `validate-gitops`、CODEOWNERS 审批和线性历史。Production 目录至少需要平台负责人和应用负责人审批。GitHub App 仅获得本仓库 Contents/PR 权限；镜像发布工作流创建 Staging PR，Production 晋级工作流只复制已经在 Staging 使用的 digest。
