#!/usr/bin/env bash
set -Eeuo pipefail

readonly EXPECTED_NODE="worker-monitor"
readonly ROOT_DIR="/var/lib/cloudsentinel/monitoring"
readonly REQUIRED_AVAILABLE_BYTES=$((12 * 1024 * 1024 * 1024))
readonly LOG_FILE="/root/cloudsentinel-monitoring-storage-$(hostname -s)-$(date +%Y%m%dT%H%M%S).log"

exec > >(tee -a "${LOG_FILE}") 2>&1

fail() {
  echo "MONITORING_STORAGE_PREPARE=FAIL reason=$1" >&2
  exit 1
}

echo '===== execution boundary ====='
echo "node=$(hostname -s)"
echo "root_dir=${ROOT_DIR}"
echo 'This script only prepares Local PV directories; it does not apply Kubernetes resources.'

[[ "$(id -u)" -eq 0 ]] || fail 'root-required'
[[ "$(hostname -s)" == "${EXPECTED_NODE}" ]] || fail "must-run-on-${EXPECTED_NODE}"
systemctl is-active --quiet kubelet || fail 'kubelet-not-active'
systemctl is-active --quiet containerd || fail 'containerd-not-active'

available_bytes="$(df --output=avail -B1 / | tail -n 1 | tr -d ' ')"
[[ "${available_bytes}" =~ ^[0-9]+$ ]] || fail 'capacity-undetermined'
(( available_bytes >= REQUIRED_AVAILABLE_BYTES )) || fail 'less-than-12-GiB-available'

install -d -o root -g root -m 0750 "${ROOT_DIR}"
install -d -o 65534 -g 65534 -m 0700 "${ROOT_DIR}/prometheus"
install -d -o 65534 -g 65534 -m 0700 "${ROOT_DIR}/alertmanager"
install -d -o 472 -g 472 -m 0700 "${ROOT_DIR}/grafana"

echo '===== verification ====='
for path in \
  "${ROOT_DIR}/prometheus" \
  "${ROOT_DIR}/alertmanager" \
  "${ROOT_DIR}/grafana"; do
  stat -c 'numeric_owner=%u:%g mode=%a path=%n' "${path}"
done
df -hT /
echo "available_bytes=${available_bytes}"
echo 'MONITORING_STORAGE_PREPARE=PASS'
echo "MONITORING_STORAGE_PREPARE_LOG=${LOG_FILE}"

