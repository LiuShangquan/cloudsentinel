# CloudSentinel Web 控制台

## 1. 产品定位

`web` 是 CloudSentinel 的业务管理入口，用于完成平台现有能力的日常操作。它不是 Grafana 的替代品：Web 控制台负责资产、探测任务、执行结果和故障事件；Grafana 负责时序指标分析和趋势看板。

前端采用 React、TypeScript、Vite 和 React Router。界面使用自有轻量设计系统，不依赖大型组件库；桌面端提供固定导航，窄屏自动切换为抽屉导航。所有文案、状态和日期均面向中文运维场景。

## 2. 页面与业务闭环

| 页面 | 路径 | 主要能力 |
|---|---|---|
| 登录 | `/login` | Bootstrap 用户或现有平台用户登录，获取 JWT |
| 运行总览 | `/` | 汇总资产、任务、近期结果、事件和依赖就绪状态 |
| 主机资产 | `/hosts` | 查询、新建、编辑、停用主机 |
| 服务管理 | `/services` | 查询、新建、编辑、停用 HTTP/TCP 服务 |
| 探测任务 | `/probe-tasks` | 配置执行周期、超时和重试，编辑或停用任务 |
| 执行结果 | `/probe-results` | 分页查看响应码、耗时和错误详情 |
| 故障事件 | `/incidents` | 查看事件详情，执行确认、处理和关闭动作 |
| 个人账户 | `/profile` | 查看当前用户、账户状态和会话安全说明 |

推荐操作顺序为：

```text
登录 -> 创建主机 -> 创建服务 -> 创建探测任务
     -> Worker 执行 HTTP/TCP 探测 -> 查看执行结果
     -> Prometheus/Alertmanager 产生告警 -> 在事件中心处置
```

前端不提供“直接创建 Incident”入口。Incident 仍只能由 Alertmanager Webhook 驱动，符合后端既有责任边界。

## 3. 认证与安全边界

- 登录调用 `POST /api/v1/auth/login`，后续请求使用 `Authorization: Bearer <JWT>`。
- JWT 只保存在 `sessionStorage`，关闭浏览器标签页后会话凭据自动清除。
- 遇到 `401` 时清理本地会话并返回登录页。
- 不在浏览器日志、页面或构建参数中写入密码、JWT、数据库凭证或 Registry 凭证。
- Nginx 设置 CSP、`X-Content-Type-Options`、`Referrer-Policy` 和 `Permissions-Policy`。
- 前端容器以非 Root 用户运行，使用只读根文件系统；Kubernetes Pod 不挂载 ServiceAccount Token。
- 当前后端只有统一登录身份，没有完整业务 RBAC，因此前端不会用隐藏按钮伪装服务端授权。后续若增加角色，必须先由 API 提供可验证的授权契约。

## 4. 本地开发

安装和检查：

```powershell
Set-Location D:\project\web
npm ci
npm run typecheck
npm test
npm run build
npm run dev
```

开发服务器默认监听 `http://127.0.0.1:5173`，并把 `/api`、`/healthz` 和 `/readyz` 代理到 `http://127.0.0.1:8080`。

完整 Compose 环境：

```powershell
Set-Location D:\project
Copy-Item .env.example .env
docker compose up -d --build
docker compose ps
```

浏览器访问 `http://127.0.0.1:8088`。默认本地 Bootstrap 用户仅来自 `.env.example` 的开发占位值，真实环境必须使用独立 Secret。

## 5. 容器与 Kubernetes

生产镜像由 `web/Dockerfile` 构建：Node 只参与静态资源编译，运行层使用非特权 Nginx。浏览器只访问同源 Web 地址；Nginx 将 API 请求转发到 `cloudsentinel-api` Service，避免在前端构建物中固化集群地址。

Kubernetes Base 包含：

- `cloudsentinel-web` Deployment，默认 2 副本；
- `cloudsentinel-web` ClusterIP Service，端口 80；
- Readiness、Liveness、资源限额、安全上下文和 PDB；
- NetworkPolicy 只允许 Ingress Controller 访问 Web，并只允许 Web 访问 API；
- Staging 与 Production Ingress 的 `/` 路径进入 Web，API 由 Web 同源代理。

镜像与 API、Worker、Migration 一样按 ACR VPC 地址和不可变 digest 发布。Production 仍然只晋级已在 Staging 验证过的相同 digest。

## 6. 首次两仓库发布顺序

本次是已有三镜像 GitOps 仓库首次增加第四个 Web 镜像，必须区分“一次性引导”和后续常规发布：

1. 源码 PR 通过前端与 Go CI 后合并，等待 Web 镜像在 ACR 构建完成并记录 digest。
2. 将 `deploy/gitops-repository` 中本次 Web Deployment、Service、NetworkPolicy、PDB、Ingress 和 Overlay 改动同步到独立 `cloudsentinel-gitops` 仓库。
3. 用第一步得到的真实 Web digest 替换启用的 `lab-staging`、`lab-production` 或目标 Overlay 中 `REPLACE_WEB_DIGEST`；不得提交占位符或可变标签。
4. 创建 GitOps PR，通过 `validate-gitops` 后合并；先观察 Staging，再按现有人工门禁晋级 Production。
5. 从下一次源码发布起，四个镜像 digest 将由 `release-images-and-stage` 自动写入 GitOps 发布 PR。

这套一次性顺序避免在镜像尚不存在时让 Argo CD 创建无法拉取的 Web Pod。

## 7. 验收标准

- `npm run typecheck`、`npm test`、`npm run build` 全部通过。
- 登录页桌面端和 390px 窄屏无横向溢出，表单有可访问标签。
- 后端不可达或返回错误时显示可恢复错误态，不出现空白页。
- 登录后七个业务页面可访问，401 会回到登录页。
- Staging 的 Web Deployment 为 `Available`，Web 与 API Service 都有 Endpoint。
- 浏览器从 Ingress 访问时，登录、资产、任务、结果、事件操作均走同源 HTTPS。
- Argo CD 显示 Staging `Synced/Healthy` 后，才允许 Production 人工同步。

本地浏览器真实登录联调只有在 API、MySQL、Redis 已启动时才成立；仅完成静态构建不得宣称业务 E2E 已通过。
