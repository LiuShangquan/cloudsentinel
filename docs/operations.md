# 生产运行、回滚与恢复

## 日常只读检查

先确认 kubeconfig Context 和 Namespace；以下命令只读，但输出可能包含内部地址，不应贴到公开工单：

```bash
kubectl config current-context
kubectl -n cloudsentinel-production get deploy,pod,job,pdb,ingress
kubectl -n cloudsentinel-production get externalsecret
kubectl -n cloudsentinel-data get sts,pod,pvc,cronjob,job
kubectl -n cloudsentinel-production get events --sort-by=.lastTimestamp
```

应用健康标准：API/Worker Desired=Available；MySQL/Redis StatefulSet 均为 `1/1 Ready`；Readiness 为 200；没有长期 Pending Stream；最新 Probe 持续产生；Alertmanager Webhook 无认证失败；最近一次 MySQL/Redis 备份 Job 成功。

镜像分发基线使用北京 ACR 个人版。发布从公网端点推送，集群从 VPC 端点拉取；不要把公网地址写入工作负载。个人版共享带宽可能限流，发布时避免同时重建超过 10 个需要冷拉取镜像的 Pod，并保留 `IfNotPresent` 节点缓存。出现 `TOOMANYREQUESTS` 时应暂停继续滚动、保留健康副本并降低并发，不得切换到来源不明的镜像站。

当前 ACR 个人版实测拒绝 BuildKit 附着式 SBOM/Provenance 的 OCI Attestation Manifest。业务发布工作流因此只推送可运行镜像并记录不可变 Digest，显式关闭附着式 `sbom` 与 `provenance`。这属于 Registry 兼容性折中，不得表述为已具备完整供应链证明；后续应把 SBOM/Provenance 作为独立、可留存和可验证的 CI Artifact，或迁移到支持 OCI Referrers/Attestations 的 Registry。

学生 Overlay 将 API/Worker 每进程 MySQL 上限降为 8，Production API HPA 为 3–4 副本、Worker 固定 3 副本。Staging 各 2 副本时，运行时理论上界为 88 个连接，再加 Migration、Bootstrap、备份和运维连接；MySQL 配置上限为 100，因此不要继续提高副本或连接池。正式压测前应进一步检查实际连接峰值。

## Migration 规则

- 每个 Schema 变更必须包含成对 Up/Down SQL，但生产默认只执行 Up。
- 采用 Expand/Contract：先增加兼容结构，再发布兼容代码，确认旧版本退出后才在后续发布删除旧结构。
- Migration Job 使用独立账户和 URL；不得把连接串打印到日志。
- PreSync Job 失败后停止同步，保留错误信息，评估是否修复 Migration 或恢复数据库。不得反复点击 Sync 试错。
- Bootstrap Secret 只创建首个用户；修改 Secret 不会更新已有 bcrypt 密码。

## 应用回滚

适用于 Migration 向后兼容、数据库无破坏性变化的发布：

1. 冻结新的 GitOps 合并，记录故障版本、API/Worker/Migration/Web 四个 digest、开始时间和影响。
2. 在 GitOps 历史中找到上一个健康 Production `kustomization.yaml`。
3. 创建回滚 PR，只恢复 API/Worker/Migration/Web digest；通过 CODEOWNERS 和必需检查。
4. 合并后查看 Argo Diff，再同步并观察 Readiness、错误率、Probe/Incident 链路。

不要使用 `kubectl set image` 长期绕过 Git；它会产生漂移并被 Argo CD 覆盖。

## 数据库恢复

镜像回滚不会恢复 Schema 或数据。需要 Down Migration、PITR 或实例恢复时，必须由数据库负责人执行独立变更：冻结写入、确认备份点、评估数据丢失窗口、在隔离实例演练、切换连接并验证。CloudSentinel GitOps PR 只能在新数据库 Endpoint 和 Schema 已验证后更新 Secret 引用。

学生数据层从逻辑备份恢复后检查：Migration 版本、业务表行数抽样、用户登录、任务调度、Queued/Running Execution、Incident 状态与审计轨迹。Redis 恢复可能产生重复消息；现有 `execution_id` 和条件迁移用于幂等，但仍需观察 Pending 与 Recovery Dispatcher。详细风险与命令边界见 `docs/kubernetes/lab-stateful-data.md`。

## 紧急处置

- API 故障：保留 Worker 探测链路，回滚 API digest；不要删除 Incident 数据。
- Worker 故障：暂停 Production Sync，检查 MySQL/Redis Readiness、消费者组 Pending 和资源限制，再回滚 Worker。
- MySQL 不可用：API/Worker Readiness 应失败；不要通过修改探针强行接流量。
- Redis 不可用：调度消息暂时无法发布；MySQL 中 Queued Execution 是恢复依据。
- Secret 泄露：先在源系统轮换，确认 ExternalSecret 已刷新，再重启受影响 Deployment；随后吊销旧值并完成审计。
- ACR 凭证泄露：先在 ACR 控制台重置固定密码，再更新 GitHub Repository Secret `ACR_PASSWORD` 和密钥后端中的 `.dockerconfigjson`，确认 Staging 能推送/拉取后再滚动 Production。轮换期间不要把密码放到命令历史、工单或 Git。

删除 Namespace、Argo Application、StatefulSet、PVC/PV、本地目录、备份或执行数据库 Down/恢复都是破坏性操作，本手册不提供可直接复制执行的删除命令。操作者必须单独确认精确目标、异机备份、审批和回退路径。
