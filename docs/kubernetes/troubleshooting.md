# Kubernetes 高频故障排查与受控重建

> 本文是未来人工操作手册，未在真实集群执行。先采集证据，再选择恢复方向。不得用关闭全部防火墙、禁用证书校验、清空 iptables、删除 `/var/lib/etcd` 或删除 `/etc/kubernetes` 来掩盖根因。

## 通用证据采集

在不泄露 Token、Kubeconfig、Certificate Key 和 Private Key 的前提下，先记录：

```bash
date --iso-8601=seconds
hostnamectl status
cat /etc/os-release
uname -r
getenforce
swapon --show
systemctl status containerd kubelet --no-pager
journalctl -u containerd -u kubelet -n 200 --no-pager
containerd --version
kubeadm version -o short
```

能访问 API 时再记录：

```bash
kubectl get nodes -o wide
kubectl get pods -A -o wide
kubectl get events -A --sort-by=.lastTimestamp
kubectl get --raw='/readyz?verbose'
```

## 1. kubeadm init 失败

**现象**：Preflight Error、等待 Control Plane 超时、Static Pod Crash，或 `kubeadm init` 无法完成。

**常见原因**：Swap 未关闭；CRI 不可用；Cgroup Driver 不一致；6443/2379 已被占用；镜像拉取失败；Hostname/IP/ControlPlaneEndpoint 错误；NLB Timeout；旧 kubeadm 状态残留。

**检查命令**：

```bash
swapon --show
systemctl status containerd kubelet --no-pager
sudo crictl --runtime-endpoint unix:///run/containerd/containerd.sock info
sudo crictl --runtime-endpoint unix:///run/containerd/containerd.sock ps -a
sudo ss -lntp | grep -E ':6443|:2379|:2380|:10250'
sudo journalctl -u kubelet -n 200 --no-pager
sudo kubeadm config validate --config /root/kubeadm-config.yaml
```

**解决方向**：按第一个明确错误修复；确保 Config API、CIDR、Advertise Address、NLB Endpoint、CRI Socket 和版本一致。若初始化从未成功且确认要重试，先按本文“整个实验集群重新初始化”的审批流程判断是否允许 Reset。

**恢复验证**：`kubeadm init` 成功；API 6443 与 etcd 2379/2380 监听；Static Pod Running；`readyz` 可查询。

## 2. kubeadm join 失败

**现象**：Discovery Timeout、Unauthorized、CA Hash 不匹配、Node 已存在，或等待 kubelet Bootstrap 超时。

**常见原因**：Token 过期；NLB 6443 不通；Discovery Hash/Endpoint 错误；Hostname 重复；CRI/Cgroup 异常；节点有旧 `/etc/kubernetes` 状态。

**检查命令**：

```bash
hostname -s
nc -zv -w 2 <NLB_PRIVATE_IP_OR_DNS> 6443
kubeadm token list                 # Control Plane 上执行
journalctl -u kubelet -n 200 --no-pager
ls -la /etc/kubernetes /var/lib/kubelet
```

**解决方向**：重新生成短期 Token；从受控渠道重新取得 Discovery Hash；修复 NLB/CRI/Hostname。旧 Node 确实是失败的 Worker Bootstrap 时，才按“Worker 加入失败”流程 Reset。

**恢复验证**：Node 出现在 `kubectl get nodes`，CSR 正常批准，Calico Ready，Node 最终 Ready。

## 3. containerd 或 CRI Unavailable

**现象**：`validate service connection: CRI v1 runtime API is not implemented`、Socket Not Found，或 kubelet 报 Container Runtime Not Ready。

**常见原因**：containerd 未启动；CRI 被列入 `disabled_plugins`；Config TOML 错误；Socket Path 不一致；错误的 containerd 主版本配置路径。

**检查命令**：

```bash
systemctl status containerd --no-pager
journalctl -u containerd -n 200 --no-pager
test -S /run/containerd/containerd.sock
grep -nE 'disabled_plugins|SystemdCgroup|io.containerd.*cri' /etc/containerd/config.toml
sudo crictl --runtime-endpoint unix:///run/containerd/containerd.sock info
```

