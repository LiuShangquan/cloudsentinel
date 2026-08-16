# CloudSentinel 轻量监控平台实施计划

## 目标

在不引入 `kube-prometheus-stack`、Prometheus Operator 或额外长期组件的前提下，为学生练习集群建立可由 Argo CD 管理的监控平台：

- Prometheus 采集 staging API/Worker 指标并评估 CloudSentinel 告警规则；
- Alertmanager 将业务探测告警通过机器 Token 回调 staging API；
- Grafana 使用受控凭证并加载现有 CloudSentinel 仪表盘；
- Metrics Server 提供 `kubectl top` 和后续 HPA 的资源指标；
- 所有平台镜像经 GitHub Actions 镜像到北京 ACR Personal Edition，并以不可变 digest 部署；
- 有状态数据使用 `worker-monitor` 上的 Local PV，所有监控工作负载固定到该节点；
- 继续保持生产环境手工同步，不在本阶段引入域名、Ingress 或 TLS 证书。

## 资源边界

目标节点为 `worker-monitor`（2 vCPU、2 GiB）。部署采用单副本并限制资源：

| 组件 | 请求 | 上限 | 持久化 |
|---|---:|---:|---:|
| Prometheus | 150m / 256Mi | 500m / 640Mi | 6 GiB，保留 3 天且限制 4 GiB |
| Alertmanager | 25m / 48Mi | 150m / 128Mi | 1 GiB |
| Grafana | 50m / 128Mi | 250m / 320Mi | 1 GiB |
| Metrics Server | 50m / 96Mi | 200m / 192Mi | 无 |

该配置是实验环境的容量折中，不宣称组件自身高可用。Kubernetes 控制面高可用与监控应用高可用是两个不同层级。

## 交付分解

### Phase 1：GitOps 与 Secret 边界

- 新增独立 `cloudsentinel-platform` AppProject，只允许监控所需的 Namespace、RBAC、APIService 和命名空间资源。
- 新增 `cloudsentinel-monitoring-lab` Application，默认自动同步、禁止空应用、使用 Server-Side Apply。
- 扩展 ClusterSecretStore 允许 `monitoring` Namespace，并仅增加读取 `cloudsentinel-monitoring` 源 Secret 的权限。
- 用 ExternalSecret 将 Grafana 管理凭证、Alertmanager Webhook Token 和 ACR 拉取凭证物化到 `monitoring`。

### Phase 2：监控工作负载

- 使用 Kustomize 管理 Namespace、RBAC、Local PV/PVC、配置、NetworkPolicy 和工作负载。
- Prometheus 使用 Kubernetes Pod 服务发现，只采集显式带 `prometheus.io/scrape=true` 的 staging/production Pod。
- Alertmanager 使用 `authorization.credentials_file` 读取 Secret 文件，不把 Token 写进 ConfigMap 或 Git。
- Grafana 通过 Secret 环境变量读取管理员凭证，数据源与仪表盘由只读 ConfigMap 提供。
- Metrics Server 使用官方组件清单的最小等价部署，并固定调度到监控节点。

### Phase 3：镜像供应链

- 新增手工触发的 GitHub Actions 工作流，逐个拉取 `linux/amd64` 官方镜像、推送 ACR，并再次按 digest 拉取验证。
- 工作流只修改独立 GitOps 仓库中的监控 Overlay image digest，并创建受保护 PR。
- GitOps 校验拒绝公网镜像、可变标签、占位符、错误节点选择器和非 ClusterIP 暴露。

### Phase 4：离线准备与验收

- 提供节点目录/权限准备脚本，仅允许在 `worker-monitor` 执行并做容量门禁。
- 提供集群验收脚本，采用有界轮询，不使用可能长时间无输出的 `kubectl wait`。
- 验收包括：PV/PVC 绑定、Pod Ready、Metrics API 可用、Prometheus targets/rules、Alertmanager ready、Grafana ready、Argo CD Synced/Healthy。

## 明确不做

- 不部署 Prometheus Operator、kube-state-metrics、node-exporter、Loki、Tempo 或 OpenTelemetry Collector；
- 不创建公网 Service、NodePort、Ingress、域名和证书；
- 不把任何密码、Token、Docker 配置或私钥写入 Git；
- 不更改生产环境自动同步策略；
- 不把 Local PV 单副本监控描述为生产高可用。

## 完成门禁

- 所有 Kustomize 目录可渲染，GitOps CI 静态断言全部通过；
- Shell 脚本通过 `bash -n`；
- 文档明确手工准备、回滚与容量边界；
- 集群实际执行结果只有在操作者运行后才能标记为 `VERIFIED`，仓库本地验证不得冒充运行时结果。
