# Compose MVP 演示手册

本手册只使用公开/JWT API 和可选的 `demo` Profile；绝不通过直接修改 MySQL 伪造业务结果。

## 1. 启动与验证

```powershell
docker compose --profile demo up -d --build
docker compose ps
Invoke-RestMethod http://127.0.0.1:8080/healthz
Invoke-RestMethod http://127.0.0.1:8080/readyz
```

使用 README 中的示例登录并设置 `$headers`。创建地址为 `mvp-http-target` 的 Host、目标为 `http://mvp-http-target:8080/` 的 HTTP Service，以及周期为 10 秒的 Probe Task。如果具有相同唯一名称的演示资源已经存在，请直接复用。

预期的健康链路：

```text
已调度任务 -> Redis Stream -> Worker -> HTTP 200 -> Succeeded Result
-> cloudsentinel_probe_status{service_id,task_id,probe_type="http"} 1
```

通过 Result API 和 Prometheus Query API 验证：

```powershell
Invoke-RestMethod -Headers $headers 'http://127.0.0.1:8080/api/v1/probe-results?page=1&page_size=20'
Invoke-RestMethod 'http://127.0.0.1:9090/api/v1/query?query=cloudsentinel_probe_status'
```

访问 `http://127.0.0.1:3000` 的 Grafana，其中已自动预置 Prometheus Datasource 和 `CloudSentinel Overview` Dashboard。

## 2. 触发并解决 Incident

```powershell
docker compose stop mvp-http-target
```

探测失败后，最新状态变为 `0`。`CloudSentinelProbeFailure` 规则先保持 Pending 30 秒，然后进入 Firing。Alertmanager 等待 5 秒，使用 Machine Token 向 Webhook 认证，并创建一条 `firing` Incident。重复通知会更新相同 `event_key` 的 `occurrence_count`，不会创建第二条 Incident。

```powershell
Invoke-RestMethod http://127.0.0.1:9090/api/v1/alerts
Invoke-RestMethod http://127.0.0.1:9093/api/v2/alerts
Invoke-RestMethod -Headers $headers 'http://127.0.0.1:8080/api/v1/incidents?page=1&page_size=20'
```

恢复目标：

```powershell
docker compose --profile demo start mvp-http-target
```

下一次探测成功后，状态变为 `1`；Prometheus 解决告警；Alertmanager 发送 `send_resolved`；已有 Incident 变为 `resolved`。通过专用状态迁移 Endpoint 将其关闭：

```powershell
Invoke-RestMethod -Method Post -Headers $headers http://127.0.0.1:8080/api/v1/incidents/<incident-id>/close
```

预期最终状态：`closed`。

## 3. 运维检查

```powershell
docker compose exec -T prometheus promtool check config /etc/prometheus/prometheus.yml
docker compose exec -T prometheus promtool check rules /etc/prometheus/rules/cloudsentinel.yml
docker compose exec -T alertmanager amtool check-config /tmp/alertmanager.yml
docker compose logs --tail=200 cloudsentinel-api cloudsentinel-worker prometheus alertmanager grafana
```

使用 `docker compose --profile demo down` 停止项目并保留数据。除非明确要永久删除所有 Named Volume 数据，否则绝不能添加 `-v`。

