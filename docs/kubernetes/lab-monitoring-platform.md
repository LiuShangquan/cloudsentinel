# 学生集群轻量监控平台

本文定义 CloudSentinel 学生集群的监控基线。它只服务当前 7 节点练习环境，不是企业生产高可用监控方案。所有持久化组件均为单副本并固定到 `worker-monitor`；该节点或系统盘故障会使监控数据暂时不可用，但不会直接中断 CloudSentinel API、Worker、MySQL 或 Redis。

## 1. 组件与边界

| 组件 | 固定版本 | 用途 | 持久化 |
|---|---|---|---|
| Prometheus | `3.5.3` | 抓取 API/Worker 指标、规则计算 | 6 GiB Local PV，3 天且最多 4 GB |
| Alertmanager | `0.32.1` | 把告警回调到 Staging API Webhook | 1 GiB Local PV |
| Grafana | `13.0.1` | 展示 CloudSentinel 仪表盘 | 1 GiB Local PV |
| Metrics Server | `0.8.1` | 提供 `kubectl top` 所需的 Resource Metrics API | 无 |

当前不安装 Prometheus Operator、`kube-prometheus-stack`、node-exporter、kube-state-metrics、Loki 或日志采集系统。这样可以在 2 vCPU、2 GiB 的 `worker-monitor` 上保留完整业务监控链路，同时避免把练习集群压垮。Metrics Server 不是 Prometheus 的替代品，它只提供短期 CPU/内存资源指标。

所有服务保持 `ClusterIP`，没有 Ingress、公网负载均衡、域名或 TLS 暴露。需要查看 Grafana 时，由操作者临时执行端口转发。

## 2. 调度、容量与安全

- `worker-monitor` 必须具有 `node-role=monitoring` Label 和 `dedicated=monitoring:NoSchedule` Taint。
- Prometheus、Alertmanager、Grafana 和 Metrics Server 都只调度到 `worker-monitor`，并显式容忍该 Taint。
- 三个 Local PV 使用 `Retain`，删除 PVC 不会自动删除宿主机目录；Local PV 仍然是单节点存储，不等价于备份。
- Prometheus 只获得发现 Pod、Service、EndpointSlice 和 Namespace 的只读权限。
- Grafana 管理员密码、Alertmanager Webhook Token 和 ACR Pull Secret 通过 External Secrets 物化，Git 中没有明文。
- 监控 Namespace 默认拒绝入站流量，仅允许 Namespace 内部组件互访。CloudSentinel 应用的既有 NetworkPolicy 已允许 `monitoring` Namespace 抓取指标。
- 监控镜像只使用 ACR VPC 地址和不可变 digest，节点不直接从 Docker Hub、GHCR 或 `registry.k8s.io` 拉取。

## 3. 首次启用顺序

以下操作必须由操作者逐步执行。不要把整段命令一次性粘贴到多个终端。

### 3.1 在 `worker-monitor` 准备 Local PV 目录

把源码仓库中的 `deploy/kubernetes/prepare-monitoring-storage-alinux4.sh` 上传到 `worker-monitor:/root/`，核对脚本 SHA256 后执行：

```bash
chmod 0700 /root/prepare-monitoring-storage-alinux4.sh
bash /root/prepare-monitoring-storage-alinux4.sh
```

脚本要求节点名精确为 `worker-monitor`、kubelet/containerd 为 Active，并至少保留 12 GiB 可用空间。成功标记为 `MONITORING_STORAGE_PREPARE=PASS`。

### 3.2 同步监控镜像并合并 GitOps PR

先在 ACR 个人版命名空间 `cloudsentinel0306` 中创建四个私有仓库：`cloudsentinel-prometheus`、`cloudsentinel-alertmanager`、`cloudsentinel-grafana`、`cloudsentinel-metrics-server`。当前实例已关闭自动创建仓库，缺少任意仓库都会导致 Push 失败。

由于首次监控平台 PR 会同步 GitOps 仓库的 `.github/workflows/validate.yml`，发布 GitHub App 除 Contents/Pull Requests 读写外，必须拥有 Workflows 读写权限。修改 GitHub App 权限后，需要在安装页确认新权限；工作流只为短期 Token 请求这三项权限，不给予 Administration 或 Secrets 等无关权限。

在源码仓库 GitHub Actions 中，从 `main` 手工运行 `mirror-monitoring-images`。工作流会：

