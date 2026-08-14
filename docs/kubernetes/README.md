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

- OS：Alibaba Cloud Linux 4 LTS 64 位普通版，Linux 6.6 LTS，统一 cgroup v2。
- 当前八台 ECS 已完成操作系统镜像更换并全部通过只读预检；7 个 Kubernetes 节点也已完成 containerd、kubeadm、kubelet、kubectl、cri-tools 与 CNI Plugins 基础安装验收，并已从 ACR VPC 端点预拉固定 kubeadm 镜像、对齐 containerd Pause 镜像。`ops-storage` 保持集群外角色，不执行 Kubernetes 镜像准备脚本。
- 当前实验集群已冻结 Kubernetes `v1.35.7`、containerd `1.7.34` 与 Calico `v3.32.1`；不得把这些版本描述为长期自动跟随的“最新版本”。
- Pod 网络：Calico VXLAN，UDP 4789 只允许 Kubernetes 节点之间通信。

## 建议阅读与部署顺序

1. 阅读[架构设计](architecture.md)，确认节点规格、角色和单点限制。
2. 按 [`deploy/kubernetes/inventory.example.yaml`](../../deploy/kubernetes/inventory.example.yaml) 维护实际 Inventory；当前学生集群已记录在 [`inventory.lab.yaml`](../../deploy/kubernetes/inventory.lab.yaml)，其中仍为占位符的字段必须在建群前补齐。
3. 按[网络与安全设计](security-and-network.md)在阿里云控制台准备 VPC、VSwitch、Security Group、Internal NLB 和私有 DNS。
4. 从[部署手册](deployment-guide.md)第 0 节开始逐条执行，直至第 30 节验收。
5. 集群通过验收后，按[实验 StatefulSet 数据层](lab-stateful-data.md)准备数据节点、密钥、ACR 镜像和 Argo CD 同步顺序。
6. 发生异常时查阅[故障排查与重建](troubleshooting.md)。

## 各文档作用

| 文件 | 用途 |
|---|---|
| `architecture.md` | 集群拓扑、ECS 节点规划、Control Plane HA、CloudSentinel 生产映射和生产限制 |
| `security-and-network.md` | CIDR、Calico VXLAN、Security Group、firewalld 和 NLB 设计 |
| `deployment-guide.md` | 从变量准备、CentOS 初始化、kubeadm 建群到网络与 HA 验收的顺序操作手册 |
| `troubleshooting.md` | 高频故障的现象、检查、解决方向、恢复验证，以及受控重建流程 |
| `lab-stateful-data.md` | 学生集群 MySQL/Redis StatefulSet、静态本地存储、Secret、备份与启用顺序 |
| `inventory.example.yaml` | 不含 Secret 的环境变量与节点信息模板 |
| `inventory.lab.yaml` | 当前 8 台 ECS 的私网地址、角色、容量和部署阻断项；不记录公网地址或 Secret |
| `kubeadm-init-master-01.yaml` | 当前实验集群完成变量替换的 kubeadm v1beta4 初始化配置；不含 Bootstrap Token、Certificate Key 或其他 Secret |
| `preflight-alinux4.sh` | Alibaba Cloud Linux 4 八节点只读体检；不安装软件、不加载模块、不修改系统 |
| `prepare-kubernetes-images-alinux4.sh` | 在 7 个 Kubernetes 节点从 ACR VPC 端点预拉固定 kubeadm 镜像，并对齐 containerd Pause 镜像；不执行 init/join |
| `prepare-calico-images-alinux4.sh` | 在 7 个 Kubernetes 节点从 ACR VPC 端点预拉固定 Calico Linux 镜像；不执行 init/join 或安装 CNI |
| `prepare-cluster-test-image-alinux4.sh` | 按工作流输出的 digest，在两个应用节点预拉并验证固定集群验收工具镜像 |
| `run-cluster-network-smoke.sh` | 在 `master-01` 执行可回收的跨节点 Pod、ClusterIP、DNS 和 NetworkPolicy 验收 |
| `tests/network-smoke.yaml`、`tests/network-policy-smoke.yaml` | 只用于 `cloudsentinel-smoke` 临时 Namespace 的固定验收资源；成功后删除 |
| `calico/v3.32.1/` | 固定的官方 CRD、上游 Operator 审计副本、ACR Operator 清单与实验集群 Installation 资源 |
| `mirror-calico-images.yml` | GitHub Actions 手工工作流：将固定 Calico Linux/amd64 镜像同步到 ACR 个人版扁平仓库 |
| `mirror-cluster-test-images.yml` | GitHub Actions 手工工作流：将固定 BusyBox Linux/amd64 工具镜像同步到 ACR，并输出目标 digest |
| `preflight-centos7.sh` | 已停用的 CentOS 7 历史体检脚本，仅保留审计上下文，不再用于当前集群 |

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

