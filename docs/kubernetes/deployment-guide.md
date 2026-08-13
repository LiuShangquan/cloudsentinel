# Kubernetes 集群部署手册（Alibaba Cloud Linux 4）

> 状态：集群建设命令未执行、未验证。本文只用于建立 Kubernetes 与平台前置条件；应用部署采用 Argo CD，见 `../gitops.md`。执行者必须先替换全部 `<PLACEHOLDER>`、确认版本兼容性、完成变更审批并准备回滚。禁止把 Token、Certificate Key、Kubeconfig、SSH Key 或密码提交到 Git。

## 使用约定

- `[所有 Kubernetes Node]`：3 个 Master + 4 个 Worker。
- `[仅 Control Plane]`：`master-01/02/03`。
- `[仅 master-01]`：第一个初始化节点。
- `[对应节点]`：只在标题或命令指定的那一台执行。
- `[运维终端]`：已安装 kubectl 且能通过 Internal NLB 访问 API Server 的受控终端，通常为 `ops-storage`。
- 命令中的 `sudo` 表示需要 Root 权限；不要直接长期使用 Root SSH 登录。
- 每一节都应在成功后再进入下一节。出现异常时先查看“失败时看哪里”，不要跳过 Preflight Check。

## 0. 准备变量

**目标**：在执行任何变更前冻结网络、节点和版本变量。

**执行节点**：先在受控运维记录中填写；需要命令时，在当前操作节点的临时 Shell 中设置。不要把含 Secret 的环境文件提交 Git。

**命令**：

```bash
export CLUSTER_NAME="cloudsentinel-k8s"
export CONTROL_PLANE_ENDPOINT="<NLB_PRIVATE_IP_OR_DNS>:6443"
export NLB_HOST="<NLB_PRIVATE_IP_OR_DNS>"

export VPC_CIDR="<VPC_CIDR>"
export POD_CIDR="<POD_CIDR>"
export SERVICE_CIDR="<SERVICE_CIDR>"

# 语义版本，例如 v1.xx.y；不要使用浮动版本标识。
export KUBERNETES_MINOR="v1.<MINOR>"
export KUBERNETES_VERSION="v1.<MINOR>.<PATCH>"
export KUBERNETES_RPM_VERSION="<FULL_KUBERNETES_RPM_VERSION>"
export CONTAINERD_VERSION="<FULL_CONTAINERD_RPM_VERSION>"
export CALICO_VERSION="v<CALICO_VERSION>"

export CRI_SOCKET="unix:///run/containerd/containerd.sock"
export KUBERNETES_IMAGE_REPOSITORY="registry.k8s.io"

export MASTER_01_IP="<MASTER_01_PRIVATE_IP>"
export MASTER_02_IP="<MASTER_02_PRIVATE_IP>"
export MASTER_03_IP="<MASTER_03_PRIVATE_IP>"
export WORKER_APP_01_IP="<WORKER_APP_01_PRIVATE_IP>"
export WORKER_APP_02_IP="<WORKER_APP_02_PRIVATE_IP>"
export WORKER_MONITOR_IP="<WORKER_MONITOR_PRIVATE_IP>"
export WORKER_DATA_01_IP="<WORKER_DATA_01_PRIVATE_IP>"
export OPS_STORAGE_IP="<OPS_STORAGE_PRIVATE_IP>"

# 验收测试必须使用可信仓库和固定版本或 Digest。
export TEST_TOOLBOX_IMAGE="<TRUSTED_TOOLBOX_IMAGE_WITH_VERSION_OR_DIGEST>"
export TEST_HTTP_IMAGE="<TRUSTED_HTTP_IMAGE_WITH_VERSION_OR_DIGEST>"
```

**命令作用**：显式记录后续文档使用的唯一值，避免不同节点使用不同 CIDR、Endpoint 或组件版本。

**预期结果**：所有占位符都有经审批的真实值；VPC、Pod、Service CIDR 互不重叠；版本均为固定值。

**如何检查**：

```bash
printf '%s\n' "$CONTROL_PLANE_ENDPOINT" "$POD_CIDR" "$SERVICE_CIDR"
printf '%s\n' "$KUBERNETES_VERSION" "$CONTAINERD_VERSION" "$CALICO_VERSION"
```

不得在共享日志中执行 `env` 或 `export -p`，避免未来环境变量中出现 Secret。

**失败时看哪里**：回到 Inventory、阿里云 VPC/VSwitch/NLB 页面和版本选择记录；任何未知值都应阻止继续执行。

## 1. 阿里云基础设施确认

**目标**：确认 ECS、VPC、VSwitch、Security Group、Internal NLB 和 Private DNS 已满足设计，而不在节点初始化期间临时改网络。

**执行节点**：阿里云控制台；本节没有服务器命令。

**操作**：

1. 确认 7 台 Kubernetes ECS 均为 Alibaba Cloud Linux 4 LTS 64 位普通版、固定私网 IP，并位于目标 VPC；`ops-storage` 使用相同操作系统但不加入集群。
2. 确认 Control Plane 尽可能分布在不同可用区，NLB 在对应 Zone/VSwitch 启用。
3. 确认 Security Group 符合 `security-and-network.md`，尤其是 2379/2380 和 UDP 4789。
4. 创建 Internal NLB、TCP 6443 Listener、TCP 6443 Server Group 和 Health Check。
5. 添加 3 个 Master 为 Backend；在 API Server 启动前允许其显示不健康。
6. 确认 Private DNS（推荐）解析到 NLB；没有 DNS 时记录 NLB 私网 IP。
7. 确认 `ops-storage` 不加入 Kubernetes，但可以按最小权限 SSH 到节点并访问 NLB 6443。

**命令作用**：无；本节建立后续 kubeadm 所依赖的云侧基础设施。

**预期结果**：Inventory 中每个地址都可在控制台对应到唯一资源，NLB 与 Backend 配置完整，未开放不必要的公网规则。

**如何检查**：导出或截图 VPC Route、VSwitch、SG Rule、NLB Listener/Server Group/Health Check 配置，交叉检查 CIDR 与 Backend IP。

**失败时看哪里**：阿里云 NLB 事件、Server Group Health、Security Group Rule、VSwitch Route、PrivateZone 解析记录。不要通过放开 `0.0.0.0/0` 绕过问题。

## 2. 配置 Hostname

**目标**：为每台节点设置唯一且稳定的 Kubernetes Node Name。

**执行节点**：`[对应节点]`。

**命令**：

```bash
# master-01
sudo hostnamectl set-hostname master-01

# 其余节点分别执行对应的一条
sudo hostnamectl set-hostname master-02
sudo hostnamectl set-hostname master-03
sudo hostnamectl set-hostname worker-app-01
sudo hostnamectl set-hostname worker-app-02
sudo hostnamectl set-hostname worker-monitor
sudo hostnamectl set-hostname worker-data-01
```

**命令作用**：写入 Static Hostname；kubeadm 默认使用该名称注册 Node。

**预期结果**：每台机器的 Hostname 与 Inventory 完全一致且全局唯一。

**如何检查**：

```bash
hostnamectl status
hostname -s
```

**失败时看哪里**：检查 `/etc/hostname`、Cloud-init 是否会在重启时覆盖 Hostname，以及是否存在重复名称。

## 3. 配置 Hosts 或内部 DNS

