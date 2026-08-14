# Calico v3.32.1 固定安装资源

本目录固定学生实验集群使用的 Calico `v3.32.1` Operator 安装资源。所有文件均不得包含凭据。

- `v1_crd_projectcalico_org.yaml`：从 `projectcalico/calico` 的 `v3.32.1` Tag 原样取得的官方 CRD 清单。
- `tigera-operator.upstream.yaml`：从同一 Tag 原样取得的官方 Operator 清单，用于来源审计，不直接安装。
- `tigera-operator-acr.yaml`：仅将官方 Operator 镜像替换为已同步的 ACR VPC 镜像，其他内容与上游清单一致。
- `installation.yaml`：CloudSentinel 实验集群配置；使用 `10.244.0.0/16`、VXLAN、关闭 BGP，并从 ACR VPC 扁平 `calico-*` 仓库拉取组件镜像。
- `apiserver.yaml`：启用 Calico API Server，使策略 Tier 控制器完成收敛；依赖 `calico-apiserver:v3.32.1` 和已经固定的 `calico-key-cert-provisioner:v3.32.1`。

安装前必须确认节点间私网 UDP 4789 可达、firewalld 已关闭、NetworkManager 不管理 Calico 接口，且 `KUBECONFIG=/etc/kubernetes/admin.conf`。首次安装顺序为 CRD、ACR Operator、`Installation`、`APIServer`；不得直接安装 `tigera-operator.upstream.yaml`。
