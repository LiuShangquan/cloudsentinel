# CloudSentinel 学生集群 StatefulSet 数据层计划

## 决策基线

- 保留 3 Control Plane、4 Worker 与 1 个集群外运维节点的既有拓扑和全部业务功能。
- 学生练习集群不购买 RDS/Tair；MySQL 8.4 与 Redis 8 以单副本 StatefulSet 运行在 `cloudsentinel-data` Namespace。
- 两个 StatefulSet 通过 `nodeSelector` 与 `dedicated=data:NoSchedule` Taint 固定在 `worker-data-01`。
- 数据使用静态 Local PersistentVolume，`persistentVolumeReclaimPolicy: Retain`；删除 Pod、PVC 或 Argo Application 不应自动删除宿主机数据。
- Staging 与 Production 共用一个 MySQL/Redis 进程，但使用独立 MySQL Database、独立账户和不同 Redis Logical DB。
- 应用 Migration 继续由 Argo CD PreSync Job 执行；不得使用 `AutoMigrate`。
- Secret 不进入 Git。学生集群使用 External Secrets Operator Kubernetes Provider，从受控的 `cloudsentinel-secret-source` Namespace 同步 Secret。
- MySQL/Redis Service 仅为 ClusterIP，NetworkPolicy 只允许 CloudSentinel Staging/Production Namespace 访问。
- Lab 内部数据链路暂不启用 TLS，风险由 VPC、Security Group、ClusterIP 和 NetworkPolicy 收敛；该例外不得解释为生产 TLS 基线。
- 原 RDS/Tair Production 模板和企业边界保留；真实 GitOps Application 改为读取 `lab-staging` 与 `lab-production` Overlay。

## Phase L0 - 清单与持久化

- 新建 `platform/cloudsentinel-data/overlays/lab`。
- 创建 MySQL、Redis、Backup 三个 Local PV/PVC。
- 创建单副本 MySQL/Redis StatefulSet、Headless Service、配置与 NetworkPolicy。
- 创建每日 MySQL/Redis Backup CronJob，备份写入保留型本地卷并由 `ops-storage` 定期拉取。

## Phase L1 - Secret 与环境隔离

- 创建 Kubernetes Provider `ClusterSecretStore`、最小 RBAC 和 Secret Source Namespace。
- 数据层同步数据库 Root、环境账户与 Redis 密码。
- Staging/Production ExternalSecret 分别生成运行时连接契约与 Migration URL。
- Staging 使用 Redis DB 0，Production 使用 Redis DB 1；MySQL 使用独立 Database 与账户。

## Phase L2 - GitOps 编排

- 新建 Data Argo CD Application，启用 Self-Heal、不启用自动 Prune。
- Secret ApplicationSet 增加 Data 环境。
- 应用 ApplicationSet 改为读取 `lab-staging`/`lab-production`，Production 继续人工 Sync。
- AppProject 增加 `cloudsentinel-data` 目标和 StatefulSet/PVC/PV/StorageClass/CronJob 最小权限。

## Phase L3 - 供应链与文档

- 新增手工工作流，将固定版本 MySQL/Redis 镜像镜像到北京 ACR 个人版。
- 更新架构、GitOps、数据库、Kubernetes、运维、恢复和路线图文档。
- 明确单节点故障、Local PV 绑定、无 TLS Lab 例外及升级/恢复操作。

## Phase L4 - 验证

- Kustomize 渲染 Data、Lab Staging、Lab Production 与 Argo CD Bootstrap。
- 验证 Git 中无 Secret，数据服务无 NodePort/LoadBalancer，PV 为 Retain 且绑定 `worker-data-01`。
- 执行 Go 格式化、Vet、测试和构建，确保配置修改未改变应用行为。
- 真实 Kubernetes、ACR 镜像镜像、PVC 绑定、Migration、备份与恢复结果在服务器执行前标记 `NOT VERIFIED`。

## 实现状态

- 已新增数据 StatefulSet、静态存储、初始化、网络策略、逻辑备份、实验应用/Secret Overlay 与 Argo CD 编排。
- 已新增 Kubernetes Provider Secret Store、数据镜像同步工作流和 digest 更新脚本测试。
- 已补充中文部署、风险、备份恢复和迁移出口文档。
- 本地验证已完成：`gofmt -l cmd internal` 无输出，`go vet ./...`、`go test ./...`、`go build ./cmd/api ./cmd/worker` 通过；4 个 GitOps 镜像脚本测试通过；GitOps 种子与独立工作目录中的 13 个 Kustomize 根目录全部渲染成功。
- 结构断言已完成：实验数据层恰好包含 2 个 StatefulSet、3 个 Retain PV、3 个固定绑定 PVC、5 个 `worker-data-01` 工作负载和 6 个 digest 镜像引用；当前应用 Overlay 无 Ingress/StatefulSet，数据与应用 Service 无 NodePort/LoadBalancer；启用路径无 `REPLACE_*`，敏感信息模式扫描无命中，`git diff --check` 通过。
- `D:\cloudsentinel-gitops` 已采用非删除式复制与种子逐文件同步，未执行 Git Add、Commit 或 Push。
- 真实 Kubernetes、ACR 镜像同步、ExternalSecret、PVC 绑定、Migration、备份与恢复仍为 `NOT VERIFIED`，直到操作者提供服务器执行证据。