**目标**：确保所有节点对 Node Name 和 Control Plane DNS 的解析一致。

**执行节点**：`[所有 Kubernetes Node]`；Private DNS 由阿里云控制台管理。

**推荐方案**：使用内部 DNS 为所有节点和 NLB 提供 A/CNAME 记录。

**学习环境 `/etc/hosts` 模板**：先备份，再由人工编辑追加，不得覆盖原文件。

```bash
sudo cp /etc/hosts /etc/hosts.pre-k8s
sudo vi /etc/hosts
```

追加：

```text
<MASTER_01_IP> master-01
<MASTER_02_IP> master-02
<MASTER_03_IP> master-03
<WORKER_APP_01_IP> worker-app-01
<WORKER_APP_02_IP> worker-app-02
<WORKER_MONITOR_IP> worker-monitor
<WORKER_DATA_01_IP> worker-data-01
<OPS_STORAGE_IP> ops-storage
```

如果 `CONTROL_PLANE_ENDPOINT` 使用 DNS，该 DNS 必须解析到 Internal NLB，不能在 `/etc/hosts` 中错误映射到 `master-01`。

**命令作用**：提供稳定一致的名称解析。

**预期结果**：每个名称解析为对应私网 IP；API DNS 解析到 NLB。

**如何检查**：

```bash
getent hosts master-01
getent hosts worker-app-01
getent hosts "${NLB_HOST}"
```

**失败时看哪里**：`/etc/nsswitch.conf`、`/etc/resolv.conf`、NetworkManager Connection DNS、PrivateZone 关联的 VPC 和重复 `/etc/hosts` 条目。

## 4. 时间同步

**目标**：使用 Chrony 保持所有 Node 时间一致，避免证书、etcd Lease 和日志时间线异常。

**执行节点**：`[所有 Kubernetes Node]`。

**命令**：

```bash
sudo timedatectl set-timezone Asia/Shanghai
sudo dnf install -y chrony
sudo systemctl enable --now chronyd
timedatectl status
chronyc tracking
chronyc sources -v
```

**命令作用**：设置显示时区并启用 NTP 同步；Kubernetes 内部时间仍应按 UTC 理解。

**预期结果**：`System clock synchronized: yes`，Chrony 有可用 Source，节点时间偏差在组织允许范围内。

**如何检查**：对比所有节点 `date --iso-8601=seconds` 与 `chronyc tracking`。

**失败时看哪里**：`journalctl -u chronyd`、DNS、NTP UDP 123 出站策略和组织内部 NTP 地址。

## 5. SELinux 与 firewalld 策略

**目标**：在明确安全影响的前提下配置 Host 安全层，避免 CNI/kubelet 被意外阻断。

**执行节点**：`[所有 Kubernetes Node]`。

**SELinux 命令（基线为 Permissive）**：

```bash
getenforce
sudo setenforce 0
sudo cp /etc/selinux/config /etc/selinux/config.pre-k8s
sudo sed -ri 's/^SELINUX=.*/SELINUX=permissive/' /etc/selinux/config
grep '^SELINUX=' /etc/selinux/config
```

**firewalld 学习环境方案**：仅在 Security Group 已严格控制边界并完成审批时选择。

```bash
sudo systemctl disable --now firewalld
systemctl is-enabled firewalld
systemctl is-active firewalld
```

**firewalld 严格方案**：按 `security-and-network.md` 的节点角色开放端口，并额外验证 Calico Interface 与 Forward Policy。不要同时执行“关闭”和“保留”两套方案。

**命令作用**：SELinux Permissive 保留审计但不强制阻断；firewalld 策略明确 Host Firewall 与 Cloud SG 的职责。

**预期结果**：`getenforce` 为 `Permissive`；firewalld 状态与已选择方案一致；没有为了排障把两层 Firewall 全部放开。

**如何检查**：

```bash
getenforce
systemctl status firewalld --no-pager
sudo ausearch -m AVC -ts recent
```

**失败时看哪里**：`journalctl -u firewalld`、`ausearch -m AVC`、Security Group Flow/Rule 与 Calico 系统要求。

**CentOS Stream 10 Compatibility Note**：重新确认 SELinux Policy、firewalld/nftables Backend 和 Calico 对当前内核的支持，不要假定 Stream 9 行为完全一致。

## 6. 禁用 Swap

**目标**：符合 kubeadm 默认 Preflight 要求，并避免未显式配置的 kubelet Swap 行为。

**执行节点**：`[所有 Kubernetes Node]`。

**命令**：

```bash
swapon --show
sudo swapoff -a

# 永久修改前必须备份
sudo cp /etc/fstab /etc/fstab.pre-k8s
sudo vi /etc/fstab
```

在编辑器中只注释实际的 Swap 条目，不要覆盖整个 `/etc/fstab`，不要删除 Root/Data Disk Mount。

**命令作用**：立即停用 Swap，并通过安全编辑避免重启后重新启用。

**预期结果**：`swapon --show` 无输出；`/etc/fstab` 的 Swap 条目被注释，备份存在。

**如何检查**：

```bash
swapon --show
grep -nE '[[:space:]]swap[[:space:]]' /etc/fstab
sudo test -f /etc/fstab.pre-k8s
```

**失败时看哪里**：`systemctl list-units --type=swap`、ZRAM Service、Cloud-init 和 `/etc/fstab` 语法。若误改，使用备份恢复并先验证 `findmnt --verify`。

## 7. 加载 Kernel Module

**目标**：启用 OverlayFS 与 Bridge Netfilter 支持，并持久化到重启后。

**执行节点**：`[所有 Kubernetes Node]`。

**命令**：

```bash
sudo modprobe overlay
sudo modprobe br_netfilter

sudo tee /etc/modules-load.d/k8s.conf >/dev/null <<'EOF'
overlay
br_netfilter
EOF

lsmod | grep -E 'overlay|br_netfilter'
```

**命令作用**：`overlay` 支持容器 Snapshot；`br_netfilter` 让 Bridge 流量进入 Netfilter Hook。

**预期结果**：两个 Module 均已加载，持久化文件内容正确。

**如何检查**：`lsmod` 和 `modinfo overlay`、`modinfo br_netfilter`。

**失败时看哪里**：当前内核配置、`journalctl -k`、模块包是否完整，以及是否运行了非标准精简内核。

## 8. 配置 Sysctl

**目标**：允许 IPv4 Forward，并让 Bridge 流量经过 iptables/ip6tables。

**执行节点**：`[所有 Kubernetes Node]`。

**命令**：

```bash
sudo tee /etc/sysctl.d/99-kubernetes.conf >/dev/null <<'EOF'
net.ipv4.ip_forward = 1
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

sudo sysctl --system
```

**命令作用**：`ip_forward` 允许 Pod/Service 流量路由；Bridge 参数让 Netfilter/NetworkPolicy 能看到 Bridge 流量。

**预期结果**：三个参数值均为 1，`sysctl --system` 无 Unknown Key。

**如何检查**：

```bash
sysctl net.ipv4.ip_forward
sysctl net.bridge.bridge-nf-call-iptables
sysctl net.bridge.bridge-nf-call-ip6tables
```

**失败时看哪里**：确认 `br_netfilter` 已加载，检查其他 `/etc/sysctl.d/*.conf` 是否覆盖值，以及 `journalctl -k`。