**解决方向**：使用当前版本 `containerd config default` 重新比对配置；启用 CRI；修正 Socket；验证 TOML 后重启。不要盲目套用其他主版本的 `sed`。

**恢复验证**：containerd Active；`crictl info` 成功；kubelet 不再报告 RuntimeNotReady。

## 4. kubelet NotReady

**现象**：Node 为 `NotReady`，Condition 显示 KubeletNotReady、NetworkPluginNotReady 或 RuntimeNotReady。

**常见原因**：containerd/CRI、CNI 未安装或损坏、Cgroup、Swap、磁盘压力、证书、Node IP 错误。

**检查命令**：

```bash
kubectl describe node <NODE_NAME>
journalctl -u kubelet -n 200 --no-pager
systemctl status containerd --no-pager
df -h
df -i
ip address
```

**解决方向**：根据 Node Condition 修复 Runtime、CNI、Disk、Certificate 或网络；不要直接删除 Node 重新加入。

**恢复验证**：所有 Condition 健康，Node Ready，DaemonSet Pod Ready，持续观察不再反复切换状态。

## 5. Cgroup Driver Mismatch

**现象**：kubelet 启动失败、Pod Sandbox 创建失败，Log 显示 `systemd`/`cgroupfs` 不一致。

**常见原因**：containerd `SystemdCgroup` 未设为 True；KubeletConfiguration 未使用 `systemd`；升级后配置被覆盖。

**检查命令**：

```bash
stat -fc %T /sys/fs/cgroup
sudo containerd config dump | grep -n -A8 -B4 SystemdCgroup
grep -n 'cgroupDriver' /var/lib/kubelet/config.yaml
journalctl -u kubelet -n 200 --no-pager
```

**解决方向**：使 containerd 和 kubelet 都使用 `systemd`。现有集群变更应按 Kubernetes 官方 Cgroup Migration 流程逐 Node Drain、修改、重启和 Uncordon。

**恢复验证**：kubelet Active，Node Ready，新 Pod 可创建。

## 6. Swap 导致 Preflight 或 kubelet 异常

**现象**：kubeadm 报 Swap Enabled，或 kubelet 拒绝启动。

**常见原因**：`swapoff -a` 未执行；`/etc/fstab` 仍有 Swap；ZRAM/Cloud-init 重建 Swap。

**检查命令**：

```bash
swapon --show
systemctl list-units --type=swap
grep -nE '[[:space:]]swap[[:space:]]' /etc/fstab
```

**解决方向**：备份 `/etc/fstab` 后只注释 Swap 条目；关闭 ZRAM/Swap Unit；不要覆盖 Fstab。

**恢复验证**：`swapon --show` 无输出，重启后仍无 Swap，kubeadm Preflight 通过。

## 7. SELinux 阻断

**现象**：文件、Socket、CNI Interface 或 Mount 明明存在，但组件收到 Permission Denied；Audit Log 有 AVC。

**常见原因**：节点仍为 Enforcing；Policy 不支持当前组件路径；Context 错误。

**检查命令**：

```bash
getenforce
sudo ausearch -m AVC -ts recent
sudo journalctl -t setroubleshoot -n 100 --no-pager
ls -lZ <AFFECTED_PATH>
```

**解决方向**：学习环境按基线切换 Permissive 并保留 Audit；生产环境根据 AVC 构建/采用受支持 Policy。不要关闭 Audit，也不要使用宽泛 Allow Rule。

**恢复验证**：相同操作成功，组件 Ready；记录安全影响和后续恢复 Enforcing 计划。

## 8. firewalld 或 Security Group 阻断

**现象**：同 Node 正常，跨 Node/NLB Timeout；UDP 4789、6443、10250 等端口不可达。

**常见原因**：只配置了 SG 或只配置了 firewalld；Source SG 错误；Zone/Interface 不匹配；端口方向错误。

**检查命令**：

```bash
systemctl status firewalld --no-pager
sudo firewall-cmd --get-active-zones
sudo firewall-cmd --list-all
sudo ss -lntup
nc -zv -w 2 <TARGET_IP> <TCP_PORT>
```

