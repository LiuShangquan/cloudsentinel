# CloudSentinel Kubernetes 基础设施手册

> 本目录描述 Kubernetes 集群基础设施的人工建设与排障。CloudSentinel Workload、Ingress、ExternalSecret 和 Argo CD 期望状态已经在 `deploy/gitops-repository` 中提供，其发布流程见 `docs/gitops.md`。集群建设命令仍必须由操作者在确认变量、备份与风险后手工执行。

## 集群目标

- 单个 Kubernetes Cluster。
- 3 个 Control Plane，使用 kubeadm 与 stacked etcd。
- 4 个 Worker，分别承载应用、监控和学习用有状态负载。
- containerd 作为 CRI Runtime。
- Calico VXLAN，禁用 BGP，启用 Kubernetes NetworkPolicy。
- Alibaba Cloud Internal NLB 为 kube-apiserver 提供统一的 `ControlPlaneEndpoint`。
- 集群默认地域固定为华北 2（北京，`cn-beijing`），与当前 ACR 个人版实例一致。
- `ops-storage` 作为集群外运维入口、备份和 NFS 学习节点，不加入 Kubernetes。

## 节点数量与职责

| 类型 | 数量 | 节点 |
|---|---:|---|
| Control Plane + stacked etcd | 3 | `master-01`、`master-02`、`master-03` |
| Application Worker | 2 | `worker-app-01`、`worker-app-02` |
| Monitoring Worker | 1 | `worker-monitor` |
| Data Worker（学习环境） | 1 | `worker-data-01` |
| 集群外运维节点 | 1 | `ops-storage` |

## 技术基线

- OS：CentOS Stream 9。
- CentOS Stream 10：只在关键差异处提供兼容说明，执行前必须确认 DNF5、内核、SELinux、containerd RPM 和 Kubernetes RPM 的实际可用性。
- Kubernetes、containerd、Calico：不在文档中固定“当前最新”版本，部署时通过 `KUBERNETES_VERSION`、`CONTAINERD_VERSION`、`CALICO_VERSION` 锁定。
- Pod 网络：Calico VXLAN，UDP 4789 只允许 Kubernetes 节点之间通信。

## 建议阅读与部署顺序

1. 阅读[架构设计](architecture.md)，确认节点规格、角色和单点限制。
2. 填写 [`deploy/kubernetes/inventory.example.yaml`](../../deploy/kubernetes/inventory.example.yaml) 的副本。
3. 按[网络与安全设计](security-and-network.md)在阿里云控制台准备 VPC、VSwitch、Security Group、Internal NLB 和私有 DNS。
4. 从[部署手册](deployment-guide.md)第 0 节开始逐条执行，直至第 30 节验收。
5. 发生异常时查阅[故障排查与重建](troubleshooting.md)。

## 各文档作用

| 文件 | 用途 |
|---|---|
| `architecture.md` | 集群拓扑、ECS 节点规划、Control Plane HA、CloudSentinel 生产映射和生产限制 |
| `security-and-network.md` | CIDR、Calico VXLAN、Security Group、firewalld 和 NLB 设计 |
| `deployment-guide.md` | 从变量准备、CentOS 初始化、kubeadm 建群到网络与 HA 验收的顺序操作手册 |
| `troubleshooting.md` | 高频故障的现象、检查、解决方向、恢复验证，以及受控重建流程 |
| `inventory.example.yaml` | 不含 Secret 的环境变量与节点信息模板 |

## 部署前必须准备的信息

- 阿里云 Region（当前为 `cn-beijing`）、VPC ID、VPC CIDR、各 VSwitch ID/CIDR 和可用区。
- 7 个 Kubernetes ECS 的私网 IP，以及 `ops-storage` 私网 IP。
- Internal NLB 私网 DNS 或私网 IP，以及监听器/服务器组信息。
- 管理源 IP 或 `ops-storage` Security Group。
- 不重叠的 Pod CIDR 与 Service CIDR。
- 经兼容性确认的 Kubernetes Minor/Patch、containerd 和 Calico 版本。
- 可信镜像来源；CloudSentinel 业务镜像使用北京 ACR 个人版 VPC 端点，系统组件镜像无法访问 `registry.k8s.io` 时应按 digest 同步到受控仓库。
- 备份位置、etcd 备份策略、SSH 管理方式和变更窗口。

## 重要边界

- `worker-data-01` 是单点，不用于 CloudSentinel 生产数据。
- CloudSentinel 生产统一使用 Alibaba Cloud RDS 和 Tair，不保留集群内数据库 StatefulSet 选项。
- 平台控制器由平台仓库维护；应用 GitOps 仓库只管理 CloudSentinel 的 Namespace 级资源。
- ACR 个人版固定凭证只进入 GitHub Repository Secrets 和集群密钥后端，不进入 Inventory 或 Git。
- 本基础设施手册不会自动执行部署；实际应用期望状态以独立 GitOps 仓库为准。
