# Kubernetes 集群架构设计

## 1. 设计目标与边界

目标是为 CloudSentinel 下一阶段准备一个可学习、可演进的单 Kubernetes Cluster。集群基础设施采用 3 Control Plane、4 Worker、stacked etcd、containerd、kubeadm、Calico VXLAN 和 Alibaba Cloud Internal NLB。

当前 ACR 个人版实例固定在华北 2（北京，`cn-beijing`），因此集群、VPC、RDS 与 Tair 的默认地域也固定为北京。若改用其他地域，必须先迁移 Registry 或设计并验证跨地域网络；不得继续引用北京 VPC Registry 域名却把节点采购到杭州。

本文负责集群基础设施设计。CloudSentinel 的 Deployment、Service、ConfigMap、ExternalSecret 引用、Ingress 与 Argo CD 资源已在 `deploy/gitops-repository` 中定义；生产数据库和 Redis 只使用集群外 RDS/Tair，不创建 StatefulSet/PVC。

## 2. 总体拓扑

```text
                    Private DNS（推荐）
                            |
                  Internal NLB :6443
                            |
             +--------------+--------------+
             |              |              |
             v              v              v
         master-01      master-02      master-03
         kube-apiserver kube-apiserver kube-apiserver
         etcd member    etcd member    etcd member
             |              |              |
             +--------------+--------------+
                            |
                  Kubernetes Cluster
                            |
       +--------------------+--------------------+
       |                    |                    |
       v                    v                    v
 worker-app-01/02      worker-monitor      worker-data-01
 Application           Monitoring          Stateful learning

 ops-storage：位于 VPC 内但不加入集群，用作运维入口、备份、Ansible 和 NFS 学习。
```

## 3. Node Roles

| Node | Role | 主要职责 |
|---|---|---|
| `master-01` | control-plane + etcd | 首个 kubeadm 初始化节点、API Server、Controller Manager、Scheduler、stacked etcd |
| `master-02` | control-plane + etcd | 第二个 Control Plane 与 etcd Member |
| `master-03` | control-plane + etcd | 第三个 Control Plane 与 etcd Member |
| `worker-app-01` | application | CloudSentinel API/Worker、未来 Ingress |
| `worker-app-02` | application | CloudSentinel API/Worker、未来 Ingress |
| `worker-monitor` | monitoring | Prometheus、Grafana、Alertmanager |
| `worker-data-01` | reserved | 不承载 CloudSentinel 生产数据库；保留给独立实验或后续重新规划 |

Control Plane 保留 kubeadm 默认 `node-role.kubernetes.io/control-plane:NoSchedule` Taint。Monitoring 与 Data 节点分别使用 Label 和 `NoSchedule` Taint；Application 节点不设置专用 Taint。

## 4. ECS 节点规划

以下是起步规格，不是采购承诺；部署前应根据 Region 可用实例、预算、监控保留期和数据库数据量重新评估。

| 节点组 | 建议起步规格 | 系统盘/数据盘 | 可用区建议 |
|---|---|---|---|
| 3 × Control Plane | 4 vCPU / 8 GiB | 高性能云盘，系统盘至少 80 GiB；etcd 关注低延迟与稳定 IOPS | 尽量分布到 3 个可用区；至少不要全部位于同一物理故障域 |
| 2 × App Worker | 4–8 vCPU / 16 GiB | 系统盘至少 100 GiB，按镜像和日志增长配置 | 分散到不同可用区 |
| Monitoring Worker | 8 vCPU / 32 GiB 起 | 独立数据盘，容量按 Prometheus Retention 估算 | 与应用节点分离 |
| Data Worker | 8 vCPU / 32 GiB 起 | 独立高性能数据盘并配置备份 | 明确接受单点后使用 |
| ops-storage | 2–4 vCPU / 8 GiB 起 | 备份盘按保留策略配置 | 不加入 Kubernetes |

所有 Kubernetes 节点使用固定私网 IP。不要把公网 IP 写入 Inventory；推荐只为 `ops-storage` 或受控 Bastion 提供管理入口。

## 5. Control Plane HA

```text
kubectl / kubelet / kubeadm join
             |
             v
  <CONTROL_PLANE_ENDPOINT>:6443
             |
             v
  Alibaba Cloud Internal NLB
             |
     +-------+-------+
     |       |       |
     v       v       v
 master-01 master-02 master-03
     |       |       |
 kube-apiserver instances
             |
     stacked etcd cluster
```

