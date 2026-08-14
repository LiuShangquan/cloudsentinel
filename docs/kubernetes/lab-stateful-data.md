# 学生集群 StatefulSet 数据层

本文只适用于当前 8 台 ECS 的学生练习环境。MySQL 与 Redis 均为单副本，固定到 `worker-data-01`；节点、系统盘或本地目录故障会导致整体数据层不可用。静态 PV 的 `Retain` 只能防止声明式误删直接清空目录，不能替代异机备份或高可用。

当前 `worker-data-01` 为 2 vCPU / 2 GiB。Lab 清单因此将 MySQL Buffer Pool 收紧到 256 MiB、MySQL 容器上限收紧到 896 MiB，Redis `maxmemory` 收紧到 96 MiB；这些参数只服务于低并发演示。部署后必须观察 `kubectl top node/pod`、Node `MemoryPressure`、容器 Restart/OOMKilled 和 MySQL 连接数，资源不足时应升级 ECS，而不是继续压低数据库安全余量。

所有命令都必须由操作者先确认 kubeconfig Context、节点名称、路径和 Secret 值，再在标注的机器上手工执行。本仓库不会连接集群或自动执行部署。

## 1. 交付结构

| 组件 | 设计 |
|---|---|
| MySQL | 8.4.10、单副本 StatefulSet、8 GiB 静态 Local PV |
| Redis | 8.4.5 Alpine、单副本 StatefulSet、2 GiB 静态 Local PV、AOF everysec |
| 备份 | 每日 MySQL SQL Dump 与 Redis RDB，5 GiB 同节点 Local PV，保留 7 天 |
| 隔离 | `cloudsentinel-data` Namespace、Calico NetworkPolicy、数据节点 Label/Taint |
| 环境 | 两个 MySQL Database/账户；Staging 使用 Redis DB 0，Production 使用 DB 1 |
| Secret | External Secrets Kubernetes Provider；Git 中没有明文 |
| 入口 | `lab-staging`、`lab-production` 不含 Ingress，暂时不要求域名 |

MySQL、Redis 与同盘备份共享一个故障域。重要演示数据应定期复制到 `ops-storage` 或操作者电脑；正式业务不得使用此布局。

## 2. 部署前检查

在受控运维终端执行只读检查：

```bash
kubectl config current-context
kubectl get nodes -o wide
kubectl get node worker-data-01 -L node-role
kubectl describe node worker-data-01 | sed -n '/Taints:/,/Unschedulable:/p'
```

必须确认：

- 节点真实名称恰好是 `worker-data-01`；
- Label 为 `node-role=data`；
- Taint 为 `dedicated=data:NoSchedule`；
- 集群已安装 Argo CD/ApplicationSet、External Secrets Operator v1 API、Metrics Server 与支持 NetworkPolicy 的 Calico；当前 ESO 控制器固定为与 Kubernetes `1.35` 对齐的 `v2.8.0`；
- ACR 中已手工创建业务、数据和平台所需私有仓库；数据层至少包括 `cloudsentinel-mysql` 与 `cloudsentinel-redis`，ESO 安装还需要 `cloudsentinel-external-secrets`。

若 Label/Taint 尚未设置，应回到 `deployment-guide.md` 的节点隔离步骤，核对后手工执行，不要修改 GitOps 清单绕过隔离。

## 3. 准备本地目录

先确认 `worker-data-01` 的目标磁盘容量和挂载点。当前清单固定使用下列路径；如果它们位于容量不足的根盘，先挂载数据盘并修改清单走 PR，不要用符号链接临时绕过。

在 `worker-data-01` 手工执行：

```bash
sudo install -d -o 999 -g 999 -m 0700 /var/lib/cloudsentinel/mysql
sudo install -d -o 999 -g 999 -m 0700 /var/lib/cloudsentinel/redis
sudo install -d -o 999 -g 999 -m 0700 /var/lib/cloudsentinel/backups
sudo findmnt --target /var/lib/cloudsentinel/mysql
sudo df -h /var/lib/cloudsentinel/mysql /var/lib/cloudsentinel/redis /var/lib/cloudsentinel/backups
sudo stat -c '%U:%G %a %n' /var/lib/cloudsentinel/mysql /var/lib/cloudsentinel/redis /var/lib/cloudsentinel/backups
```