## 9. 安装 containerd

**目标**：从受信 RPM Repository 安装固定版本 containerd，而不安装或依赖 Docker Engine。

**执行节点**：`[所有 Kubernetes Node]`。

**版本确认命令**：

```bash
cat /etc/os-release
uname -m
dnf repolist
dnf list --showduplicates containerd
dnf list --showduplicates containerd.io
dnf info containerd
dnf info containerd.io
```

Alibaba Cloud Linux 4 不是 CentOS Stream 9。不要直接添加 Docker CentOS Repository 或强装 EL9 RPM；先用当前系统仓库枚举可用的 `containerd`/`containerd.io` 包，确认包名、版本、来源和 Kubernetes CRI 兼容性后再更新本节安装命令。当前系统基线变更后的运行时安装步骤仍为 `NOT VERIFIED`，在版本冻结前只执行上述查询，不执行安装。

**命令作用**：Docker 的 CentOS RPM Repository 同时提供独立 `containerd.io` Package；Kubernetes 使用 containerd CRI，不要求运行 Docker Engine 或 Dockershim。

**预期结果**：查询结果能确定唯一受信软件源、实际包名和可固定的完整版本；安装尚未发生。

**如何检查**：记录 `dnf repolist` 与两个候选包的查询结果，确认后再把安装命令和完整 NEVRA 写回本手册。

**失败时看哪里**：Repository GPG Key、Proxy/DNS、Alibaba Cloud Linux 4 Package 可用性、架构和冲突 Package。不要改用来源不明或为其他发行版构建的 RPM。

## 10. 配置 containerd 与 systemd cgroup

**目标**：启用 CRI，并使 containerd 与 kubelet 都使用 `systemd` Cgroup Driver。

**执行节点**：`[所有 Kubernetes Node]`。

**确认 Cgroup v2**：

```bash
stat -fc %T /sys/fs/cgroup
mount | grep cgroup
```

`cgroup2fs` 表示统一 Cgroup v2。Alibaba Cloud Linux 4 默认启用 Cgroup v2，但必须以每台节点的实际输出为准。

**生成并检查配置**：

```bash
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml >/dev/null
containerd --version
grep -nE 'version =|disabled_plugins|SystemdCgroup|io.containerd.*cri' /etc/containerd/config.toml
sudo vi /etc/containerd/config.toml
```

在人工编辑中完成两件事：

1. 确认 `cri` 不在 `disabled_plugins` 中。
2. 在当前 containerd 主版本生成的 runc Runtime Option 下设置 `SystemdCgroup = true`。

containerd 不同主版本的 Plugin Path 可能不同；不要使用版本无关的盲目 `sed`。必须先查看 `containerd --version` 和生成的 Default Config，再对照该版本官方文档确认位置。

**启动命令**：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now containerd
sudo systemctl restart containerd
sudo systemctl status containerd --no-pager
sudo containerd config dump | grep -n -A8 -B4 SystemdCgroup
sudo ctr version
```

**命令作用**：启用 CRI Socket `/run/containerd/containerd.sock`，并让 Runtime 使用 systemd 管理 Cgroup。

**预期结果**：containerd Active，CRI 未禁用，SystemdCgroup 为 True，Socket 存在。

**如何检查**：

```bash
sudo test -S /run/containerd/containerd.sock
sudo journalctl -u containerd -n 100 --no-pager
```

安装 `cri-tools` 后还需执行 `crictl --runtime-endpoint "$CRI_SOCKET" info`。

**失败时看哪里**：`journalctl -u containerd`、TOML 语法、Plugin ID、Snapshotter、SELinux AVC、磁盘空间和 Socket 权限。

## 11. 配置 Kubernetes RPM Repository

**目标**：使用 `pkgs.k8s.io` 中与目标 Minor Version 对应的 RPM Repository；不使用已废弃的旧 Google Yum 软件源。

**执行节点**：`[所有 Kubernetes Node]`。

**命令**：先把 `KUBERNETES_MINOR` 设置为 `v1.xx`，然后创建 Repository。

```bash
sudo tee /etc/yum.repos.d/kubernetes.repo >/dev/null <<EOF
[kubernetes]
name=Kubernetes ${KUBERNETES_MINOR}
baseurl=https://pkgs.k8s.io/core:/stable:/${KUBERNETES_MINOR}/rpm/
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/${KUBERNETES_MINOR}/rpm/repodata/repomd.xml.key
exclude=kubelet kubeadm kubectl cri-tools kubernetes-cni
EOF

sudo dnf clean all
sudo dnf makecache
dnf repolist
dnf --showduplicates list kubeadm --disableexcludes=kubernetes
```

**命令作用**：Kubernetes Package Repository 按 Minor Version 分离，防止在同一个 Repo 中意外跨 Minor 升级。

**预期结果**：Repo URL 中 Minor 与目标一致，能看到多个可用 Patch 的 kubeadm RPM。

**如何检查**：核对 `/etc/yum.repos.d/kubernetes.repo` 中的 Minor、GPG 配置和 `exclude`，并确认 `dnf --showduplicates list kubeadm --disableexcludes=kubernetes` 返回目标 Patch。

**如何查询目标 Patch**：

1. 查看 Kubernetes Version Skew Policy 与目标 Minor 的 Release/Support 状态。
2. 使用上面的 `dnf --showduplicates list` 查询 Repo 实际可用 RPM。
3. 选择同一个 Patch 对应的 kubeadm、kubelet、kubectl 完整 RPM 版本。
4. 将语义版本写入 `KUBERNETES_VERSION`，RPM 字符串写入 `KUBERNETES_RPM_VERSION`。

**失败时看哪里**：Minor URL、GPG Key、DNS/Proxy、系统时间和 CA Certificate。不要通过关闭 GPG Check 绕过错误。

**CentOS Stream 10 Compatibility Note**：DNF5 使用 `--setopt=disable_excludes=kubernetes` 的场景可能取代 `--disableexcludes=kubernetes`。实际执行前运行 `dnf5 --help` 并对照当时 Kubernetes 官方 RPM 指南。

## 12. 安装 kubeadm、kubelet、kubectl

**目标**：在所有节点安装完全相同的固定 Patch Version。

**执行节点**：`[所有 Kubernetes Node]`。

**命令**：

```bash
dnf --showduplicates list kubeadm kubelet kubectl --disableexcludes=kubernetes

sudo dnf install -y \
  "kubeadm-${KUBERNETES_RPM_VERSION}" \
  "kubelet-${KUBERNETES_RPM_VERSION}" \
  "kubectl-${KUBERNETES_RPM_VERSION}" \
  --disableexcludes=kubernetes

