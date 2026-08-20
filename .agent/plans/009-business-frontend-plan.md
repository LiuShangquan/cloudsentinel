# CloudSentinel 业务管理前端计划

## 目标

在不改变 CloudSentinel 现有业务边界和 API 分层的前提下，新增一个可独立构建、可通过 Docker Compose 本地运行、可由企业 GitOps 流程发布的正式管理门户。前端必须覆盖当前已经实现的业务能力，而不是复制 Grafana、Argo CD 或 Kubernetes 管理功能。

## 产品边界

- 登录与当前用户信息。
- 运行总览：资产、服务、探测任务、最近探测和事件的聚合视图。
- 主机、监控服务、探测任务的新增、查看、编辑和逻辑禁用。
- 探测结果列表和详情。
- 故障事件列表、详情以及确认、处理中、关闭的合法状态迁移。
- 不实现用户管理、角色权限、OAuth/OIDC、自动修复或复杂报表编辑器。
- Grafana 继续负责时序监控；Argo CD 继续负责 GitOps；业务门户不重复它们的职责。

## 技术方案

- React、TypeScript、Vite、React Router。
- 使用项目内设计系统和原生 CSS，避免引入重量级组件框架。
- JWT 保存在 `sessionStorage`，不写入日志、不进入 URL；收到 401 时清理会话并返回登录页。
- Nginx 提供静态文件、SPA 回退和 `/api/` 同源反向代理。
- 前端作为独立 `cloudsentinel-web` 非 Root 镜像发布。
- Compose 与 Kubernetes 均只通过内部网络访问 API；浏览器不直接接触集群内部地址。

## 信息架构

1. 登录页。
2. 总览。
3. 资产管理：主机、监控服务。
4. 探测中心：探测任务、执行结果。
5. 事件中心：故障事件与状态处理。
6. 当前用户与会话退出。

## 实施阶段

### 阶段一：产品骨架

- 建立前端工程、路由、应用壳、主题、响应式布局和 API Client。
- 形成登录页和总览页的首个可运行版本。

### 阶段二：业务闭环

- 实现所有列表、分页、表单、详情、状态标签、空状态和错误反馈。
- 实现 Host -> Service -> Probe Task 的引导式创建流程。
- 实现 Incident 合法状态操作。

### 阶段三：交付集成

- 新增 Web 容器、Compose 服务和健康检查。
- 新增 Kubernetes Deployment、Service、PDB、NetworkPolicy 与 Overlay 镜像声明。
- 扩展发布工作流和 digest 更新工具，使 Web 镜像与 API/Worker/Migration 一起通过 GitOps 晋级。

### 阶段四：验证

- TypeScript 类型检查、组件测试、生产构建。
- Docker 镜像构建与 Nginx 配置检查。
- Go 测试、镜像脚本测试、Kustomize 渲染和安全边界检查。

## 完成标准

- 未登录用户只能访问登录页。
- 用户可以从页面完成主机、服务和探测任务的完整创建流程。
- 所有现有业务 API 都有对应的可用界面。
- 401、403、404、409、500、网络超时和空数据均有明确反馈。
- 前端可在桌面和窄屏正常使用，并满足键盘操作和基础无障碍要求。
- Compose 与四个 Kubernetes Overlay 均能渲染 Web 工作负载。
- CI 能验证前端和不可变 Web 镜像引用。

## 实施状态

- 阶段一至阶段三：已完成。
- 阶段四：TypeScript 类型检查、前端单测、生产构建、生产依赖审计、镜像引用工具测试、Compose 配置解析和四个 Kustomize Overlay 渲染均已通过。
- 浏览器验收：登录页已完成桌面与窄屏检查，网络失败可显示明确错误且无页面崩溃。
- 当前限制：本机 Docker 守护进程未运行，因此 Web 容器实际构建和连接真实本地 API 的登录后浏览器 E2E 标记为 `NOT VERIFIED`；Go 测试二进制被本机 Application Control 策略拦截，但 `go vet ./...` 已通过。
