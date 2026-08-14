# External Secrets Operator v2.8.0 离线安装边界

本目录记录学生集群使用的 External Secrets Operator 版本与安装边界，不保存上游 1.9 MiB 发布清单、镜像层或任何凭据。

- 上游版本：`v2.8.0`
- 上游镜像：`ghcr.io/external-secrets/external-secrets:v2.8.0`
- 目标镜像：`crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com/cloudsentinel0306/cloudsentinel-external-secrets@sha256:<工作流返回值>`
- 官方清单：`https://github.com/external-secrets/external-secrets/releases/download/v2.8.0/external-secrets.yaml`
- 官方清单 SHA256：`7d1533a7ace1f0ee9a21ac2215665e8f5856690ca7b224961df2011ff735d572`
- 兼容边界：Kubernetes `1.35-1.36`；当前集群为 `v1.35.7`

`.github/workflows/mirror-external-secrets-image.yml` 会完成以下操作：

1. 只复制 `linux/amd64` 镜像到 ACR 个人版公网端点；
2. 从 Registry Push 输出取得不可变 digest；
3. 下载并校验官方发布清单；
4. 对上游清单中固定的 16 个 `namespace: default` 字段和 2 个 Namespace 参数做精确归一化，把控制器隔离到 `external-secrets`；替换数量不一致时工作流直接失败；
5. 生成使用 ACR VPC digest、`platform-acr-registry` Pull Secret、`node-role=app` 节点选择器和实验环境资源边界的 Kustomize 安装包；
6. 上传保留 14 天的 `external-secrets-v2.8.0-linux-amd64` Actions Artifact。

安装前，操作者必须在 ACR 中手工创建私有仓库 `cloudsentinel-external-secrets`，并在集群 `external-secrets` Namespace 创建类型为 `kubernetes.io/dockerconfigjson` 的 `platform-acr-registry` Secret。随后把 Artifact 解压目录上传到 `master-01`，再手工运行：

```bash
bash /root/install-external-secrets.sh \
  --bundle-dir /root/external-secrets-v2.8.0 \
  --apply
```

脚本不会创建或输出 Secret 值，也不会创建 ClusterSecretStore、Argo CD、PV、StatefulSet 或 CloudSentinel 工作负载。