UDP 需要结合两端 `tcpdump -ni any udp port 4789` 和受控测试判断，不能只用 TCP 工具。

**解决方向**：逐层核对 SG、Route、firewalld 和进程监听；按最小 Source/Target 修正规则，不开放 `0.0.0.0/0`。

**恢复验证**：指定端口从指定 Source 可达，非授权 Source 仍被阻止，组件恢复 Ready。

## 9. registry.k8s.io 拉取失败

**现象**：ImagePull、TLS、DNS、Timeout 或 Rate Limit 错误，`kubeadm config images pull` 失败。

**常见原因**：出站网络、DNS/Proxy、CA、Registry 限制或镜像不存在。

**检查命令**：

```bash
kubeadm config images list --kubernetes-version <KUBERNETES_VERSION>
journalctl -u containerd -n 200 --no-pager
sudo crictl --runtime-endpoint unix:///run/containerd/containerd.sock pull <EXACT_IMAGE>
```

**解决方向**：从可信环境按 Digest 同步到组织自己的 Alibaba Cloud ACR，并更新 kubeadm `imageRepository`。不要关闭 TLS，也不要用来源不明 Mirror。

**恢复验证**：全部固定版本镜像在 CRI 中可见，Digest 与清单一致，kubeadm 不再拉取失败。

## 10. NLB 6443 Timeout

**现象**：`nc`、kubectl 或 kubeadm Join 连接 NLB 时超时。

**常见原因**：NLB Zone/VSwitch、Listener、Server Group、Route、SG、firewalld 或 Health Check Source 配置错误。

**检查命令**：

```bash
getent hosts <NLB_PRIVATE_DNS>
nc -zv -w 2 <NLB_PRIVATE_DNS_OR_IP> 6443
sudo ss -lntp | grep ':6443'        # Master 上
sudo tcpdump -ni any tcp port 6443 # 审批后的短时检查
```

**解决方向**：先确认 Backend API 是否监听，再沿 NLB -> VSwitch -> SG -> Host Firewall -> Process 检查。初始化前 Connection Refused 与 Timeout 含义不同。

**恢复验证**：API 启动后 NLB Backend Healthy，`kubectl get --raw=/readyz` 成功。

## 11. 6443 Connection Refused

**现象**：能快速到达目标，但收到 Connection Refused。

**常见原因**：初始化前 API 尚未监听（可预期）；初始化后 kube-apiserver Static Pod 未运行、崩溃或绑定错误地址。

**检查命令**：

```bash
sudo ss -lntp | grep ':6443'
sudo crictl --runtime-endpoint unix:///run/containerd/containerd.sock ps -a
sudo crictl --runtime-endpoint unix:///run/containerd/containerd.sock logs <APISERVER_CONTAINER_ID>
journalctl -u kubelet -n 200 --no-pager
```

**解决方向**：初始化前继续到正确步骤；初始化后修复 Static Pod Manifest、Certificate、Advertise Address、etcd 或镜像问题。

**恢复验证**：本机 6443 监听，NLB Backend Healthy，Readyz 成功。

## 12. Calico Pod 异常

**现象**：Tigera/Calico Pod Pending、CrashLoopBackOff，Node NetworkPluginNotReady。

**常见原因**：错误 Installation CR、Pod CIDR 不一致、Kernel/Sysctl、NetworkManager、SELinux、Image、VXLAN 或 API 权限。

**检查命令**：

```bash
kubectl get tigerastatus
kubectl describe tigerastatus calico
kubectl get pods -n tigera-operator -o wide
kubectl get pods -n calico-system -o wide
kubectl describe pod -n calico-system <POD>
kubectl logs -n calico-system <POD> --all-containers --tail=200
kubectl get ippools.crd.projectcalico.org -o yaml
```

**解决方向**：修正 CIDR、Installation CR、Kernel Module、Sysctl、NetworkManager Unmanaged Interface、镜像或网络端口；不要直接编辑 Operator 管理的 Deployment。

**恢复验证**：TigeraStatus Available，Calico Pod Ready，Node Ready，跨节点 Pod 测试成功。

## 13. VXLAN UDP 4789 被阻断

