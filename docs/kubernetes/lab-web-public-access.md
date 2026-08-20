# 学生集群 Web 控制台公网访问

## 边界

`cloudsentinel-staging` 的业务门户默认保持为 ClusterIP。当前学生练习集群可
额外使用固定 `30443/TCP` NodePort，通过一台应用 Worker 的公网 IP 提供
HTTPS 登录。该例外不使用域名、公网负载均衡或 Ingress，也不代表生产方案。

API、MySQL、Redis、Argo CD、Prometheus 和 Alertmanager 不得因此直接暴露
到公网。浏览器对 `/api/` 的请求仍由 Web 容器通过集群内部 Service 转发。

## 发布顺序

1. 将本次源码改动通过 PR 合并到 `cloudsentinel`；
2. 将 `deploy/gitops-repository/` 对应变更同步到独立
   `cloudsentinel-gitops` 仓库并通过 PR 合并；
3. 使用 GitOps 仓库中的 `bootstrap/secret-store/rbac.yaml` 运行
   `kubectl auth reconcile`，使源 Secret Reader 仅新增
   `cloudsentinel-web-public-tls` 的读取权限；
4. 在控制面节点运行 `bootstrap-web-public-tls.sh`；
5. 等待 Argo CD 将 Web Application 同步为 `Synced/Healthy`；
6. 只给选定应用 Worker 的安全组开放 TCP 30443；
7. 访问 `https://<PUBLIC_IP>:30443/`。

## 证书引导

把 `deploy/kubernetes/bootstrap-web-public-tls.sh` 上传到健康控制面节点，
确认所选应用 Worker 当前公网 IPv4 后执行：

```bash
export KUBECONFIG=/etc/kubernetes/admin.conf

CLOUDSENTINEL_WEB_PUBLIC_IP='<APPLICATION_WORKER_PUBLIC_IPV4>' \
KUBERNETES_API_SERVER='https://172.29.253.155:6443' \
bash /root/bootstrap-web-public-tls.sh
```

脚本生成包含公网 IP SAN 的 30 天自签名证书，只把证书、私钥与公网 URL
写入 `cloudsentinel-secret-source/cloudsentinel-web-public-tls`。脚本不会打印
私钥。公网 IP 改变或证书到期时，必须用新 IP 重新运行脚本。

## 阿里云安全组

只在准备作为入口的应用 Worker 上增加：

| 授权 | 协议 | 端口 | 来源 |
|---|---|---:|---|
| 允许 | TCP | 30443 | 当前管理公网 IP `/32` |

客户端公网 IP 无法固定时，可在演示期间临时使用 `0.0.0.0/0`，但必须使用
独立强密码，并在演示完成后立即删除规则。不得开放 `30000-32767` 整段。

## 验收

```bash
kubectl -n cloudsentinel-staging get \
  externalsecret/cloudsentinel-web-public-tls \
  service/cloudsentinel-web-public \
  deployment/cloudsentinel-web

kubectl -n cloudsentinel-staging get pods \
  -l app.kubernetes.io/component=web \
  -o wide
```

预期 ExternalSecret 为 `Ready=True`，Deployment 为 `2/2`，每个 Pod 中
`web` 和 `public-tls-proxy` 两个容器均 Ready。浏览器首次访问会提示自签名
证书不受信任，继续前应核对脚本输出的 SHA256 指纹和证书 IP。

## 回滚

先删除安全组 30443 入站规则，再通过 GitOps PR 移除 `web-public.yaml`、
`web-public-deployment-patch.yaml` 以及 Kustomization 中的引用。删除公网入口
不影响内部 Web、API 或业务数据。