sudo systemctl enable kubelet
kubeadm version -o short
kubelet --version
kubectl version --client=true
rpm -q kubeadm kubelet kubectl cri-tools kubernetes-cni
```

**命令作用**：安装 Node Bootstrap、Node Agent 和客户端工具。kubelet 在 kubeadm 写入配置前可能反复重启，这是预期状态，不代表集群已建立。

**预期结果**：三个工具显示同一个 `v1.<minor>.<patch>`；安装版本与变量一致。

**如何检查**：比较所有节点 `rpm -q` 输出；任何节点不同都必须先修复。

**失败时看哪里**：Package Epoch/Release 字符串、Repository Exclude、DNF Transaction Log 和 GPG 校验。

## 13. 锁定版本

**目标**：阻止日常 `dnf upgrade` 自动漂移 kubeadm、kubelet、kubectl 及相关 Kubernetes Package。

**执行节点**：`[所有 Kubernetes Node]`。

**Alibaba Cloud Linux 4 命令（执行前先确认插件包名）**：

```bash
sudo dnf install -y python3-dnf-plugin-versionlock
sudo dnf versionlock add kubeadm kubelet kubectl cri-tools kubernetes-cni
sudo dnf versionlock list
```

Repository 中的 `exclude=` 是第一道保护；Version Lock 是第二道保护。后续升级必须按 Kubernetes Upgrade 流程逐版本解除和重新锁定，不能直接批量升级所有节点。

**预期结果**：Version Lock List 包含已安装 NEVRA；普通 `dnf upgrade` 不会升级这些 Package。

**如何检查**：

```bash
sudo dnf check-update || true
sudo dnf versionlock list
```

检查命令不得实际执行 Upgrade。

**失败时看哪里**：Version Lock Plugin 是否安装、DNF Plugin 是否被禁用、Repository Exclude 和 Package NEVRA。

**CentOS Stream 10 Compatibility Note**：确认 DNF5 Version Lock Plugin 的实际 Package 与 `dnf5 versionlock --help` 输出；不要直接套用 Stream 9 Plugin Package Name。

## 14. 检查并准备 Kubernetes 镜像

**目标**：在 `kubeadm init` 前确认所需固定版本镜像可获取，避免初始化中途失败。

**执行节点**：`[所有 Kubernetes Node]`；至少在所有 Control Plane 完成 Pull。

**命令**：

```bash
kubeadm config images list \
  --kubernetes-version "$KUBERNETES_VERSION" \
  --image-repository "$KUBERNETES_IMAGE_REPOSITORY"

sudo kubeadm config images pull \
  --kubernetes-version "$KUBERNETES_VERSION" \
  --image-repository "$KUBERNETES_IMAGE_REPOSITORY" \
  --cri-socket "$CRI_SOCKET"

sudo crictl --runtime-endpoint "$CRI_SOCKET" images
```

**命令作用**：列出并预拉 API Server、Controller Manager、Scheduler、Proxy、CoreDNS、Pause 和 etcd 等目标版本镜像。

**预期结果**：列表版本与 `KUBERNETES_VERSION` 对应；Pull 无失败；镜像在 containerd CRI Namespace 可见。

**如何检查**：比较 `kubeadm config images list` 与 `crictl images` 的 Repository、Tag 和 Digest，确认所有 Control Plane Node 都具备相同镜像集合。

**registry.k8s.io 不可访问时**：

1. 在可联网且可信的环境取得官方镜像及 Digest。
2. 扫描并同步到组织自己的 Alibaba Cloud ACR。
3. 将 `KUBERNETES_IMAGE_REPOSITORY` 改为 ACR Namespace。
4. 再次执行 `images list/pull` 并核对 Digest。

当前实验集群已于 2026-08-13 实测确认：北京 ECS 拉取
`registry.k8s.io/kube-apiserver:v1.35.7` 时，请求被重定向到
`europe-west3-docker.pkg.dev`，随后发生 TCP 443 超时。因此本项目使用
`.github/workflows/mirror-kubernetes-images.yml`，由 GitHub 托管 Runner
把 kubeadm `v1.35.7` 所需的 7 个固定 `linux/amd64` 镜像同步到北京 ACR。

运行工作流前，必须在命名空间 `cloudsentinel0306` 中创建以下 7 个仓库：

- `kube-apiserver`
- `kube-controller-manager`
- `kube-scheduler`
- `kube-proxy`
- `coredns`
- `pause`
- `etcd`

这些仓库只保存原本公开的 Kubernetes 官方镜像，可设置为公开仓库，
从而允许自建集群通过 VPC Endpoint 匿名拉取；CloudSentinel 业务镜像和
数据镜像仍必须保持私有。公开仓库可能被外部用户拉取并消耗个人版额度，
因此不得在其中放置任何私有制品。

工作流成功后，目标镜像仓库固定为：

```bash
export KUBERNETES_IMAGE_REPOSITORY="crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com/cloudsentinel0306"
```

CoreDNS 的上游路径是 `registry.k8s.io/coredns/coredns`，但 kubeadm 使用
自定义 `imageRepository` 时会请求目标仓库中的 `coredns`。必须以
`kubeadm config images list --image-repository ...` 的输出作为最终目标路径，
不能直接复制上游子路径。同步后的 Push Digest 记录在 GitHub Actions
Step Summary 中，节点拉取后应核对目标 Tag 与 Digest。

不要使用来源不明的公共 Mirror。

**失败时看哪里**：containerd Log、DNS/Proxy、ACR Credential Helper、Registry TLS、Image Repository Path 和目标架构。

## 15. 检查 Internal NLB

**目标**：在 API Server 初始化前确认 NLB 到 3 个 Master 的网络路径正确。

**执行节点**：`[所有 Control Plane]` 检查监听状态；`[运维终端]` 检查 Endpoint。

**命令**：

```bash
# API Server 尚未运行时，Backend 6443 尚无 Listener
sudo ss -lntp | grep ':6443' || true

# 从运维终端检查 NLB；需提前安装 nmap-ncat
nc -zv -w 2 "${NLB_HOST}" 6443
```

**命令作用**：区分网络 Timeout 与 API 尚未监听造成的 Connection Refused。

**预期结果**：初始化前 Connection Refused 可以接受；Timeout 不可接受。NLB 控制台应显示 Backend 不健康但网络路径配置完整。

**如何检查**：Security Group、firewalld、NLB Zone/VSwitch、Server Group Backend IP/Port 和 Private DNS。

**失败时看哪里**：NLB Health Check Detail、Route、SG、Host Firewall 和 `tcpdump -ni any tcp port 6443`。只有在审批下短时抓包，抓包文件不得包含不必要的业务流量。

## 16. 在 master-01 初始化 Control Plane

**目标**：使用完整 kubeadm Config 初始化首个 Control Plane、首个 stacked etcd Member 和 Cluster Bootstrap 配置。

**执行节点**：`[仅 master-01]`。

**执行前检查**：

```bash
hostname -s
getenforce
swapon --show
systemctl is-active containerd
systemctl is-enabled kubelet
stat -fc %T /sys/fs/cgroup
sysctl net.ipv4.ip_forward
sudo ss -lntp | grep -E ':6443|:2379|:2380' || true
kubeadm version -o short
containerd --version
sudo crictl --runtime-endpoint "$CRI_SOCKET" info
```

再次确认 NLB 不存在 Timeout，并确认镜像已准备。

**创建 `kubeadm-config.yaml`**：下面使用 kubeadm `v1beta4`。该 API 适用于支持它的较新 kubeadm；真正执行前必须运行 `kubeadm config print init-defaults` 和查阅所选版本文档，确认 Config API Version，不能假定所有历史 Minor 都支持 `v1beta4`。

```yaml
apiVersion: kubeadm.k8s.io/v1beta4
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: "<MASTER_01_PRIVATE_IP>"
  bindPort: 6443