**现象**：同节点 Pod 通信正常，跨节点 Pod 不通；Calico Pod 可能仍显示 Running。

**常见原因**：SG/firewalld 未允许 Node-to-Node UDP 4789；MTU 不匹配；`vxlan.calico` Interface 异常。

**检查命令**：

```bash
ip -d link show vxlan.calico
ip route
sudo tcpdump -ni any udp port 4789
kubectl get pods -o wide
```

**解决方向**：只在 Kubernetes Node SG 间双向放行 UDP 4789；核对 MTU 与 Underlay；检查 Calico Felix Log。

**恢复验证**：不同 Worker 上的 Pod IP 双向可达，抓包能看到封装/解封装，未向公网开放端口。

## 14. CoreDNS Pending

**现象**：CoreDNS Pod 一直 Pending，Cluster DNS 不可用。

**常见原因**：CNI 尚未安装；Node 未 Ready；Taint/Toleration、Resource 或 Image Pull 问题。

**检查命令**：

```bash
kubectl get pods -n kube-system -l k8s-app=kube-dns -o wide
kubectl describe pod -n kube-system <COREDNS_POD>
kubectl get nodes
kubectl get events -n kube-system --sort-by=.lastTimestamp
```

**解决方向**：先修复 Calico/Node Ready，再处理调度、Image 或 Resource。CNI 安装前 Pending 是预期现象。

**恢复验证**：CoreDNS Pod Running/Ready，kube-dns Service 有 Endpoint，DNS 测试成功。

## 15. CoreDNS CrashLoopBackOff

**现象**：CoreDNS 启动后反复退出。

**常见原因**：Corefile 错误、Upstream DNS Loop、无法访问 API、Plugin 配置或 NetworkPolicy 阻断。

**检查命令**：

```bash
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=200
kubectl get configmap coredns -n kube-system -o yaml
kubectl describe pod -n kube-system <COREDNS_POD>
kubectl get endpointslice -n default kubernetes -o yaml
```

**解决方向**：恢复 kubeadm 默认 Corefile 后逐项引入定制；修复 Upstream Resolver 或 API Connectivity。不要把 Kubeconfig/Secret 输出到工单。

**恢复验证**：Deployment Available，Pod 不再重启，内置和 Service DNS 均能解析。

## 16. Pod 跨节点不通

**现象**：同节点 Pod 通，跨节点 Pod Timeout。

**常见原因**：VXLAN 4789、MTU、Route、IPPool、Node IP Autodetection、firewalld 或 NetworkPolicy。

**检查命令**：

```bash
kubectl get pods -o wide
kubectl get nodes -o wide
kubectl get ippools.crd.projectcalico.org -o yaml
ip -d link show vxlan.calico
ip route
sudo tcpdump -ni any udp port 4789
```

**解决方向**：先排除 Policy，再检查 VXLAN Underlay、Node IP 与 MTU。禁止通过全局 Allow Policy 长期绕过。

**恢复验证**：固定到不同 App Worker 的测试 Pod 双向访问成功，Policy 仍按预期生效。

## 17. ClusterIP 不通

**现象**：能直连 Pod IP，但访问 Service ClusterIP 失败。

**常见原因**：Service Selector/TargetPort、EndpointSlice、kube-proxy、Netfilter/Sysctl 或 NetworkPolicy。

**检查命令**：

```bash
kubectl get svc <SERVICE> -o yaml
kubectl get endpointslice -l kubernetes.io/service-name=<SERVICE> -o yaml
kubectl get pods -n kube-system -l k8s-app=kube-proxy -o wide
kubectl logs -n kube-system -l k8s-app=kube-proxy --tail=200
sysctl net.ipv4.ip_forward
```

**解决方向**：先修复 Selector/Endpoint，再检查 kube-proxy 与 Host Netfilter；确认 ClusterIP 属于 Service CIDR。

**恢复验证**：Client Pod 通过 ClusterIP 和 Service DNS 均访问成功，EndpointSlice Ready。

## 18. Service DNS 不通

**现象**：ClusterIP 可访问，但 `service.namespace.svc.cluster.local` 解析失败。

