# 平台契约

CloudSentinel GitOps 应用依赖平台团队预先提供并独立维护以下能力：

- Argo CD 与 ApplicationSet Controller；
- ingress-nginx 以及阿里云内网/公网 NLB 的入口策略；
- cert-manager，以及名为 `cloudsentinel-public-issuer` 的 `ClusterIssuer`；
- External Secrets Operator v1 CRD，以及名为 `cloudsentinel-secret-store` 的 `ClusterSecretStore`；
- Metrics Server（Production API HPA 使用）；
- 可发现 Service/Pod Prometheus 注解的监控系统；
- 节点到 ACR、RDS、Tair 与受控探测目标的网络路径。

这些控制器属于集群平台生命周期，不与 CloudSentinel 应用同步。平台版本必须在组织的平台仓库按兼容矩阵和镜像 digest 锁定；本仓库不代替平台仓库安装 CRD，也不向 Argo CD 项目授予 CRD、ClusterRole 或 ClusterSecretStore 的创建权限。

`cloudsentinel-secrets/` 是例外：它只创建 Namespace 级 ExternalSecret，由独立 ApplicationSet 自动同步，以保证应用 PreSync Migration 之前所需 Secret 已存在。它仍不创建 Secret 明文、ClusterSecretStore 或任何集群级权限。
