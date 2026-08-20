# CloudSentinel Web 公网 IP 直连实施计划

## 目标

在不增加域名、公网负载均衡或 Ingress Controller 的前提下，为学生练习
集群中的 `cloudsentinel-staging` 管理门户提供公网 HTTPS 入口。

该入口只用于当前实验环境，不改变企业生产 Overlay，也不把 API、MySQL、
Redis、Argo CD 或监控内部组件直接暴露到公网。

## 设计

- 保留现有 `cloudsentinel-web` ClusterIP Service；
- 新增固定 `30443/TCP` 的 `cloudsentinel-web-public` NodePort Service；
- 在现有 Web Pod 中增加非 Root TLS 代理容器，并转发到同 Pod 的
  `127.0.0.1:8080`；
- 使用包含目标 ECS 公网 IPv4 SAN 的短期自签名证书；
- 证书、私钥与公网 URL 存入集群源 Secret，再由 External Secrets
  物化到 `cloudsentinel-staging`，不得写入 Git；
- Kubernetes NetworkPolicy 只为 TLS 代理开放 8443/TCP；阿里云安全组只在
  选定的应用 Worker 上开放 30443/TCP，不开放完整 NodePort 端口范围；
- API 继续只由 Web 内部反向代理访问，浏览器不接触 API ClusterIP。

## 实施顺序

1. 合并源码仓库中的证书引导脚本、文档和 GitOps 基线；
2. 将 GitOps 基线同步到独立 `cloudsentinel-gitops` 仓库并通过 PR 合并；
3. 使用 `auth reconcile` 增量更新源 Secret 的最小读取权限；
4. 在健康控制面节点生成自签名证书并创建源 Secret；
5. 让 Argo CD 同步 ExternalSecret、TLS 代理、NodePort 与 NetworkPolicy；
6. 只在选定应用 Worker 的安全组开放 30443/TCP；
7. 从浏览器访问 `https://<worker-public-ip>:30443/` 并验证登录及业务功能。

## 风险与回滚

- 自签名证书不会受到浏览器默认信任，首次访问必须人工核对 SHA256 指纹；
- 客户端公网 IP 不固定而使用 `0.0.0.0/0` 时，登录面会暴露到互联网，必须
  使用独立强密码并在演示结束后删除安全组规则；
- NodePort 入口不具备公网负载均衡高可用能力；
- 回滚时先删除安全组 30443 入站规则，再通过 GitOps 移除公网 Service、
  ExternalSecret、TLS 代理补丁和 NetworkPolicy；不得删除业务数据库。

## 完成门禁

- 四个 Kustomize Overlay 均可渲染，只有 `lab-staging` 新增 30443 NodePort；
- TLS 代理以非 Root、只读根文件系统运行，并设置资源限制和健康检查；
- Shell 脚本通过 `bash -n`；
- Git 中不存在证书、私钥、公网 IP、密码或 JWT；
- 集群运行时验证 ExternalSecret Ready、Web 2/2 Ready、Argo CD
  `Synced/Healthy`，并从外部浏览器完成 HTTPS 登录。
