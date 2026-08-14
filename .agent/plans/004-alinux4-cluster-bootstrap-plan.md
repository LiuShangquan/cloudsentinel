# Alibaba Cloud Linux 4 集群引导计划

## 目标

在不改变既定 3 Control Plane、4 Worker、1 个集群外运维节点拓扑的前提下，将 Kubernetes 基础设施基线从已放弃的 CentOS 7/Stream 9 路线切换为 Alibaba Cloud Linux 4 LTS 64 位普通版，并以真实节点证据冻结 containerd、Kubernetes 和 Calico 版本后再开始安装。

## 已确认事实

- 操作者于 2026-08-13 报告八台 ECS 已完成 Alibaba Cloud Linux 4 LTS 64 位普通版更换。
- `master-01` 已通过命令确认 Alibaba Cloud Linux 4、内核 `6.6.102-6.alnx4.x86_64`、`x86_64` 和 `cgroup2fs`。
- 两段私网位于同一 VPC，并由操作者确认双向连通；Inventory 中保留固定私网 IP。
- 当前没有 Kubernetes 或业务部署结果需要迁移，CentOS 7 ELRepo 内核升级路线取消。

## 阶段

1. 在八台节点执行只读预检，核对主机名、操作系统、内核、cgroup v2、容量、模块、时间、私网 SSH 和 ACR VPC 端点。
2. 在 Alibaba Cloud Linux 4 真实仓库中枚举并冻结 containerd、kubeadm、kubelet、kubectl 与相关依赖的完整版本；不得强装 EL9 RPM。
3. 冻结 VPC/Pod/Service CIDR、Internal NLB Endpoint、安全组和 Calico 版本。
4. 分批完成系统初始化与 containerd 安装，并保留逐节点验收证据。
5. 依次初始化 `master-01`、加入其余 Control Plane、安装 CNI、加入 Worker，并完成 HA/网络/调度验收。
6. 集群基础设施验收完成后，才进入 Argo CD、Secret 控制面和 CloudSentinel GitOps 同步。

## 当前状态