**常见原因**：CoreDNS、kube-dns Service/Endpoint、Pod Resolver、NetworkPolicy 阻断 UDP/TCP 53 或错误 Search Domain。

**检查命令**：

```bash
kubectl get svc kube-dns -n kube-system
kubectl get endpointslice -n kube-system -l k8s-app=kube-dns
kubectl exec <CLIENT_POD> -- cat /etc/resolv.conf
kubectl exec <CLIENT_POD> -- nslookup kubernetes.default.svc.cluster.local
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=200
```

**解决方向**：修复 CoreDNS、DNS Policy、kube-dns Endpoint 或 NetworkPolicy；不要把 Node `/etc/resolv.conf` 无条件复制进 Pod。

**恢复验证**：内置 API Service 和业务测试 Service 均可解析并访问。

## 19. Join Token 过期

**现象**：Join 返回 Token Invalid/Expired 或 Unauthorized。

**常见原因**：Bootstrap Token 默认 TTL 到期，或复制时损坏。

**检查命令**：

```bash
sudo kubeadm token list
```

**解决方向**：在健康 Control Plane 上执行 `sudo kubeadm token create --print-join-command`，通过受控渠道短期传递。不要延长为无限期 Token。

**恢复验证**：新 Token 能完成指定 Node Join，完成后按策略删除或等待过期。

## 20. Certificate Key 过期

**现象**：Control Plane Join 无法解密/下载 Certificate。

**常见原因**：`--upload-certs` 生成的 Secret/Key 已过默认有效期，或 Key 复制错误。

**检查命令**：检查 Join 时间线和 kubeadm 输出；不要把 Key 写入 Log。

**解决方向**：在健康 Control Plane 执行：

```bash
sudo kubeadm init phase upload-certs --upload-certs
```

取得新的短期 Certificate Key，并重新生成/组合 Control Plane Join Command。

**恢复验证**：新 Master Join 成功，Control Plane Static Pod 与 etcd Member 健康；临时凭证按策略销毁。

## 21. Pod CIDR 与 VPC/Service CIDR 冲突

**现象**：部分地址访问走错 Route、Pod 或 Service 网络不稳定、访问 VPC 资源被黑洞。

**常见原因**：部署前未做 CIDR 交集检查；VPC/CEN/VPN 后续引入了冲突 Route。

**检查命令**：

```bash
kubectl cluster-info dump | grep -E 'cluster-cidr|service-cluster-ip-range'
kubectl get ippools.crd.projectcalico.org -o yaml
ip route
```

并在阿里云控制台核对 VPC/VSwitch/CEN/VPN Route。

**解决方向**：新集群应在初始化前重新规划。已运行集群修改 Pod/Service CIDR 属于高风险重建/迁移，不应现场修改几条 Route 规避。

**恢复验证**：通过独立实验环境完成无冲突规划、重建并通过全部网络验收。

## 22. etcd Unhealthy

**现象**：API Server Readyz 的 etcd Check 失败、请求超时、etcd Pod 重启或失去 Quorum。

**常见原因**：同时失去两个 Member；磁盘 I/O/空间；时钟；2379/2380；Certificate；Data Corruption。

**检查命令**：

```bash
kubectl get pods -n kube-system -l component=etcd -o wide
kubectl get --raw='/readyz?verbose'
df -h /var/lib/etcd
df -i /var/lib/etcd
journalctl -u kubelet -n 200 --no-pager
```

可选 `etcdctl endpoint health/member list` 使用 `/etc/kubernetes/pki/etcd` 下 kubeadm Certificate；Private Key 不得复制。一次只检查/维护一个 Member。

**解决方向**：先恢复 Quorum、磁盘、网络或证书。没有 Snapshot/Member 状态分析，不得删除 `/var/lib/etcd` 或执行 Member Remove。

**恢复验证**：3 Member 状态明确，Quorum 正常，Readyz 全部通过，API 延迟恢复，并立即检查/执行备份计划。

# 受控重建与回滚

## Worker 加入失败

仅适用于确认未承载业务、没有独立数据且需要重新 Join 的 Worker。

