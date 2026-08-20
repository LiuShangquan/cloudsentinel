# Kubernetes 网络与安全设计

## 1. 网络对象必须明确区分

| 对象 | 含义 | 是否分配给 ECS 网卡 | 典型使用方 |
|---|---|---:|---|
| VPC CIDR | 阿里云私有网络总地址空间 | 是 | ECS、NLB、RDS、Tair 等 |
| VSwitch CIDR | VPC 内某个可用区的子网 | 是 | ECS 与 NLB Zone |
| Node IP | ECS 主网卡私网地址 | 是 | kubelet、API Server、etcd、VXLAN Underlay |
| Pod CIDR | Kubernetes Pod 地址空间 | 否 | Calico IPAM 与 Pod 网络 |
| Service CIDR | Kubernetes ClusterIP 虚拟地址空间 | 否 | kube-proxy 与 Service |
| ControlPlaneEndpoint | kube-apiserver 的稳定入口 | NLB 私网 DNS/IP + 6443 | kubeadm、kubelet、kubectl |

VPC CIDR、Pod CIDR 和 Service CIDR 三者必须互不重叠，也不得与企业专线、VPN、其他已连接 VPC、办公网或未来需要路由的网络重叠。VSwitch CIDR 必须是 VPC CIDR 的子网。

示例仅用于说明：

```text
Pod CIDR:     10.244.0.0/16
Service CIDR: 10.96.0.0/12
```

> 如果阿里云 VPC 或任何互联网络已经覆盖这些网段，必须重新规划，不能直接照抄。

部署前至少执行书面冲突检查：

```text
VPC/所有 VSwitch
VPN/专线/云企业网 CEN
办公网与数据中心网段
Pod CIDR
Service CIDR
未来对等连接或多集群网段
```

## 2. VPC 与 VSwitch 规划

- 所有节点和 Internal NLB 位于同一 VPC。
- 3 个 Control Plane 尽量跨 3 个可用区；NLB 在相同可用区启用对应 Zone/VSwitch。
- Application Worker 至少分散在两个可用区。
- Monitoring/Data 学习节点可以是单节点，但必须明确其单点属性。
- Node IP 使用固定私网地址；不要依赖会变化的临时地址。
- `ops-storage` 位于可访问节点私网 IP 的受控子网，但不加入 Cluster。

## 3. Calico VXLAN 设计

采用：

```text
Calico CNI
VXLAN Encapsulation
BGP Disabled
Kubernetes NetworkPolicy Enabled
```

选择原因：

- 云环境不需要维护 BGP Peer 或向 VPC 路由表发布 Pod Route。
- VXLAN 在 Node Underlay 上封装跨节点 Pod 流量，部署边界清晰。
- Calico 同时提供 Kubernetes NetworkPolicy 实现。

数据路径：

```text
Pod A
  -> Calico veth
  -> VXLAN Encapsulation
  -> ECS Node 私网（UDP 4789）
  -> 目标节点 VXLAN Decapsulation
  -> Calico veth
  -> Pod B
```

UDP 4789 只允许 Kubernetes Node Security Group 之间双向通信，不得向公网开放。BGP 已禁用，因此无需开放 TCP 179。Calico 仍需要主机内核允许管理 `cali*` 与 `vxlan.calico` Interface；NetworkManager 不得接管这些 Interface。

建议在所有 Node 创建 `/etc/NetworkManager/conf.d/calico.conf`：

```ini
[keyfile]
unmanaged-devices=interface-name:cali*;interface-name:tunl*;interface-name:vxlan.calico
```

随后在维护窗口重载 NetworkManager。该操作会影响节点网络管理，执行前必须确认主网卡连接配置不会被重启中断。

## 4. 阿里云 Security Group 规划

建议至少分为 Control Plane SG、Worker SG 和 Ops SG。规则 Source 应使用明确的 SG 引用、VPC/VSwitch CIDR、NLB Health Check Source 或管理 IP，不使用 `0.0.0.0/0`。

| Port | Protocol | Source | Target | Purpose |
|---|---|---|---|---|
| 22 | TCP | 管理 IP / `ops-storage` SG | Nodes | SSH |
| 6443 | TCP | NLB、Kubernetes Node、受控运维端 | Control Plane | kube-apiserver |
| 2379-2380 | TCP | Control Plane SG | Control Plane | etcd Client/Peer |
| 10250 | TCP | Control Plane SG、Kubernetes Node SG | Nodes | kubelet API |
| 10257 | TCP | Control Plane SG | Control Plane | kube-controller-manager |
| 10259 | TCP | Control Plane SG | Control Plane | kube-scheduler |
| 10256 | TCP | Kubernetes Node SG / 必要的 LB Source | Worker | kube-proxy Health |
| 4789 | UDP | Kubernetes Node SG | Kubernetes Node SG | Calico VXLAN |
| ICMP | ICMP | Kubernetes Node SG、受控运维端 | Nodes | 可选故障诊断；按组织策略限制 |
| 30000-32767 | TCP/UDP | 默认不开放 | Worker | 未来确有 NodePort Service 时按端口和来源最小化开放 |