nodeRegistration:
  name: master-01
  criSocket: "unix:///run/containerd/containerd.sock"
---
apiVersion: kubeadm.k8s.io/v1beta4
kind: ClusterConfiguration
clusterName: cloudsentinel-k8s
kubernetesVersion: "<KUBERNETES_VERSION>"
controlPlaneEndpoint: "<NLB_PRIVATE_IP_OR_DNS>:6443"
imageRepository: "<KUBERNETES_IMAGE_REPOSITORY>"
networking:
  podSubnet: "<POD_CIDR>"
  serviceSubnet: "<SERVICE_CIDR>"
  dnsDomain: cluster.local
apiServer:
  certSANs:
    - "<NLB_PRIVATE_DNS_OR_IP>"
    - "<MASTER_01_PRIVATE_IP>"
    - "<MASTER_02_PRIVATE_IP>"
    - "<MASTER_03_PRIVATE_IP>"
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cgroupDriver: systemd
```

将模板保存为仅 Root 可写的文件：

```bash
sudo install -m 0600 /dev/null /root/kubeadm-config.yaml
sudo vi /root/kubeadm-config.yaml
sudo kubeadm config validate --config /root/kubeadm-config.yaml
sudo kubeadm init --config /root/kubeadm-config.yaml --upload-certs
```

**命令作用**：创建 Cluster CA、API Server、Controller Manager、Scheduler、首个 etcd、Kubeconfig、Bootstrap Token，并把 Control Plane Certificate 临时上传供其他 Master Join。

**预期结果**：`kubeadm init` 成功输出 Worker Join Command、Control Plane Join Command 和 Certificate Key。Certificate Key 默认有效期有限（官方 HA 指南说明通常为 2 小时），必须安全临时保存，绝不提交 Git。

**如何检查**：

```bash
sudo systemctl status kubelet --no-pager
sudo crictl --runtime-endpoint "$CRI_SOCKET" ps -a
sudo ss -lntp | grep -E ':6443|:2379|:2380'
```

**失败时看哪里**：`journalctl -u kubelet`、`journalctl -u containerd`、`crictl ps -a`、Static Pod Log、Preflight 输出、NLB/SG、CRI Socket、Cgroup、Swap 和镜像。不要在不理解错误时直接运行 `kubeadm reset`。

## 17. 配置 kubectl

**目标**：为受控管理员建立访问首个 API Server 的 Kubeconfig。

**执行节点**：先在 `[仅 master-01]`；复制到 `[运维终端]` 时使用组织批准的安全传输方式。

**命令**：

```bash
mkdir -p "$HOME/.kube"
sudo cp -i /etc/kubernetes/admin.conf "$HOME/.kube/config"
sudo chown "$(id -u):$(id -g)" "$HOME/.kube/config"
chmod 0600 "$HOME/.kube/config"

kubectl cluster-info
kubectl get nodes -o wide
kubectl get --raw='/readyz?verbose'
```

**命令作用**：`admin.conf` 包含 Cluster Endpoint、CA 和高权限客户端证书，用于集群管理。

**预期结果**：kubectl 通过 `CONTROL_PLANE_ENDPOINT` 连接；当前只有 `master-01`，在 CNI 安装前 Node 可能为 `NotReady`，CoreDNS 可能 Pending。

**如何检查**：

```bash
kubectl config view --minify
kubectl get nodes
```

确认 Server 指向 NLB，而不是 `master-01` 私网 IP。

**失败时看哪里**：Kubeconfig Permission、Server Endpoint、NLB、API Server Certificate SAN、系统时间和 `kubectl -v=8`。`admin.conf` 等同高权限凭证，绝不能提交 Git、粘贴到工单或长期共享。

## 18. 安装 Calico（Tigera Operator、VXLAN、禁用 BGP）

**目标**：安装与目标 Kubernetes 版本兼容的固定版本 Calico，提供 Pod Network 和 NetworkPolicy。

**执行节点**：`[仅 master-01 或运维终端]`。

**版本选择**：

1. 打开目标 Calico Release 的官方 System Requirements 与 Kubernetes Compatibility 页面。
2. 确认 Kubernetes Minor、CentOS/RHEL、Kernel 与架构受支持。
3. 确认 Operator Installation CR 的字段适用于该 Release。
4. 把 Release Tag 写入 `CALICO_VERSION`；不要使用浮动分支 URL。

Calico 官方推荐 Operator 管理安装生命周期。下载并检查固定版本 Manifest：

```bash
mkdir -p "$HOME/calico-${CALICO_VERSION}"
cd "$HOME/calico-${CALICO_VERSION}"

curl -fL -o calico-crds.yaml \
  "https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/v1_crd_projectcalico_org.yaml"
curl -fL -o tigera-operator.yaml \
  "https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/tigera-operator.yaml"

grep -n 'image:' tigera-operator.yaml
sha256sum calico-crds.yaml tigera-operator.yaml
```

Checksum 应与组织审批记录或自行建立的可信制品清单比较，不能只记录后继续。

**创建 `calico-installation.yaml`**：

```yaml
apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  variant: Calico
  calicoNetwork:
    bgp: Disabled
    ipPools:
      - name: default-ipv4-ippool
        blockSize: 26
        cidr: "<POD_CIDR>"
        encapsulation: VXLAN
        natOutgoing: Enabled
        nodeSelector: all()
```

真正执行前用目标 Release 的 Installation API Reference 校验字段，然后应用：

```bash
kubectl apply -f calico-crds.yaml
kubectl apply -f tigera-operator.yaml
kubectl wait --for=condition=Available deployment/tigera-operator \
  -n tigera-operator --timeout=180s
kubectl apply -f calico-installation.yaml

kubectl get tigerastatus
kubectl get pods -n tigera-operator -o wide
kubectl get pods -n calico-system -o wide
kubectl get ippools.crd.projectcalico.org -o yaml
```

**命令作用**：Operator 创建并管理 Calico 组件；Installation CR 设置 VXLAN、禁用 BGP，并使用与 kubeadm 相同的 Pod CIDR。

**预期结果**：Calico Node 在现有 Node Ready；CoreDNS 从 Pending 转为 Running。CNI 安装前 CoreDNS Pending 是预期现象，因为 Pod Sandbox 无法建立网络。

**如何检查**：

```bash
kubectl get nodes -o wide
kubectl get pods -A -o wide
kubectl describe tigerastatus calico
```

**失败时看哪里**：Operator/Calico Pod Event 和 Log、Pod CIDR 冲突、UDP 4789、Kernel Module、Sysctl、NetworkManager、SELinux、firewalld、镜像仓库和 Installation CR 字段。

## 19. 加入 master-02

**目标**：逐台加入第二个 Control Plane 和第二个 stacked etcd Member。

**执行节点**：生成凭证在 `[master-01]`；Join 在 `[仅 master-02]`。

如果原 Token 过期，在 master-01 重新生成：

```bash
sudo kubeadm token create --print-join-command
```

如果 Certificate Key 过期，重新 Upload Certs：

```bash
sudo kubeadm init phase upload-certs --upload-certs
```

安全组合为以下模板，不要写入文档或 Git：

```bash
sudo kubeadm join <NLB_PRIVATE_IP_OR_DNS>:6443 \
  --token <BOOTSTRAP_TOKEN> \
  --discovery-token-ca-cert-hash sha256:<CA_CERT_HASH> \
  --control-plane \
  --certificate-key <CERTIFICATE_KEY> \
  --cri-socket unix:///run/containerd/containerd.sock