- 3 个 etcd Member 的 Quorum 为 2，因此可容忍 1 个 etcd Member 故障。
- stacked etcd 将 Control Plane 与 etcd 故障域绑定：失去一台 Master 同时失去一个 API Server 和一个 etcd Member。
- 一次不得同时维护或停止两台 Control Plane。
- NLB 地址必须始终等于 kubeadm 的 `controlPlaneEndpoint`。Kubernetes 官方 HA 指南也要求 Load Balancer 地址与该值保持一致。
- kubeconfig 不得固定到 `master-01` IP，否则 `master-01` 故障时客户端无法利用其余 API Server。
- 优先使用 `Private DNS -> Internal NLB`；暂时没有私有 DNS 时可使用 NLB 私网 IP，但后续变更 IP 的迁移成本更高。

## 6. stacked etcd 的运维含义

- etcd 数据目录默认位于每个 Control Plane 的 `/var/lib/etcd`。
- etcd Peer 使用 TCP 2380，Client 使用 TCP 2379；仅允许 Control Plane Security Group 内部访问。
- NLB 只代理 kube-apiserver TCP 6443，绝不代理 etcd 2379/2380。
- 必须制定 etcd Snapshot、异地备份和定期恢复演练计划；3 Member 不是备份。
- 不得因单节点加入失败而直接删除 `/var/lib/etcd`，必须先确认该节点是否已成为 etcd Member。

## 7. Worker 调度设计

| Worker | Label | Taint | 预期 Workload |
|---|---|---|---|
| `worker-app-01/02` | `node-role=app` | 无 | 普通应用 |
| `worker-monitor` | `node-role=monitoring` | `dedicated=monitoring:NoSchedule` | 显式 Toleration 的监控组件 |
| `worker-data-01` | `node-role=data` | `dedicated=data:NoSchedule` | 显式 Toleration 的学习型有状态组件 |

Label 用于选择节点；Taint 用于排斥没有 Toleration 的 Pod。Toleration 只代表获得进入资格，不代表一定选择该节点，因此未来 Workload 还需要配合 `nodeSelector` 或 Node Affinity。

## 8. CloudSentinel 生产映射

| 当前 Compose 组件 | 未来 Kubernetes 形态 | 目标节点 |
|---|---|---|
| `cloudsentinel-api` | Deployment | `worker-app-01/02` |
| `cloudsentinel-worker` | Deployment | `worker-app-01/02` |
| Prometheus/Grafana/Alertmanager | 平台监控栈 | `worker-monitor` 或平台规划节点 |
| MySQL | Alibaba Cloud RDS | 集群外托管服务 |
| Redis | Alibaba Cloud Tair | 集群外托管服务 |
| 镜像仓库 | 北京 ACR 个人版，VPC 端点按 digest 拉取 | 集群外托管服务 |

对应 Kustomize 与 Argo CD 资源见 `deploy/gitops-repository`；正式环境参数仍须由平台团队完成替换和审批。

## 9. 明确的生产限制

1. RDS/Tair 必须启用符合业务 SLA 的多可用区、备份与恢复演练；托管服务不等于无需运维。
2. 单 Monitoring Worker 是监控可用性单点，生产应扩展为跨故障域平台监控栈。
3. stacked etcd 可容忍一个 Member 故障，但仍需 Snapshot 与恢复演练。
4. API/Worker 通过多副本和拓扑分散降低单节点影响，容量参数需在真实压测后调整。
5. ACR 个人版无生产 SLA且共享带宽；冷拉取并发应低于 10，必须验证节点缓存、凭证轮换和限流恢复，需求增长后升级企业版或 Harbor。

## 10. 版本策略

- `KUBERNETES_VERSION`：完整语义版本，例如 `v1.<minor>.<patch>`，部署时查询并锁定。
- `KUBERNETES_RPM_VERSION`：与语义版本对应的完整 RPM 版本字符串，供 DNF 精确安装。
- `CONTAINERD_VERSION`：仓库中确认可用的完整 RPM 版本。
- `CALICO_VERSION`：经 Calico 官方兼容矩阵确认的 Release Tag。
- kubeadm、kubelet、kubectl 必须使用同一 Patch Version。
- 版本选择、升级和兼容性确认必须在实际执行窗口重新完成，本文不声称已查询或安装任何版本。

## 11. 官方依据

- [Kubernetes kubeadm HA 指南](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/high-availability/)
- [Kubernetes 端口与协议](https://kubernetes.io/docs/reference/networking/ports-and-protocols/)
- [Calico 官方文档入口](https://docs.tigera.io/calico/)
- [Alibaba Cloud NLB Health Check](https://www.alibabacloud.com/help/en/slb/network-load-balancer/user-guide/nlb-health-check-overview/)
