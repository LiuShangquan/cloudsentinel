# CloudSentinel GitOps 配置仓库种子

此目录是一套可复制为独立私有仓库 `cloudsentinel-gitops` 的完整种子。正式启用后，源码仓库继续保留模板和发布脚本，Argo CD 只读取独立配置仓库；不要让 Argo CD 直接跟踪应用源码仓库。

## 当前学生集群入口

Argo CD ApplicationSet 已指向 `lab-staging` 与 `lab-production`；两者不创建 Ingress，允许先通过端口转发或后续单独入口方案访问。数据层位于 `platform/cloudsentinel-data/overlays/lab`，包含单副本 MySQL/Redis StatefulSet、静态 Retain 本地 PV、NetworkPolicy 和逻辑备份 CronJob。企业 RDS/Tair Overlay 继续保留，但当前不被 ApplicationSet 使用。

## 初始化前必须替换

- 企业 RDS/Tair Overlay 中的 `REPLACE_*` 与域名可以保持休眠，但任何被 Argo CD 引用的 `lab-*`、`bootstrap` 和数据目录不得含占位符；ACR Registry/Namespace 已固定为北京个人版实例和 `cloudsentinel0306`；
- 三个镜像的真实 SHA-256 digest；
- Staging/Production 的外部密钥对象路径；
- Argo CD `repoURL` 与 CODEOWNERS 团队。

`validate-gitops` 只对当前激活路径拒绝占位符，并渲染两个 `lab-*` 应用 Overlay、两个 Secret Overlay、数据层、Secret Store 和 Argo CD Bootstrap。`apps/` 保存应用期望状态；`bootstrap/secret-store` 由平台管理员先应用，`bootstrap/argocd` 后应用；`platform/cloudsentinel-secrets` 负责物化 Secret，使数据初始化和 PreSync Migration 开始前凭证已存在。

应用镜像必须使用 `crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com` 拉取；公网端点只用于 GitHub Actions 推送。Registry 远端 Secret 的 `.dockerconfigjson` 必须将该 VPC 域名作为认证服务器。ACR 个人版无生产 SLA并可能限流，滚动并发和节点缓存应由平台运行手册约束。

## 分支保护

GitHub Free 私有仓库无法强制分支保护或 CODEOWNERS 审批，因此当前基线要求操作者自律使用 PR、检查 `validate-gitops`，并禁止把 GitHub App 配置为绕过流程。CODEOWNERS 只用于责任标记，不等价于强制审批。Production 晋级工作流只复制已经在 Staging 使用的 digest；合并后仍必须由操作者在 Argo CD 手工 Sync。若升级 GitHub Pro，应立即为 `main` 启用分支保护、必需状态检查、对话解决和线性历史。

数据层是单点实验配置。不得删除 `cloudsentinel-data` Namespace、数据 Application、PVC 或 PV；这些操作即使保留底层目录，也会破坏声明式绑定和恢复流程。正式生产必须切回托管数据服务或经验证的高可用平台。
