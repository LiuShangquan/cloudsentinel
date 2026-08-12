# 平台契约

CloudSentinel GitOps 应用依赖平台团队预先提供并独立维护以下能力：

- Argo CD 与 ApplicationSet Controller；
- ingress-nginx 以及阿里云内网/公网 NLB 的入口策略；
- cert-manager，以及名为 `cloudsentinel-public-issuer` 的 `ClusterIssuer`；
- External Secrets Operator v1 CRD，以及名为 `cloudsentinel-secret-store` 的 `ClusterSecretStore`；
- Metrics Server（Production API HPA 使用）；
- 可发现 Service/Pod Prometheus 注解的监控系统；
- 节点到北京 ACR 个人版 VPC 端点与受控探测目标的网络路径。

这些控制器属于集群平台生命周期，不与 CloudSentinel 应用同步。平台版本必须在组织的平台仓库按兼容矩阵和镜像 digest 锁定；本仓库不代替平台仓库安装 CRD，也不向 Argo CD 项目授予 CRD、ClusterRole 或 ClusterSecretStore 的创建权限。

本次学生集群由平台管理员先应用 `bootstrap/secret-store`：它建立只读取 `cloudsentinel-secret-source` 中四个指定 Secret 的 Kubernetes Provider `ClusterSecretStore`。明文仍不进入 Git；源 Secret 必须由操作者通过受控终端创建。`cloudsentinel-secrets/` 只创建 Namespace 级 ExternalSecret，由独立 ApplicationSet 自动同步，以保证应用 PreSync Migration 之前所需 Secret 已存在。

`cloudsentinel-data/overlays/lab` 是第二个明确例外：它为学生练习集群创建 MySQL/Redis StatefulSet、静态本地 PV/PVC 和备份 CronJob。该目录依赖 `worker-data-01` 的固定主机名、`node-role=data` Label、`dedicated=data:NoSchedule` Taint 和预创建目录；不适用于企业生产。

`cloudsentinel-registry` 远端对象必须提供完整 `.dockerconfigjson`，认证服务器为 `crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com`。个人版新实例不依赖免密组件，固定凭证由密钥后端管理并按轮换流程更新。
