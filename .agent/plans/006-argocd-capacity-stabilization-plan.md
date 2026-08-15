# Argo CD 学生集群容量稳定计划

## 目标

在不改变 CloudSentinel GitOps 架构、Application/ApplicationSet、自动同步、Self-Heal、Prune 或环境边界的前提下，降低 Argo CD 对 2 GiB Kubernetes 控制面的瞬时并发压力。

## 已确认事实

- 7 个 Kubernetes 节点在 Argo CD Application Controller 暂停后全部恢复为 `Ready`。
- `worker-app-01` 仅执行过 `cordon`、`drain` 和 `uncordon`，没有修改 ECS 规格；被驱逐的均为无状态平台 Pod。
- Application Controller 恢复后控制面再次出现资源压力，因此必须先限制协调与同步并发。

## 修改范围

1. 在离线 Argo CD Kustomize Bundle 中固定以下 `argocd-cmd-params-cm` 参数：
   - `controller.status.processors: "1"`
   - `controller.operation.processors: "1"`
   - `controller.kubectl.parallelism.limit: "1"`
   - `reposerver.parallelism.limit: "1"`
2. 构建脚本和安装脚本必须审计四个参数，防止后续重新生成 Bundle 时回退为上游默认并发。
3. 文档说明该配置只适用于学生集群容量边界，不得描述为企业生产吞吐配置。

## 验证

- `bash -n` 通过两个 Shell 脚本。
- `git diff --check` 无空白错误。
- 真实集群先更新 ConfigMap，再保持 ApplicationSet Controller 暂停，只恢复 Application Controller 并观察单次状态查询。
- Secret Application 使用 `ServerSideDiff=true` 排除 External Secrets Webhook 默认化字段造成的伪漂移；对已有 Application 执行一次硬刷新并确认全部为 `Synced/Healthy` 后，再恢复 ApplicationSet Controller。

## 回退

发生 API 延迟或节点资源压力时，将两个 Argo CD Controller 缩容到 `0`。不得删除 Application、ApplicationSet、Namespace、Secret、PV 或 CRD 作为故障处理手段。
