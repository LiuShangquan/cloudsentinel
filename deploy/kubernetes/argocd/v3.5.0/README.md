# Argo CD v3.5.0 学生集群离线安装基线

本目录记录 CloudSentinel 学生集群使用的 Argo CD 固定版本和安装边界。Argo CD `v3.5.0` 的官方测试矩阵覆盖 Kubernetes `v1.35`。当前使用官方 `manifests/install.yaml` 非 HA 拓扑，适合练习与演示，不得描述为企业生产高可用方案。正式生产应改用官方 HA 清单并重新评估资源、Redis、入口、SSO、审计和灾难恢复。

## 固定输入

| 项目 | 固定值 |
|---|---|
| Argo CD | `v3.5.0` |
| Dex | `v2.45.0` |
| Argo CD Redis 缓存 | `8.2.3-alpine` |
| 官方非 HA 清单 SHA256 | `a32bf36a437071a1f563ebf9e81c8a39fba9057c17db7d5d041afb7b6e3f4afe` |
| 节点架构 | `linux/amd64` |
| 调度边界 | `node-role=app` |
| 服务暴露 | `ClusterIP`，不创建 Ingress/公网 LoadBalancer |

这里的 Redis 是 Argo CD 控制面的临时缓存，不是 CloudSentinel 业务 Redis，不保存业务数据。学生集群保留单副本；它故障时会影响 GitOps 控制面，但不会把已经运行的 CloudSentinel Pod 自动删除。

## 操作顺序

1. 在 ACR `cloudsentinel0306` 命名空间手工创建三个私有仓库，并保持“自动创建仓库”关闭：
   - `cloudsentinel-argocd`
   - `cloudsentinel-argocd-dex`
   - `cloudsentinel-argocd-redis`
2. 合并源仓库变更后，从 `main` 手工运行 `mirror-argocd-images`。
3. 确认工作流绿色，下载 `argocd-v3.5.0-linux-amd64` Artifact 并解压。
4. 将解压后的整个目录上传为 `master-01:/root/argocd-v3.5.0`，不要改内部文件名。
5. 将源仓库中的 `deploy/kubernetes/install-argocd.sh` 上传为 `master-01:/root/install-argocd.sh`。
6. 在 `master-01` 核对 context、文件清单和校验值后，手工执行：

```bash
export KUBECONFIG=/etc/kubernetes/admin.conf
kubectl config current-context
kubectl get nodes -o wide
kubectl get clustersecretstore cloudsentinel-secret-store

chmod 0700 /root/install-argocd.sh
chmod 0700 /root/argocd-v3.5.0
chmod 0600 /root/argocd-v3.5.0/*

bash /root/install-argocd.sh \
  --bundle-dir /root/argocd-v3.5.0 \
  --apply
```

脚本首先通过 External Secrets 将受控源 Secret `cloudsentinel-registry` 物化为 `argocd/platform-acr-registry`，然后执行 Server-Side Dry Run 和正式安装。脚本不创建 GitHub 凭据、不连接 GitOps 仓库、不应用 `bootstrap/argocd`，也不部署 CloudSentinel 工作负载。

## 安装后只读验收

```bash
kubectl -n argocd get deployment,statefulset,pod -o wide
kubectl -n argocd get externalsecret platform-acr-registry
kubectl -n argocd get service argocd-server
kubectl get crd applications.argoproj.io applicationsets.argoproj.io appprojects.argoproj.io
```

预期为 6 个 Deployment、1 个 StatefulSet 全部 Ready，Pod 只位于两个 `node-role=app` 节点，所有运行时镜像均为北京 ACR VPC digest，`argocd-server` 保持 `ClusterIP`。

## 学生集群并发边界

本集群的三个 Control Plane 均只有 2 GiB 内存。离线 Bundle 因此通过 `argocd-cmd-params-cm` 固定 Application Controller 状态处理器、操作处理器、`kubectl` 并发和 Repo Server 清单生成并发为 `1`。该配置不关闭自动同步、Self-Heal 或 Prune，只把多个应用的比较与同步串行化，避免瞬时 API 请求和清单生成压垮实验控制面。

这些值是学生集群容量保护，不是企业生产吞吐建议。扩大应用数量或升级控制面容量后，必须结合 API Server、etcd、Repo Server 和 Application Controller 指标重新压测，不能直接沿用或盲目提高并发。

只读检查：

```bash
kubectl -n argocd get configmap argocd-cmd-params-cm \
  -o jsonpath='{.data.controller\.status\.processors}{" "}{.data.controller\.operation\.processors}{" "}{.data.controller\.kubectl\.parallelism\.limit}{" "}{.data.reposerver\.parallelism\.limit}{"\n"}'
```

预期输出为 `1 1 1 1`。修改 ConfigMap 后必须重启 `argocd-repo-server` 和 `argocd-application-controller` 才能使对应进程读取新值。

安装完成后再单独配置 `cloudsentinel-gitops` 的只读 Deploy Key。初始管理员密码位于 `argocd-initial-admin-secret`，仅应在受控终端按需读取；首次登录并修改密码后删除该初始 Secret。不得把密码、Deploy Key、仓库 Secret、Kubeconfig 或其 Base64 内容复制到 Git、聊天记录或工单。
