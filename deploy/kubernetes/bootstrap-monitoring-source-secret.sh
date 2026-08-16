#!/usr/bin/env bash
set -Eeuo pipefail

readonly SOURCE_NAMESPACE="cloudsentinel-secret-source"
readonly SOURCE_SECRET="cloudsentinel-monitoring"
readonly KUBECTL_REQUEST_TIMEOUT="${KUBECTL_REQUEST_TIMEOUT:-20s}"

fail() {
  echo "MONITORING_SOURCE_SECRET=FAIL reason=$1" >&2
  exit 1
}

command -v kubectl >/dev/null 2>&1 || fail 'kubectl-not-found'
kube() {
  kubectl --request-timeout="${KUBECTL_REQUEST_TIMEOUT}" "$@"
}

kube get namespace "${SOURCE_NAMESPACE}" >/dev/null || fail 'source-namespace-not-found'
kube -n "${SOURCE_NAMESPACE}" get secret cloudsentinel-staging-app >/dev/null || \
  fail 'staging-source-secret-not-found'

grafana_user="${GRAFANA_ADMIN_USER:-admin}"
[[ "${grafana_user}" =~ ^[A-Za-z0-9_.@-]{3,64}$ ]] || fail 'invalid-grafana-username'

read -r -s -p 'Grafana administrator password: ' grafana_password
echo
read -r -s -p 'Confirm Grafana administrator password: ' grafana_password_confirm
echo

[[ "${grafana_password}" == "${grafana_password_confirm}" ]] || fail 'passwords-do-not-match'
(( ${#grafana_password} >= 16 )) || fail 'password-must-have-at-least-16-characters'

credential_dir="$(mktemp -d)"
trap 'unset grafana_password grafana_password_confirm; rm -rf -- "${credential_dir}"' EXIT
chmod 0700 "${credential_dir}"

kube -n "${SOURCE_NAMESPACE}" get secret cloudsentinel-staging-app \
  -o jsonpath='{.data.ALERTMANAGER_WEBHOOK_TOKEN}' | \
  base64 --decode >"${credential_dir}/ALERTMANAGER_WEBHOOK_TOKEN"
[[ -s "${credential_dir}/ALERTMANAGER_WEBHOOK_TOKEN" ]] || \
  fail 'staging-webhook-token-is-empty'

printf '%s' "${grafana_user}" >"${credential_dir}/GRAFANA_ADMIN_USER"
printf '%s' "${grafana_password}" >"${credential_dir}/GRAFANA_ADMIN_PASSWORD"
chmod 0600 "${credential_dir}"/*

kube -n "${SOURCE_NAMESPACE}" create secret generic "${SOURCE_SECRET}" \
  --from-file=ALERTMANAGER_WEBHOOK_TOKEN="${credential_dir}/ALERTMANAGER_WEBHOOK_TOKEN" \
  --from-file=GRAFANA_ADMIN_USER="${credential_dir}/GRAFANA_ADMIN_USER" \
  --from-file=GRAFANA_ADMIN_PASSWORD="${credential_dir}/GRAFANA_ADMIN_PASSWORD" \
  --dry-run=client \
  -o yaml |
  kube apply --server-side \
    --field-manager=cloudsentinel-secret-bootstrap \
    -f -

key_count="$(kube -n "${SOURCE_NAMESPACE}" get secret "${SOURCE_SECRET}" \
  -o go-template='{{len .data}}')"
[[ "${key_count}" == "3" ]] || fail 'unexpected-key-count'

unset grafana_password grafana_password_confirm
echo "source_secret=${SOURCE_SECRET} key_count=${key_count}"
echo 'MONITORING_SOURCE_SECRET=PASS'