1. 把四个固定 `linux/amd64` 上游镜像同步到 ACR；
2. 使用 Registry 返回的 digest 做回拉验证；
3. 把监控 Overlay、平台 AppProject、Application 和 Secret Store 变更复制到独立 GitOps 仓库；
4. 创建 GitOps PR。

等待 `validate-gitops` 变绿，确认 PR 只包含监控平台及相应 Bootstrap 变更后合并。不得手工猜测或复制镜像 digest。

### 3.3 更新 Secret Store 并创建监控源 Secret

在 `master-01` 更新只读的 `cloudsentinel-gitops-bootstrap` 工作副本，然后先验证并应用 Secret Store：

```bash
export KUBECONFIG=/etc/kubernetes/admin.conf

kubectl apply --server-side --dry-run=server \
  -k /root/cloudsentinel-gitops-bootstrap/bootstrap/secret-store

kubectl apply --server-side \
  --field-manager=cloudsentinel-platform-bootstrap \
  -k /root/cloudsentinel-gitops-bootstrap/bootstrap/secret-store
```

再把源码仓库中的 `deploy/kubernetes/bootstrap-monitoring-source-secret.sh` 上传到 `master-01:/root/` 并执行：

```bash
chmod 0700 /root/bootstrap-monitoring-source-secret.sh
bash /root/bootstrap-monitoring-source-secret.sh
```

脚本不会打印任何密码或 Token。它从既有 Staging 源 Secret 安全复制 Alertmanager Webhook Token，并提示输入独立的 Grafana 管理员密码，最终只在 `cloudsentinel-secret-source/cloudsentinel-monitoring` 中保存三个键。

### 3.4 创建平台 Application

先做 Server-Side Dry Run，再应用 Argo CD Bootstrap：

```bash
kubectl apply --server-side --dry-run=server \
  -k /root/cloudsentinel-gitops-bootstrap/bootstrap/argocd

kubectl apply --server-side \
  --field-manager=cloudsentinel-platform-bootstrap \
  -k /root/cloudsentinel-gitops-bootstrap/bootstrap/argocd
```

`cloudsentinel-monitoring-lab` 会自动同步，但禁用 Prune；平台级 PV、StorageClass、RBAC 与 APIService 不会因一次 Git 误删被自动清除。

## 4. 验收

把 `deploy/kubernetes/verify-monitoring-platform.sh` 上传到 `master-01:/root/` 后执行：

```bash
chmod 0700 /root/verify-monitoring-platform.sh
bash /root/verify-monitoring-platform.sh
```

验收必须同时满足：

- Argo CD Application 为 `Synced/Healthy`；
- Prometheus、Alertmanager、Grafana 和 Metrics Server 工作负载 Ready；
- 三个 PVC 均为 Bound，四类 Pod 均在 `worker-monitor`；
- `kubectl top node` 能返回结果；
- Prometheus、Alertmanager、Grafana内部健康端点可访问；
- Prometheus 已发现 CloudSentinel Staging API 和 Worker Target。

成功标记为 `MONITORING_PLATFORM_VERIFY=PASS`。脚本只读取状态，不创建业务数据，也不打印 Secret。

## 5. 查看 Grafana

在能够访问 Kubernetes API 的管理终端执行：

```bash
kubectl -n monitoring port-forward service/grafana 3000:3000
```

浏览器访问 `http://127.0.0.1:3000`。用户名默认为 `admin`，密码是创建监控源 Secret 时输入的值。不要使用 `kubectl get secret -o yaml` 或把密码粘贴到聊天、日志和截图中。

## 6. 故障与回滚边界

- Pod Pending：先检查 `worker-monitor` 的 Label/Taint、PVC/PV 绑定和宿主机目录。
- `ImagePullBackOff`：以 Pod Event 中的真实 digest 为准，重新运行镜像同步工作流；不要改成公网 Tag。
- ExternalSecret NotReady：检查 `cloudsentinel-secret-store` 是否允许 `monitoring`，以及源 Secret 的三个键是否存在。
- Prometheus 无 Target：检查应用 Pod 的 `prometheus.io/scrape`、`prometheus.io/port`、`prometheus.io/path` 注解和 NetworkPolicy。
- 节点容量紧张：先暂停监控 Application 自动同步或将相应工作负载缩为 0，保留 PVC/PV；不要删除 Local PV 目录。
- 删除 Application 不会自动清理 Retain PV。任何目录删除都必须先确认备份与精确绝对路径。

企业生产应改用多副本 Prometheus/Thanos 或托管监控、Alertmanager HA、Grafana 外部数据库及跨可用区持久化；不得把本页方案直接升级名称后用于生产。