特别要求：

- TCP 2379/2380 只能在 Control Plane 内部访问，不得对 Worker、办公网或公网开放。
- TCP 6443 的 Backend 入站需要允许 NLB 实际 Health Check/转发来源；以阿里云控制台显示的 NLB/VSwitch Source 为准。
- NodePort 范围不要整体向 `0.0.0.0/0` 开放。未来确需 NodePort 时，只开放具体端口和具体来源。
- 当前学生集群的唯一例外是 `worker-monitor:30300/TCP` 上的 Grafana HTTPS。只给该实例的安全组增加精确端口规则；客户端公网 IP 无法固定时可临时使用 `0.0.0.0/0`，但必须记录暴露窗口并在演示结束后删除。不得据此开放完整 NodePort 范围。
- Security Group 变更后应逐条记录 Rule ID、Source、Target、负责人和回滚方式。

Kubernetes 官方默认端口可参考[端口与协议文档](https://kubernetes.io/docs/reference/networking/ports-and-protocols/)。自定义组件参数会改变实际端口，最终以 kubeadm Config 和进程监听状态为准。

## 5. Security Group 与 firewalld 是两层边界

阿里云 Security Group 作用于云网络边界；CentOS firewalld 作用于操作系统。只配置其中一层并不能保证另一层允许流量，也不应默认把两层都完全放开。

### 5.1 学习环境简化方案

前提：主要网络边界由严格的 Security Group 管理，节点无非受控公网入口，并且已经记录风险。

```bash
sudo systemctl disable --now firewalld
sudo systemctl is-enabled firewalld
sudo systemctl is-active firewalld
```

风险：失去 Host 级第二层过滤；误配 Security Group 时暴露面扩大。该方案不等于“没有防火墙”，而是把主要边界集中到 Security Group。

### 5.2 更严格方案

保留 firewalld，根据节点角色和可信 Source 开放端口。以下是命令形态，不得直接复制占位符：

```bash
sudo firewall-cmd --get-active-zones
sudo firewall-cmd --permanent --zone=public --add-port=10250/tcp
sudo firewall-cmd --permanent --zone=public --add-port=4789/udp

# 仅 Control Plane
sudo firewall-cmd --permanent --zone=public --add-port=6443/tcp
sudo firewall-cmd --permanent --zone=public --add-port=2379-2380/tcp
sudo firewall-cmd --permanent --zone=public --add-port=10257/tcp
sudo firewall-cmd --permanent --zone=public --add-port=10259/tcp

sudo firewall-cmd --reload
sudo firewall-cmd --list-all
```

严格方案还需处理 Calico 管理的 Interface、Forward Policy、Pod CIDR 和 Service 流量，必须在实验环境验证后才能用于生产。不要把 Pod CIDR 无条件放入 Trusted Zone；这会扩大 Host 暴露面。Calico 官方对使用 firewalld 或其他 iptables Manager 有额外要求，部署前应核对所选 Calico Release 的系统要求。

## 6. SELinux 策略

当前 Alibaba Cloud Linux 4 学习基线计划先采用 `permissive`，而不是删除 SELinux；执行前仍需确认当前镜像的默认状态并保留审计证据：

```bash
getenforce
sudo setenforce 0
sudo cp /etc/selinux/config /etc/selinux/config.pre-k8s
sudo sed -ri 's/^SELINUX=.*/SELINUX=permissive/' /etc/selinux/config
grep '^SELINUX=' /etc/selinux/config
```

原因是 kubelet、containerd、CNI 对 Host Path、Namespace、Network Interface 和 iptables/nftables 的操作可能被未配置的 SELinux Policy 阻止。`permissive` 仍记录 AVC Denial，便于后续构建 Policy。

生产影响：Permissive 不再强制阻断违规访问，减少了一层 Mandatory Access Control。生产环境应在测试集群收集 `ausearch -m AVC` 结果、验证官方 SELinux 支持并建立定制 Policy 后，再评估恢复 Enforcing；不得简单声称“关闭 SELinux 就安全”。

## 7. Internal NLB 设计

| 配置项 | 值 |
|---|---|
| 类型 | Alibaba Cloud Internal NLB |
| Listener Protocol | TCP |
| Listener Port | 6443 |
| Backend Protocol/Port | TCP 6443 |
| Backend | `master-01`、`master-02`、`master-03` 私网 IP |
| Health Check | TCP 6443 |
| Control Plane Endpoint | `<NLB_PRIVATE_DNS_OR_IP>:6443` |

NLB 使用 TCP Health Check 判断 Backend Port 是否可连接；不健康 Backend 会停止接收新流量，恢复后自动重新加入。实际 Health Check Source 必须同时通过 Security Group、firewalld 和其他 Host Policy。[阿里云 NLB Health Check 文档](https://www.alibabacloud.com/help/en/slb/network-load-balancer/user-guide/nlb-health-check-overview/)

配置顺序：

1. 创建 Internal NLB，并选择与 Control Plane 对应的 Zone/VSwitch。
2. 创建 TCP Server Group，添加 3 个 Master 私网 IP，Backend Port 为 6443。
3. 启用 TCP 6443 Health Check。
4. 创建 TCP 6443 Listener 并关联 Server Group。
5. 配置 Private DNS 记录指向 NLB；没有 Private DNS 时记录 NLB 私网 IP。
6. 将该地址写入 `CONTROL_PLANE_ENDPOINT`，且 kubeadm Config、Join Command 和 Kubeconfig 必须一致。

`kubeadm init` 前，API Server 尚未运行，TCP Probe 返回 Connection Refused 可以是预期现象；Timeout 表示 NLB 到 Backend 的网络、安全规则或路由存在问题。官方 kubeadm HA 指南明确区分了这两种情况。

## 8. DNS 与名称解析

推荐使用阿里云 PrivateZone 或组织内部 DNS：

```text
k8s-api.<private-domain> -> Internal NLB
master-01/02/03          -> 各自私网 IP
worker-*                 -> 各自私网 IP
```

学习环境可以使用 `/etc/hosts` 维护 Node Name，但 NLB Endpoint 仍优先使用稳定私有 DNS。所有节点的名称解析结果必须一致，禁止同一 Hostname 在不同节点解析到不同地址。

## 9. 出站访问与镜像安全

- Node 需要访问 Kubernetes RPM Repository、containerd RPM Repository、目标镜像仓库和 Calico Release 文件，或由内部镜像/制品仓库提供等价内容。
- CloudSentinel 业务镜像从 `crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com` 按 Digest 拉取；集群必须位于北京地域并验证 DNS、TCP 443 和凭证。无法访问 `registry.k8s.io` 时，优先将经过 Digest 校验的系统镜像同步到自己的受控仓库。
- 不推荐来源不明的公共 Mirror，也不在文档中写入临时 Mirror 地址。
- ACR Credential、SSH Key、Token、Certificate Key 和 Kubeconfig 不得写入 Git 或 Inventory Template。

## 10. CIDR 与 NLB 上线前检查表

- [ ] VPC、Pod、Service CIDR 两两不重叠。
- [ ] 与 VPN/CEN/办公网/数据中心网络无冲突。
- [ ] 所有 Node 使用固定私网 IP。
- [ ] UDP 4789 只允许 Kubernetes Node 之间通信。
- [ ] TCP 2379/2380 只允许 Control Plane SG 内部通信。
- [ ] 未向公网开放完整 NodePort Range。
- [ ] 若启用实验 Grafana 公网入口，仅 `worker-monitor:30300/TCP` 可达，且已记录来源与关闭时间。
- [ ] NLB 是 Internal，Listener 与 Backend 均为 TCP 6443。
- [ ] NLB 3 个 Backend 跨故障域并启用 Health Check。
- [ ] Private DNS 与 `CONTROL_PLANE_ENDPOINT` 一致。
- [ ] Security Group 和 firewalld 策略已分别审查。

## 11. CentOS Stream 10 Compatibility Note

CentOS Stream 10 可能默认使用 DNF5，并带来不同的 Package Plugin、内核、SELinux Policy 与 nftables/firewalld 行为。执行前必须：

```bash
cat /etc/os-release
dnf --version
dnf5 --version
uname -r
firewall-cmd --version
```

确认 containerd RPM 和 Kubernetes RPM 明确支持 EL10；确认 Calico 支持当前内核；确认 Version Lock Plugin 的 DNF5 命令。不要把 Stream 9 的 RPM URL、Plugin Package Name 或 firewalld 行为直接视为 Stream 10 已验证。