```

- Join Token：短期 Bootstrap Credential。
- Discovery Hash：固定 Cluster CA Public Key，防止连接伪造 API Server。
- Certificate Key：用于解密临时上传的 Control Plane Certificate，必须短期保密。

**命令作用**：注册 Node、下载 Cluster Config、创建 Control Plane Static Pod，并把本机加入 etcd Cluster。

**预期结果**：`master-02` Ready，Control Plane Pod 运行，etcd Member 数变为 2。

**如何检查**：

```bash
kubectl get nodes -o wide
kubectl get pods -n kube-system -o wide
kubectl get --raw='/readyz?verbose'
```

**失败时看哪里**：`journalctl -u kubelet/containerd`、NLB、Token TTL、Discovery Hash、Certificate Key TTL、CRI/Cgroup，以及 etcd Static Pod Log。一次只加入一个 Master。

## 20. 加入 master-03

**目标**：加入第三个 Control Plane，形成 3 Member stacked etcd Quorum。

**执行节点**：生成凭证在 `[master-01]`；Join 在 `[仅 master-03]`。

**命令**：使用第 19 节相同模板，但使用新生成或仍有效的 Token/Certificate Key。在 `master-02` 完全健康前不得开始。

```bash
sudo kubeadm join <NLB_PRIVATE_IP_OR_DNS>:6443 \
  --token <BOOTSTRAP_TOKEN> \
  --discovery-token-ca-cert-hash sha256:<CA_CERT_HASH> \
  --control-plane \
  --certificate-key <CERTIFICATE_KEY> \
  --cri-socket unix:///run/containerd/containerd.sock
```

**命令作用**：形成第三个 API Server 和 etcd Member，使 etcd Quorum 为 2。

**预期结果**：3 个 Master 均 Ready；3 套 Control Plane Static Pod 正常；NLB 3 个 Backend Healthy。

**如何检查**：

```bash
kubectl get nodes -l node-role.kubernetes.io/control-plane -o wide
kubectl get pods -n kube-system -o wide
kubectl get --raw='/readyz?verbose'
```

**失败时看哪里**：与第 19 节一致。不要因第三个节点失败而操作已有健康 Master 的 `/var/lib/etcd`。

## 21. 加入 Worker

**目标**：逐台加入 4 个 Worker。

**执行节点**：在 `[master-01]` 生成 Join Command；分别在 `[worker-app-01]`、`[worker-app-02]`、`[worker-monitor]`、`[worker-data-01]` 执行。

**命令模板**：

```bash
sudo kubeadm join <NLB_PRIVATE_IP_OR_DNS>:6443 \
  --token <BOOTSTRAP_TOKEN> \
  --discovery-token-ca-cert-hash sha256:<CA_CERT_HASH> \
  --cri-socket unix:///run/containerd/containerd.sock
```

Worker 加入路径：

```text
kubelet -> ControlPlaneEndpoint -> API Server -> Node 注册
        -> Calico DaemonSet 建立 CNI -> Node Ready
```

**预期结果**：4 个 Worker 最终均为 Ready；每个 Worker 上有 Calico Node 与 kube-proxy Pod。

**如何检查**：

```bash
kubectl get nodes -o wide
kubectl get pods -n kube-system -o wide
kubectl get pods -n calico-system -o wide
```

**失败时看哪里**：Node 上 `journalctl -u kubelet/containerd`，Cluster 中 `kubectl describe node`、CSR、Calico Pod/Event、NLB 6443 和 UDP 4789。

## 22. 配置 Labels

**目标**：为未来 Workload 提供稳定的 Node Selection 维度。

**执行节点**：`[运维终端]`。

**命令**：

```bash
kubectl label node worker-app-01 node-role=app
kubectl label node worker-app-02 node-role=app
kubectl label node worker-monitor node-role=monitoring
kubectl label node worker-data-01 node-role=data
```

重复执行需要修改值时，先审查再使用 `--overwrite`，不要默认覆盖。

**命令作用**：Label 为 Scheduler 的 `nodeSelector`/Affinity 提供可查询属性。

**预期结果**：4 个 Worker 具有对应且唯一的 `node-role` Value。

**如何检查**：

```bash
kubectl get nodes -L node-role
kubectl get nodes --show-labels
```

**失败时看哪里**：Node Name 拼写、RBAC、受限制 Label Prefix 和已有冲突值。

## 23. 配置 Taints 并验收调度

**目标**：保护 Monitoring/Data 节点，同时保留 Control Plane 默认 Taint。

**执行节点**：`[运维终端]`。

**命令**：

```bash
kubectl taint node worker-monitor dedicated=monitoring:NoSchedule
kubectl taint node worker-data-01 dedicated=data:NoSchedule

kubectl describe node worker-monitor | grep -A2 Taints
kubectl describe node worker-data-01 | grep -A2 Taints
kubectl describe node master-01 | grep -A2 Taints
```

不得删除 `node-role.kubernetes.io/control-plane:NoSchedule`。

**命令作用**：Taint 排斥无 Toleration Pod；Toleration 只赋予进入资格；`nodeSelector` 决定目标节点。

**普通 Pod 调度测试 YAML**：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: schedule-app-test
spec:
  nodeSelector:
    node-role: app
  containers:
    - name: test
      image: <TRUSTED_TOOLBOX_IMAGE_WITH_VERSION_OR_DIGEST>
      command: ["sh", "-c", "sleep 3600"]
  restartPolicy: Never
```

Monitoring 定向测试在 `spec` 中使用：

```yaml
nodeSelector:
  node-role: monitoring
tolerations:
  - key: dedicated
    operator: Equal
    value: monitoring
    effect: NoSchedule
```

Data 测试相同，但 Value 为 `data`。创建后检查 Node，再删除测试 Pod。

**预期结果**：普通 App Pod 只进入 App Worker；没有 Toleration 的 Pod 不能进入 Monitoring/Data；普通 Pod 不进入 Master；带对应 `nodeSelector + toleration` 的 Pod 能进入专用节点。

**如何检查**：`kubectl get pod -o wide`、`kubectl describe pod` 的 Scheduler Event。

**失败时看哪里**：Label/Taint 拼写、Image Pull、Node Resource、Scheduler Event 和 Pod Affinity。

## 24. 检查 CoreDNS

**目标**：确认 CNI 安装后 Cluster DNS Deployment 可用且副本分散合理。

**执行节点**：`[运维终端]`。

**命令**：

```bash
kubectl get deployment coredns -n kube-system
kubectl get pods -n kube-system -l k8s-app=kube-dns -o wide
kubectl get service kube-dns -n kube-system
kubectl rollout status deployment/coredns -n kube-system --timeout=180s
```

**命令作用**：验证 CoreDNS Deployment、Pod 与 kube-dns ClusterIP。

**预期结果**：Desired/Available 副本一致，Pod Running/Ready，不集中在不可调度节点。

**如何检查**：`kubectl describe deployment/pod` 与 `kubectl logs -n kube-system -l k8s-app=kube-dns --tail=100`。

**失败时看哪里**：CNI、Image Pull、ConfigMap `coredns`、Upstream DNS、Taint/Toleration、Resource 和 Pod Event。