- 文档与 Inventory 的操作系统事实已切换为 Alibaba Cloud Linux 4。
- 八台节点均已通过只读预检，操作系统基础验收完成。
- 已验证节点均通过 5.10 以上内核门槛、八节点私网 TCP 22 连通性和 ACR VPC HTTPS 可达性检查；ACR 未认证请求返回预期的 HTTP 401。
- 已确认 VPC `vpc-2zeer06chhjz9pjq3qm3n` 为 `172.16.0.0/12`；应用节点交换机为 `vsw-2zej6zg9kscxw7s021q9n`（`172.16.32.0/20`），其余节点交换机为 `vsw-2zeasvhv2dikiwzuvel0g`（`172.29.240.0/20`）。
- 已确认上述交换机分别位于北京可用区 K 和 G；空闲交换机 `vsw-2zeq56omjune5xz62fxz8`（`172.18.144.0/20`）位于北京可用区 H。三者绑定默认系统路由表 `vtb-2zeiq8wdyy5irqr1kstph`。
- Pod/Service CIDR 采用 `10.244.0.0/16` 与 `10.96.0.0/12`，二者相互不重叠且不与 VPC CIDR 重叠；仍需确认 VPC 自定义路由、VPN/CEN 和管理网络没有使用这些地址段。
- 默认路由表已于 2026-08-13 完成截图核验，仅包含三个 VSwitch 系统路由和 `100.64.0.0/10` 系统网段；未发现与 Pod/Service CIDR 冲突的自定义、VPN 或 CEN 路由。
- 私网 NLB `nlb-axdxf21jombh758m09` 已创建，稳定入口冻结为 `nlb-axdxf21jombh758m09.cn-beijing.nlb.aliyuncsslb.com:6443`；G/K 两区 VIP 分别为 `172.29.253.156` 与 `172.16.41.184`。TCP 6443 监听已运行，三台 Master 已加入后端服务器组；API Server 尚未初始化时健康检查异常属于预期状态。
- 已冻结 Kubernetes `v1.35.7`、containerd `1.7.34` 与 Calico `v3.32.1`。7 个 Kubernetes 节点均已完成 Runtime/Kubernetes RPM 基线验收，containerd 使用 `SystemdCgroup = true` 且 CRI v1 可用；`ops-storage` 不加入 Kubernetes。
- `registry.k8s.io` 从北京节点访问超时；固定 kubeadm 镜像已同步到 ACR 个人版，并于 2026-08-13 在全部 7 个 Kubernetes 节点通过 ACR VPC 端点完成预拉和 containerd Pause 镜像对齐。
- 固定 Calico `v3.32.1` Linux/amd64 镜像已由 `mirror-calico-images` 工作流第 1 次运行同步到 ACR 个人版，并于 2026-08-13 在全部 7 个 Kubernetes 节点通过 VPC 端点完成预拉验收。
- 公网全 TCP、HTTP 80 与 RDP 3389 规则已移除。项目所有者决定在学生实验期保留 `0.0.0.0/0` 的 ICMP 与 TCP 22，并暂时使用共享安全组组内互通承载 Kubernetes/Calico 私网通信；公网 SSH 暴露和未按角色拆分安全组均作为明确接受的实验风险记录。
- 全部 8 台节点已于 2026-08-13 完成 `sshd -T` 有效配置核验：密码认证与键盘交互认证关闭，公钥认证启用；当前 `PermitRootLogin yes` 在上述组合下仍为仅密钥登录，后续收紧为 `prohibit-password` 作为实验环境延后加固项记录。
- 已生成不含 Bootstrap Token、Certificate Key 或其他 Secret 的 `deploy/kubernetes/kubeadm-init-master-01.yaml`，并已完成配置校验、初始化预检和真实初始化。
- `master-01` 已验证初始化配置 SHA256 一致、权限为 `0600`，且 `kubeadm config validate` 通过；控制平面端口空闲，历史 PKI、Manifest 与 etcd 状态均不存在。首次 Preflight 没有创建集群，但因操作系统可见内存仅 `1671 MB`、低于 kubeadm 的 `1700 MB` 硬门槛而终止，同时发现 `master-01` 无法由当前 Resolver 解析。必须先恢复主机名解析并解决内存容量，不得直接忽略 `Mem` 门禁。
- 已确认首次内存失败由 kdump/crashkernel 预留 `192 MB` 引起；实验环境释放该预留后，第二次 `kubeadm init phase preflight` 已通过，日志为 `/root/kubeadm-preflight-master-01-second.log`。containerd 1.7 RuntimeConfig 兼容性提示仍为非致命警告。
- `master-01` 私网主机名解析已补齐，最终 Preflight 不再出现 Hostname 警告并以 `preflight_rc=0` 通过，日志为 `/root/kubeadm-preflight-master-01-final.log`；该结果曾作为后续真实初始化的放行证据。
- `master-01` 已于 2026-08-14 成功完成 `kubeadm init --upload-certs`，日志为 `/root/kubeadm-init-master-01-20260814T113659.log`。API `/livez` 与 `/readyz` 均返回 `ok`，静态控制面组件正常；CNI 安装前 Node `NotReady` 与 CoreDNS `Pending` 属于预期状态。
- 初始化输出中暴露的 Bootstrap Token 已失效，控制面证书已使用新 Key 重新上传，并在 `/root/cloudsentinel-current-credential-dir` 指向的权限 `0600` 目录中生成新的短期 Join 文件。旧凭据不得再次使用，新的 Join 文件也不得复制到 Git、聊天或工单。
- Calico `v3.32.1` 与 `APIServer/default` 已安装；`apiserver`、`calico`、`ippools`、`tiers` 四个 TigeraStatus 均为 `Available=True`、`Degraded=False`，两副本 Calico API Server 与 CoreDNS 均健康。
- 2026-08-14 已完成 `master-02`、`master-03` 和 4 个 Worker 的逐台 Join：7 个 Kubernetes 节点均为 Ready，stacked etcd 的 3 个 Member 均为 `started`。`master-03` 的 Join 在 TLS Bootstrap 等待阶段被操作者中断，但 kubelet、控制面静态 Pod、etcd 与 Calico 已完成收敛；随后补齐了 `control-plane` Label 与默认 `NoSchedule` Taint。
- `worker-app-01/02` 已设置 `node-role=app`；`worker-monitor` 已设置 `node-role=monitoring` 与 `dedicated=monitoring:NoSchedule`；`worker-data-01` 已设置 `node-role=data` 与 `dedicated=data:NoSchedule`。下一门禁是集群级 HA、跨节点 Pod、Service、DNS、NetworkPolicy、资源压力与 NLB 后端健康验收；这些检查通过前不进入 Argo CD 和业务 GitOps 同步。