预期 UID/GID 为 `999:999`、权限为 `700`。若 SELinux 拒绝容器访问，不要关闭 SELinux；先查看 AVC，再为专用目录建立经过确认的持久化文件上下文。

2026-08-14 的真实集群证据已确认：`worker-data-01` 根文件系统为 40 GiB ext4、准备时可用约 32 GiB，容量门禁通过；三个目录已创建并返回 `LOCAL_PV_DIRECTORY_PREPARE=PASS`。这只证明宿主机存储准备完成，不代表 PV/PVC、StatefulSet 或备份已经部署。

## 4. 创建源 Secret

创建源 Secret 前必须先完成 ESO 平台控制器安装：

1. 在 ACR 手工创建私有仓库 `cloudsentinel-external-secrets`；
2. 从源码仓库 `main` 手工运行 `mirror-external-secrets-image`；
3. 下载 `external-secrets-v2.8.0-linux-amd64` Artifact，核对工作流成功状态和 Artifact 校验文件；
4. 在 `external-secrets` Namespace 创建 `platform-acr-registry` Pull Secret；
5. 把 Artifact 与 `deploy/kubernetes/install-external-secrets.sh` 上传到 `master-01`，确认路径后手工执行安装；
6. 仅当三个 ESO Deployment 可用且 `external-secrets.io/v1` CRD 存在后，才应用 `bootstrap/secret-store`。

ESO 安装脚本不会创建下面的业务源 Secret；两类权限边界不得合并。

不要把密码写入 Git、聊天、工单或 Shell 历史。推荐在 `ops-storage` 上创建权限为 `600` 的临时 env 文件，通过交互式编辑器填值。所有数据层密码必须是 32–128 位 `[A-Za-z0-9_-]`；JWT、Webhook Token 与两个环境密码必须彼此不同。

数据凭证文件需要以下键：

```text
MYSQL_ROOT_PASSWORD=
MYSQL_STAGING_PASSWORD=
MYSQL_STAGING_MIGRATION_PASSWORD=
MYSQL_PRODUCTION_PASSWORD=
MYSQL_PRODUCTION_MIGRATION_PASSWORD=
MYSQL_BACKUP_PASSWORD=
REDIS_PASSWORD=
```

Staging 与 Production 应用凭证文件分别需要：

```text
JWT_SECRET=
ALERTMANAGER_WEBHOOK_TOKEN=
BOOTSTRAP_USER_USERNAME=admin
BOOTSTRAP_USER_PASSWORD=
```

确认临时文件未放在任何 Git 仓库后，在受控运维终端创建源 Namespace 与 Secret：

```bash
kubectl apply -k bootstrap/secret-store
kubectl -n cloudsentinel-secret-source create secret generic cloudsentinel-data-credentials --from-env-file=/受控绝对路径/data.env
kubectl -n cloudsentinel-secret-source create secret generic cloudsentinel-staging-app --from-env-file=/受控绝对路径/staging.env
kubectl -n cloudsentinel-secret-source create secret generic cloudsentinel-production-app --from-env-file=/受控绝对路径/production.env
kubectl -n cloudsentinel-secret-source create secret docker-registry cloudsentinel-registry \
  --docker-server=crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com \
  --docker-username='<ACR 用户名>' \
  --docker-password='<在受控终端交互准备，避免进入历史>'
```

最后一条命令在某些 Shell 中会把参数留进历史。更安全的做法是临时关闭该终端的历史记录，或使用权限为 `600` 的 Docker config 文件通过 `--from-file=.dockerconfigjson=...` 创建。创建后只检查键名，不输出 Secret 值：

```bash
kubectl -n cloudsentinel-secret-source get secret
kubectl get clustersecretstore cloudsentinel-secret-store
```

临时文件应在确认 Secret 已创建后，从受控机器安全移除；不要提供通配符或未解析变量给删除命令。

## 5. 同步数据镜像