## 25. 验收 Pod 跨节点网络

**目标**：证明 Calico VXLAN 支持不同 App Worker 间 Pod-to-Pod 通信。

**执行节点**：`[运维终端]`。

**测试 YAML**：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: pod-net-a
  labels: {test: pod-network}
spec:
  nodeSelector:
    kubernetes.io/hostname: worker-app-01
  containers:
    - name: toolbox
      image: <TRUSTED_TOOLBOX_IMAGE_WITH_VERSION_OR_DIGEST>
      command: ["sh", "-c", "sleep 3600"]
  restartPolicy: Never
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-net-b
  labels: {test: pod-network}
spec:
  nodeSelector:
    kubernetes.io/hostname: worker-app-02
  containers:
    - name: toolbox
      image: <TRUSTED_TOOLBOX_IMAGE_WITH_VERSION_OR_DIGEST>
      command: ["sh", "-c", "sleep 3600"]
  restartPolicy: Never
```

**命令**：

```bash
kubectl apply -f pod-network-test.yaml
kubectl wait --for=condition=Ready pod/pod-net-a pod/pod-net-b --timeout=120s
kubectl get pod pod-net-a pod-net-b -o wide

POD_B_IP="$(kubectl get pod pod-net-b -o jsonpath='{.status.podIP}')"
kubectl exec pod-net-a -- ping -c 3 "$POD_B_IP"

kubectl delete -f pod-network-test.yaml
```

如果固定测试镜像没有 `ping`，使用其中实际提供的 `curl`/`nc` 和临时监听服务；不要临时改用未固定镜像。

**预期结果**：两个 Pod 位于不同 Worker，Pod A 能访问 Pod B IP；删除后无残留。

**如何检查**：确认 Pod 有 `POD_CIDR` 内 IP、跨节点通信成功、Calico Node Ready，且节点间 UDP 4789 可达。

**失败时看哪里**：Calico Node Log、IPPool、VXLAN Interface、SG/firewalld UDP 4789、Route、MTU、NetworkPolicy 和 Node Sysctl。

## 26. 验收 Service 网络

**目标**：证明 Client Pod 可以通过 ClusterIP 和 EndpointSlice 访问 Backend Pod。

**执行节点**：`[运维终端]`。

**最小 YAML**：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: service-net-server
spec:
  replicas: 2
  selector:
    matchLabels: {app: service-net-server}
  template:
    metadata:
      labels: {app: service-net-server}
    spec:
      nodeSelector: {node-role: app}
      containers:
        - name: server
          image: <TRUSTED_HTTP_IMAGE_WITH_VERSION_OR_DIGEST>
          ports:
            - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: service-net-server
spec:
  selector: {app: service-net-server}
  ports:
    - name: http
      port: 80
      targetPort: 8080
---
apiVersion: v1
kind: Pod
metadata:
  name: service-net-client
spec:
  nodeSelector: {node-role: app}
  containers:
    - name: client
      image: <TRUSTED_TOOLBOX_IMAGE_WITH_VERSION_OR_DIGEST>
      command: ["sh", "-c", "sleep 3600"]
  restartPolicy: Never
```

**命令**：

```bash
kubectl apply -f service-network-test.yaml
kubectl rollout status deployment/service-net-server --timeout=120s
kubectl wait --for=condition=Ready pod/service-net-client --timeout=120s
kubectl get svc service-net-server
kubectl get endpointslice -l kubernetes.io/service-name=service-net-server -o wide
kubectl exec service-net-client -- curl -fsS http://service-net-server/
kubectl delete -f service-network-test.yaml
```

**命令作用**：ClusterIP 提供虚拟服务地址；EndpointSlice 记录 Backend Pod；kube-proxy 实现 Service 转发。

**预期结果**：Service 有 ClusterIP，EndpointSlice 有两个 Ready Endpoint，Client 收到 Backend 响应。

**如何检查**：查看 Service、EndpointSlice、Backend Pod 与 Client 输出，确认请求确实经过 ClusterIP，而不是直接访问 Pod IP。

**失败时看哪里**：Service Selector、Target Port、EndpointSlice、kube-proxy Log、NetworkPolicy、Pod Readiness 和 CNI。

## 27. 验收 DNS

**目标**：验证 CoreDNS 能解析 Kubernetes 内置域名和测试 Service DNS。

**执行节点**：`[运维终端]`。

创建固定版本的 DNS Tool Pod：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dns-test
spec:
  nodeSelector: {node-role: app}
  containers:
    - name: dnsutils
      image: <TRUSTED_DNSUTILS_OR_BUSYBOX_IMAGE_WITH_VERSION_OR_DIGEST>
      command: ["sh", "-c", "sleep 3600"]
  restartPolicy: Never
```

**命令**：

```bash
kubectl apply -f dns-test.yaml
kubectl wait --for=condition=Ready pod/dns-test --timeout=120s
kubectl exec dns-test -- nslookup kubernetes.default.svc.cluster.local
kubectl exec dns-test -- cat /etc/resolv.conf

# 在第 26 节 Service 尚保留时测试；否则临时重新创建
kubectl exec dns-test -- nslookup service-net-server.default.svc.cluster.local

kubectl delete -f dns-test.yaml
```

**命令作用**：CoreDNS 监听 kube-dns Service，读取 Kubernetes Service/Endpoint 数据并回答 `cluster.local` 查询。

**预期结果**：内置 API Service 与测试 Service 均解析到对应 ClusterIP。

**如何检查**：对照 `nslookup`、`kubectl get svc` 和 Pod `/etc/resolv.conf`，确认查询经过 kube-dns Service。

**失败时看哪里**：CoreDNS Pod/Log、kube-dns Service/Endpoint、Pod `/etc/resolv.conf`、Node Upstream DNS、`ndots` 和 NetworkPolicy。

## 28. 验收 NetworkPolicy

**目标**：证明 Calico 实际执行 Default Deny 与显式 Allow Policy。

**执行节点**：`[运维终端]`。

**测试流程**：使用专用 Namespace，先验证无 Policy 时可访问，再应用 Default Deny，最后只允许带 `access=allowed` 的 Client。

```yaml
apiVersion: v1
kind: Namespace
metadata: {name: netpol-test}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: server, namespace: netpol-test}
spec:
  replicas: 1
  selector: {matchLabels: {app: server}}
  template:
    metadata: {labels: {app: server}}
    spec:
      nodeSelector: {node-role: app}
      containers:
        - name: server
          image: <TRUSTED_HTTP_IMAGE_WITH_VERSION_OR_DIGEST>
          ports: [{containerPort: 8080}]
---
apiVersion: v1
kind: Service
metadata: {name: server, namespace: netpol-test}
spec:
  selector: {app: server}
  ports: [{port: 80, targetPort: 8080}]
---
apiVersion: v1
kind: Pod
metadata:
  name: client-denied
  namespace: netpol-test
spec:
  nodeSelector: {node-role: app}
  containers:
    - name: client
      image: <TRUSTED_TOOLBOX_IMAGE_WITH_VERSION_OR_DIGEST>
      command: ["sh", "-c", "sleep 3600"]
---
apiVersion: v1
kind: Pod
metadata:
  name: client-allowed
  namespace: netpol-test
  labels: {access: allowed}
