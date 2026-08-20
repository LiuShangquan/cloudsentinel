# Grafana 公网 IP 直连实施计划

## 目标

在不增加公网负载均衡、Ingress 或域名的前提下，让操作者无需 SSH
端口转发即可通过 `worker-monitor` 的公网 IP 查看 Grafana。

该能力仅用于当前学生练习集群。它不改变 Prometheus、Alertmanager 和
Metrics Server 的 `ClusterIP` 边界，也不作为企业生产暴露方案。

## 设计

- 保留现有内部 `grafana` ClusterIP Service；
- 新增 `grafana-public` NodePort Service，固定端口为 `30300/TCP`；
- 设置 `externalTrafficPolicy: Local`，只由实际运行 Grafana Pod 的
  `worker-monitor` 接收流量，并保留客户端源地址；
- Grafana 自身在 3000 端口启用 HTTPS；
- 使用包含公网 IP SAN 的 30 天自签名证书；
- 证书、私钥和公网 URL 存入集群源 Secret，再由 External Secrets
  物化到 `monitoring`，不得写入 Git；
- NetworkPolicy 只给 Grafana 3000/TCP 增加公网来源；阿里云安全组只给
  `worker-monitor` 增加 30300/TCP，不开放完整 NodePort 端口段；
- 保持 Grafana 登录认证、禁止匿名访问并启用 Secure/SameSite Cookie。

## 实施顺序

1. 在 `master-01` 生成自签名证书并创建源 Secret；
2. 更新 Secret Store 的最小只读 RBAC；
3. 通过 GitOps PR 发布 ExternalSecret、NodePort、HTTPS 配置和 NetworkPolicy；
4. 只在 `worker-monitor` 所属安全组开放 TCP 30300；
5. 从操作者电脑访问 `https://<worker-monitor-public-ip>:30300/`；
6. 验证 Grafana 登录、仪表盘及内部 Prometheus 数据源。

## 风险与回滚

- 自签名证书不受浏览器信任，首次访问必须人工核对证书指纹；
- 客户端公网 IP 不固定，因此安全组若使用 `0.0.0.0/0`，Grafana 登录面会暴露
  到互联网；管理员密码必须足够强，不得复用 SSH、ACR 或应用密码；
- NodePort 依赖 `worker-monitor` 公网 IP 与该节点可用性，不具备高可用；
- 回滚时先删除安全组 30300 入站规则，再从 GitOps 移除 `grafana-public`
  Service/NetworkPolicy/ExternalSecret，并恢复 Grafana HTTP 配置；不得删除
  Grafana Local PV 数据目录。

## 完成门禁

- Kustomize 可渲染且只存在一个固定的公网 NodePort `30300/TCP`；
- Shell 脚本通过 `bash -n`；
- Git 中不存在证书、私钥、公网 IP 或管理员密码；
- 集群运行时必须验证 ExternalSecret Ready、Grafana Ready、Argo CD
  `Synced/Healthy`，以及外部浏览器能通过 HTTPS 登录。