- `worker-data-01` 是学生练习数据层单点；节点或本地盘故障会同时影响 MySQL、Redis 和同盘备份。
- 当前实购的 Control Plane、Monitoring、Data 与运维节点均只有 2 GiB 内存，属于练习环境最低边缘容量：监控必须使用短保留期和低资源配置，不能直接套用企业默认值；数据层也已降低 MySQL/Redis 内存参数，并必须观察 Node MemoryPressure 与容器 OOM。
- 当前所有节点已更换为 Alibaba Cloud Linux 4 LTS 64 位普通版；CIDR、Internal NLB、组件版本与 7 节点 Runtime 基线均已冻结并完成实测。固定 kubeadm 与 Calico 核心镜像已在全部 7 个 Kubernetes 节点完成 ACR VPC 拉取验证；新增的 Calico API Server 镜像已在实际调度节点 `master-01` 完成拉取验证。公网全 TCP、HTTP 80 与 RDP 3389 已移除；学生实验期经所有者接受风险后保留公网 ICMP、公网 SSH 22 和共享安全组组内互通。全部 8 台节点已经验证采用仅密钥 SSH。
- `master-01` 已于 2026-08-14 完成真实初始化；此前的 kdump 内存门禁和主机名解析问题均已消除，API `/livez`、`/readyz` 与控制面静态 Pod 已通过验收。初始化输出中暴露的临时凭据已轮换，替换后的 Join 文件只保存在 Master 的 Root-only 目录中。
- Calico CNI 与 `APIServer/default` 已于 2026-08-14 安装，CoreDNS、两副本 Calico API Server 均健康，`apiserver`、`calico`、`ippools`、`tiers` 四个 TigeraStatus 全部可用且无降级。同日完成 3 个 Control Plane 与 4 个 Worker 的逐台 Join：7 个 Kubernetes 节点均为 Ready，stacked etcd 的 3 个 Member 均为 `started`；应用节点、监控节点和数据节点的项目 Label/Taint 已按规划落地。
- 集群基础状态与 CoreDNS 已通过验收。跨节点网络测试必须先由 `mirror-cluster-test-images.yml` 把固定 `busybox:1.37.0` 同步到 ACR，记录工作流输出 digest，并在两个应用节点使用准备脚本完成 digest 校验；测试 Pod 使用 `imagePullPolicy: Never`，不得临时从公网仓库拉取镜像。
- 2026-08-14 已按固定 digest 在两个应用节点完成测试镜像预拉，并通过跨节点 Pod、ClusterIP/EndpointSlice、CoreDNS 与 Calico NetworkPolicy 的真实验收；拒绝客户端按预期超时，临时 `cloudsentinel-smoke` Namespace 已自动删除。
- 7 个节点当前均无 Memory、Disk 或 PID Pressure；三台 API Server 直连健康，Internal NLB 的双私网 VIP 连续 20 次健康检查零失败。`master-03` 单 API Backend 故障窗口内，NLB 再次连续 20 次零失败并保持 `/readyz=ok`；恢复后 Node 与 API Server Pod 均重新 Ready。集群底层验收已结束，可以进入平台控制器、Secret、数据层和 Argo CD 部署。
- 企业生产仍统一使用 Alibaba Cloud RDS 和 Tair；本次 StatefulSet 只属于 `lab-*` Overlay，不构成生产高可用方案。
- 平台控制器由平台仓库维护；应用 GitOps 仓库只管理 CloudSentinel 的 Namespace 级资源。
- ACR 个人版固定凭证只进入 GitHub Repository Secrets 和集群密钥后端，不进入 Inventory 或 Git。
- 本基础设施手册不会自动执行部署；实际应用期望状态以独立 GitOps 仓库为准。