源码仓库 GitHub Actions 的 `mirror-lab-data-images` 会把固定的 `mysql:8.4.10` 与 `redis:8.4.5-alpine` 的 `linux/amd64` 镜像同步到 ACR 私有仓库，并直接采用 Registry Push 返回的 digest 创建 GitOps PR。当前 ECS 节点统一为 x86，因此不复制无用的 ARM 等多架构 Manifest；未来引入 ARM 节点前必须重新设计并验证多架构发布。运行前确认源码仓库已有：

- Repository Secrets：`ACR_USERNAME`、`ACR_PASSWORD`、`GITOPS_APP_PRIVATE_KEY`；
- Repository Variables：既有 GitOps App 变量，以及 `RELEASE_ALLOWED_OWNER`、`RELEASE_ALLOWED_REPOSITORY`；
- GitHub App 对 `cloudsentinel-gitops` 有 Contents/Pull Requests 写权限。

工作流成功后合并数据镜像 PR，并确认 GitOps 校验绿色。不要让 Kubernetes 节点直接从 Docker Hub 拉取。

## 6. 首次 Argo CD 启用顺序

严格按顺序进行，每一步健康后再继续：

1. 平台管理员应用 `bootstrap/secret-store`，确认 `ClusterSecretStore` Ready。
2. 将 GitOps 仓库以只读凭证接入 Argo CD。
3. 平台管理员应用 `bootstrap/argocd`。
4. 观察 `cloudsentinel-data-lab`：ExternalSecret 先物化，两个 StatefulSet 启动，MySQL Bootstrap Job 创建两个 Database 与最小权限账户。
5. 确认数据层健康后，观察 `cloudsentinel-secrets-staging` 与 `cloudsentinel-staging`；Staging 会自动同步。
6. Staging 验收通过后，才允许人工同步 Production。

只读验收命令：

```bash
kubectl get pv
kubectl -n cloudsentinel-data get externalsecret,secret,pvc,sts,pod,job,cronjob
kubectl -n cloudsentinel-data rollout status statefulset/mysql --timeout=10m
kubectl -n cloudsentinel-data rollout status statefulset/redis --timeout=5m
kubectl -n cloudsentinel-data get pod -o wide
kubectl -n cloudsentinel-staging get externalsecret,deploy,pod,job
kubectl -n cloudsentinel-production get externalsecret,deploy,pod,job
```

Secret 对象存在不代表可以输出其内容。日志中也不得搜索或复制包含连接串的环境变量。

## 7. 备份与恢复边界

CronJob 每天北京时间 02:15 生成 MySQL Dump，02:45 生成 Redis RDB，保留 7 天。首次部署后应手工从 CronJob 创建一次临时 Job 验证，但在执行前必须确认 Namespace、Job 名称与当前无同类任务运行。

只读检查：

```bash
kubectl -n cloudsentinel-data get cronjob,job
kubectl -n cloudsentinel-data get pvc data-backup
kubectl -n cloudsentinel-data logs job/<已确认的备份Job名称>
```

同节点备份无法覆盖节点或磁盘丢失。至少每日把 `/var/lib/cloudsentinel/backups` 的新文件复制到 `ops-storage`，并定期在隔离环境演练恢复。恢复 MySQL Dump、替换 Redis RDB、删除 PVC/PV、删除 Namespace 或删除 Argo Application 都是破坏性操作；本文不提供可直接执行的恢复/删除命令，必须先记录备份校验值、停写窗口、精确目标和回退路径。

## 8. 已接受风险与迁移出口

- 数据节点维护时，API/Worker Readiness 会失败，探测与事件写入中断。
- Redis AOF `everysec` 在异常断电时可能丢失约 1 秒数据；MySQL 与 Redis 不存在跨组件原子备份。
- Staging 与 Production 共享实例和资源上限，不是故障隔离环境。
- 集群内连接当前关闭 MySQL/Redis TLS，安全边界依赖 VPC、Calico NetworkPolicy 与 Namespace 隔离。
- Local PV 无法自动漂移，扩容或换节点必须执行受控迁移。

当项目开始承载真实用户或需要 SLA 时，迁移到 RDS/Tair 或高可用数据库平台，并把 Argo CD ApplicationSet 切回企业 Overlay；不要在单节点 StatefulSet 上叠加更多副本来假装高可用。