spec:
  nodeSelector: {node-role: app}
  containers:
    - name: client
      image: <TRUSTED_TOOLBOX_IMAGE_WITH_VERSION_OR_DIGEST>
      command: ["sh", "-c", "sleep 3600"]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: default-deny-ingress, namespace: netpol-test}
spec:
  podSelector: {}
  policyTypes: [Ingress]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: allow-labeled-client, namespace: netpol-test}
spec:
  podSelector: {matchLabels: {app: server}}
  policyTypes: [Ingress]
  ingress:
    - from:
        - podSelector: {matchLabels: {access: allowed}}
      ports:
        - {protocol: TCP, port: 8080}
```

**命令**：

```bash
kubectl apply -f network-policy-test.yaml
kubectl wait -n netpol-test --for=condition=Available deployment/server --timeout=120s
kubectl wait -n netpol-test --for=condition=Ready pod/client-denied pod/client-allowed --timeout=120s

# 应超时或被拒绝
kubectl exec -n netpol-test client-denied -- curl --max-time 3 http://server/

# 应成功
kubectl exec -n netpol-test client-allowed -- curl -fsS --max-time 3 http://server/

kubectl delete namespace netpol-test
```

**预期结果**：未授权 Client 失败，带允许 Label 的 Client 成功；Namespace 删除完成。

**如何检查**：记录两次 Curl 的 Exit Code，并检查两条 NetworkPolicy 的 Selector 与目标 Pod Label；清理后确认 Namespace 不存在。

**失败时看哪里**：Policy Selector/Port、Calico Controller/Node、Endpoint Label、HTTP Container 实际 Port，以及是否存在更广泛的 Allow Policy。

## 29. 验收 Control Plane NLB HA

**目标**：证明 kubectl 通过统一 Endpoint 工作，并能容忍一个 NLB Backend 退出。

**执行节点**：`[运维终端]` 执行 kubectl；阿里云控制台操作 NLB。一次只操作一个 Control Plane Backend。

**步骤与命令**：

```bash
kubectl config view --minify
kubectl get nodes
kubectl get --raw='/readyz?verbose'
```

1. 确认 Kubeconfig Server 是 `CONTROL_PLANE_ENDPOINT`。
2. 在 NLB Server Group 中临时禁用或把 `master-01` Backend 权重设为 0；不要停止两个 Master。
3. 等待 NLB 状态稳定，再执行：

```bash
for i in $(seq 1 10); do
  date
  kubectl get nodes --request-timeout=5s
  sleep 2
done
```

4. 命令应持续成功。
5. 恢复 `master-01` Backend，确认 Health Check 重新 Healthy。
6. 可在独立窗口分别对 `master-02`、`master-03` 重复，但始终只操作一个。

**命令作用**：验证客户端不依赖单一 Master IP，NLB 能把连接转发到其余健康 API Server。

**预期结果**：单 Backend 禁用期间 Cluster API 继续可用；恢复后 3 个 Backend Healthy。

**如何检查**：保存连续 10 次 kubectl 结果、NLB Backend 状态变化和最终 3 Backend Healthy 的控制台证据。

**失败时看哪里**：Kubeconfig Server、NLB Health/Algorithm、API Server Readyz、SG/firewalld、Certificate SAN、etcd Quorum 和客户端 DNS Cache。

## 30. 最终验收

**目标**：在不部署 CloudSentinel 的前提下，冻结 Kubernetes 基础设施验收证据。

**执行节点**：`[运维终端]`；etcd 深度检查在 `[仅 Control Plane]`。

**如何检查**：执行以下集群命令并保存不含 Secret 的输出。

```bash
kubectl cluster-info
kubectl get nodes -o wide
kubectl get nodes --show-labels
kubectl get pods -A -o wide
kubectl get svc -A
kubectl get daemonsets -A
kubectl get deployments -A
kubectl get endpointslices -A
kubectl get --raw='/readyz?verbose'
kubectl get events -A --sort-by=.lastTimestamp
```

**Control Plane/etcd 检查**：

```bash
kubectl get pods -n kube-system -l component=etcd -o wide
kubectl get pods -n kube-system -l component=kube-apiserver -o wide
kubectl get --raw='/readyz?verbose' | grep -E 'etcd|ok'
```

可选 `etcdctl` 检查必须在受控 Control Plane 上使用 kubeadm Certificate，且不能复制 Key：

```bash
sudo ETCDCTL_API=3 etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt \
  --key=/etc/kubernetes/pki/etcd/healthcheck-client.key \
  endpoint health

sudo ETCDCTL_API=3 etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt \
  --key=/etc/kubernetes/pki/etcd/healthcheck-client.key \
  member list
```

`etcdctl` Binary 可能不在 Host PATH；应使用与 kubeadm etcd 镜像相同版本的受控工具方案。本文不提供任何 Member Delete 命令。

**版本与 Runtime 检查**：在所有 Node 记录：

```bash
kubeadm version -o short
kubelet --version
kubectl version --client=true
containerd --version
rpm -q kubeadm kubelet kubectl containerd.io
sudo dnf versionlock list
```

**预期结果**：以下验收标准全部满足。

- 7 个 Node 均 Ready；3 个 Control Plane 与 4 个 Worker Role 正确。
- 3 个 etcd Static Pod 与 API Server 正常，Readyz 全部通过。
- Calico/Tigera、CoreDNS、kube-proxy 均健康。
- Pod 跨节点、ClusterIP、Service DNS 和 NetworkPolicy 测试通过且测试资源已清理。
- App/Monitoring/Data 调度与 Taint 行为符合设计。
- NLB 单 Backend 故障测试通过并已恢复。
- 所有组件版本固定，Version Lock 生效。
- 没有部署任何 CloudSentinel Workload。

**失败时看哪里**：根据失败项跳转到 `troubleshooting.md`，保留命令输出、Event 和不含 Secret 的 Log。任何失败都阻止声明集群可用于下一阶段。

## Alibaba Cloud Linux 4 执行边界

本文的架构与 kubeadm 流程以 Alibaba Cloud Linux 4 为当前唯一基线。由于操作系统刚完成切换，以下项目必须在真实节点上重新验证后才能冻结命令：

- DNF Repository、Package Name、Exclude 和 Version Lock 语法。
- Kubernetes 与 containerd 的实际 RPM 来源和完整版本。
- Calico 对当前 6.6 内核、nftables、NetworkManager 和 SELinux 的支持。
- `kubeadm` Config API Version 与所选 Kubernetes Minor Version。
- 所有 Package Name、Systemd Unit 和 Config Path。

不得沿用旧 CentOS Stream 9 命令强装 EL9 RPM，也不得把单节点成功直接等同于八节点验收完成。

## 官方参考

- [安装 kubeadm](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/)
- [使用 kubeadm 创建集群](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/)
- [创建 kubeadm HA 集群](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/high-availability/)
- [kubeadm v1beta4 Config API](https://kubernetes.io/docs/reference/config-api/kubeadm-config.v1beta4)
- [配置 Cgroup Driver](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/configure-cgroup-driver/)
- [containerd Getting Started](https://github.com/containerd/containerd/blob/main/docs/getting-started.md)
- [Calico 官方文档入口](https://docs.tigera.io/calico/)
- [Calico GitHub Release](https://github.com/projectcalico/calico/releases)
