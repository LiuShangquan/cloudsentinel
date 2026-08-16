#!/usr/bin/env bash
set -Eeuo pipefail

readonly NAMESPACE="monitoring"
readonly APPLICATION="cloudsentinel-monitoring-lab"
readonly ATTEMPTS="${ATTEMPTS:-36}"
readonly INTERVAL_SECONDS="${INTERVAL_SECONDS:-10}"
readonly KUBECTL_REQUEST_TIMEOUT="${KUBECTL_REQUEST_TIMEOUT:-20s}"

fail() {
  echo "MONITORING_PLATFORM_VERIFY=FAIL reason=$1" >&2
  exit 1
}

command -v kubectl >/dev/null 2>&1 || fail 'kubectl-not-found'
kube() {
  kubectl --request-timeout="${KUBECTL_REQUEST_TIMEOUT}" "$@"
}

echo '===== bounded readiness observation ====='
converged=false
for attempt in $(seq 1 "${ATTEMPTS}"); do
  prometheus="$(kube -n "${NAMESPACE}" get statefulset prometheus \
    -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || echo absent)"
  alertmanager="$(kube -n "${NAMESPACE}" get statefulset alertmanager \
    -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || echo absent)"
  grafana="$(kube -n "${NAMESPACE}" get deployment grafana \
    -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || echo absent)"
  metrics_server="$(kube -n "${NAMESPACE}" get deployment metrics-server \
    -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || echo absent)"
  application="$(kube -n argocd get application "${APPLICATION}" \
    -o jsonpath='{.status.sync.status}|{.status.health.status}' 2>/dev/null || echo absent)"
  echo "attempt=${attempt} prometheus=${prometheus} alertmanager=${alertmanager} grafana=${grafana} metrics_server=${metrics_server} application=${application}"

  if [[ "${prometheus}" == '1/1' && \
        "${alertmanager}" == '1/1' && \
        "${grafana}" == '1/1' && \
        "${metrics_server}" == '1/1' && \
        "${application}" == 'Synced|Healthy' ]]; then
    converged=true
    break
  fi
  sleep "${INTERVAL_SECONDS}"
done
[[ "${converged}" == true ]] || fail 'workloads-did-not-converge'

echo '===== storage ====='
kube -n "${NAMESPACE}" get pvc \
  -o custom-columns='NAME:.metadata.name,STATUS:.status.phase,VOLUME:.spec.volumeName'
bound_count="$(kube -n "${NAMESPACE}" get pvc \
  -o jsonpath='{range .items[?(@.status.phase=="Bound")]}{.metadata.name}{"\n"}{end}' |
  grep -c .)"
[[ "${bound_count}" == "3" ]] || fail 'not-all-pvcs-bound'

echo '===== Metrics API ====='
metrics_payload="$(kube get --raw='/apis/metrics.k8s.io/v1beta1/nodes')"
grep -Fq 'worker-monitor' <<<"${metrics_payload}" || fail 'metrics-api-missing-worker-monitor'

echo '===== component endpoints ====='
prometheus_ready="$(kube get --raw='/api/v1/namespaces/monitoring/services/http:prometheus:9090/proxy/-/ready')"
alertmanager_ready="$(kube get --raw='/api/v1/namespaces/monitoring/services/http:alertmanager:9093/proxy/-/ready')"
grafana_health="$(kube get --raw='/api/v1/namespaces/monitoring/services/http:grafana:3000/proxy/api/health')"
grep -Fq 'Prometheus Server is Ready' <<<"${prometheus_ready}" || fail 'prometheus-not-ready'
grep -Fq 'OK' <<<"${alertmanager_ready}" || fail 'alertmanager-not-ready'
grep -Fq '"database": "ok"' <<<"${grafana_health}" || \
  grep -Fq '"database":"ok"' <<<"${grafana_health}" || \
  fail 'grafana-database-not-ready'

echo '===== Prometheus discovery ====='
targets="$(kube get --raw='/api/v1/namespaces/monitoring/services/http:prometheus:9090/proxy/api/v1/targets')"
grep -Fq 'cloudsentinel-api' <<<"${targets}" || fail 'api-target-not-discovered'
grep -Fq 'cloudsentinel-worker' <<<"${targets}" || fail 'worker-target-not-discovered'
grep -Fq '"health":"up"' <<<"${targets}" || fail 'no-up-prometheus-target'

echo '===== final state ====='
kube -n "${NAMESPACE}" get pod -o wide
kube -n argocd get application "${APPLICATION}" \
  -o custom-columns='NAME:.metadata.name,SYNC:.status.sync.status,HEALTH:.status.health.status,REVISION:.status.sync.revision'
echo 'MONITORING_PLATFORM_VERIFY=PASS'