> **WARNING：以下操作会删除该 Worker 的 kubeadm/kubelet/CNI 本地状态，使其退出集群。必须确认目标 Hostname/IP，完成审批，并确保不是 Control Plane 或有数据节点。**

1. 如果 Node Object 已存在，先在 Control Plane 确认没有 Workload，并按组织流程 Drain/Delete Node。
2. 在目标 Worker 执行：

```bash
sudo kubeadm reset -f --cri-socket unix:///run/containerd/containerd.sock
```

3. 检查残留，再决定是否清理 CNI：

```bash
sudo find /etc/cni/net.d -mindepth 1 -maxdepth 1 -print
sudo find /var/lib/cni -mindepth 1 -maxdepth 1 -print
```

> **WARNING：只有确认目录内全是该失败 Cluster 的 CNI 状态后，才允许清理以下精确目录内容。不要删除上级目录或其他路径。**

```bash
sudo rm -rf /etc/cni/net.d/*
sudo rm -rf /var/lib/cni/*
sudo systemctl restart containerd kubelet
```

4. 不要全局 Flush iptables/nftables。若 Calico 留下 Host Network State，应按该 Calico Release 官方卸载/重置流程处理。
5. 修复根因后使用新 Token 重新 Join，并验证 Node/Calico Ready。

## Control Plane 加入失败

> **WARNING：Control Plane 同时包含 etcd。不能直接删除 `/var/lib/etcd`，不能直接 `rm -rf /etc/kubernetes`，也不能把 Worker Reset 流程套用到 Master。**

先判断：

- Join 在创建 etcd Member 前失败，还是已经成为 Member？
- Cluster 当前有几个健康 Member？Quorum 是否存在？
- API Server/etcd Static Pod 是否已经运行？
- NLB 是否已把该 Backend 标记 Healthy？

使用 `kubectl get pods -n kube-system -l component=etcd -o wide` 和受控 `etcdctl member list` 判断。涉及 Member Remove、Certificate 恢复或 etcd Data Directory 的操作必须采用独立恢复计划和备份，不在本手册给出删除命令。

## 整个实验集群重新初始化

> **WARNING：这是破坏性操作。将丢失 Kubernetes API 中的全部对象、etcd 数据、Cluster Certificate、Token、Node 注册、CNI 状态，以及未独立备份的 Local/Persistent Workload 数据。必须先做 etcd/业务数据备份并验证恢复路径。**

需要清理的范围（经审批后逐节点）：

- kubeadm Cluster State 与 Static Pod。
- kubelet 本地状态。
- containerd 中属于旧 Cluster 的 Sandbox/Container/Image（是否删除 Image 由策略决定）。
- Calico CNI 本地状态与旧 VXLAN/Route（按官方 Release 流程）。
- 旧 Kubeconfig 和短期 Credential。

通常不需要删除的阿里云资源：

- ECS、VPC、VSwitch、Security Group。
- Internal NLB、Private DNS（若新 Cluster 复用完全相同 Endpoint）。
- 独立 RDS/Tair。
- 已确认需要保留的备份和 `ops-storage`。

可能永久丢失的数据：

- `/var/lib/etcd` 中未备份的 Cluster State。
- `worker-data-01` Local Volume 或未做 Snapshot 的云盘数据。
- Prometheus 本地历史、应用日志和集群内 Secret。

推荐流程：冻结变更 -> 导出不含 Secret 的资源清单 -> 备份 etcd/业务数据 -> 验证备份 -> 记录云资源 -> 逐 Worker Reset -> 逐 Control Plane 受控 Reset -> 清理旧 NLB Backend Health State -> 按部署手册重新初始化 -> 完整验收。不得边排障边临时重建生产集群。

## 官方参考

- [Kubernetes kubeadm Troubleshooting](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/troubleshooting-kubeadm/)
- [Kubernetes kubeadm Administration](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/)
- [Kubernetes Cgroup Driver](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/configure-cgroup-driver/)
- [Calico 官方文档入口](https://docs.tigera.io/calico/)
- [Alibaba Cloud NLB Health Check](https://www.alibabacloud.com/help/en/slb/network-load-balancer/user-guide/nlb-health-check-overview/)
