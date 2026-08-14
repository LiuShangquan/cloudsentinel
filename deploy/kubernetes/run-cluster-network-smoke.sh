#!/usr/bin/env bash

# Run reversible Pod, Service, DNS and NetworkPolicy acceptance checks against
# the two application workers. Test resources are deleted only after every
# assertion passes; failures preserve the namespace for diagnosis.

set -Eeuo pipefail

readonly SCRIPT_REVISION="2026-08-14.2"
readonly TEST_NAMESPACE="cloudsentinel-smoke"
readonly EXPECTED_RESPONSE="cloudsentinel-network-smoke"

apply=false
log_file=""

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly NETWORK_MANIFEST="${script_dir}/tests/network-smoke.yaml"
readonly POLICY_MANIFEST="${script_dir}/tests/network-policy-smoke.yaml"

usage() {
	cat <<'EOF'
Usage:
  bash run-cluster-network-smoke.sh --apply

Run this script only on master-01 after the fixed toolbox image has been
digest-verified and pre-pulled on worker-app-01 and worker-app-02.
EOF
}

die() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

on_exit() {
	local exit_code=$?
	if [[ "${exit_code}" -eq 0 ]]; then
		printf '\nCLUSTER_NETWORK_SMOKE_RESULT=PASS\n'
	else
		printf '\nCLUSTER_NETWORK_SMOKE_RESULT=FAIL exit_code=%s\n' \
			"${exit_code}" >&2
		if kubectl get namespace "${TEST_NAMESPACE}" >/dev/null 2>&1; then
			printf 'Diagnostic resources were preserved in namespace %s.\n' \
				"${TEST_NAMESPACE}" >&2
		else
			printf 'No diagnostic namespace exists; failure occurred before resource creation.\n' >&2
		fi
	fi
	[[ -z "${log_file}" ]] || printf 'CLUSTER_NETWORK_SMOKE_LOG=%s\n' "${log_file}"
}

trap on_exit EXIT

while (($# > 0)); do
	case "$1" in
	--apply)
		apply=true
		shift
		;;
	--help | -h)
		usage
		exit 0
		;;
	*)
		usage >&2
		die "unknown argument: $1"
		;;
	esac
done

[[ "${apply}" == "true" ]] || die "no test resources created; pass --apply"
[[ "${EUID}" -eq 0 ]] || die "run this script as root"
[[ "$(hostname -s)" == "master-01" ]] || die "run only on master-01"
[[ -r /etc/kubernetes/admin.conf ]] || die "admin kubeconfig is missing"
[[ -r "${NETWORK_MANIFEST}" ]] || die "network smoke manifest is missing"
[[ -r "${POLICY_MANIFEST}" ]] || die "network policy manifest is missing"

export KUBECONFIG=/etc/kubernetes/admin.conf
log_file="/root/cloudsentinel-cluster-network-smoke-$(date +%Y%m%dT%H%M%S).log"
exec > >(tee -a "${log_file}") 2>&1

printf 'script_revision=%s\n' "${SCRIPT_REVISION}"
printf 'network_manifest=%s\n' "${NETWORK_MANIFEST}"
printf 'policy_manifest=%s\n' "${POLICY_MANIFEST}"

[[ "$(kubectl get nodes --no-headers | wc -l)" -eq 7 ]] ||
	die "expected exactly 7 Kubernetes nodes"
[[ "$(kubectl get nodes --no-headers | awk '$2 != "Ready" {count++} END {print count+0}')" -eq 0 ]] ||
	die "all Kubernetes nodes must be Ready"
[[ "$(kubectl get nodes -l node-role=app --no-headers | wc -l)" -eq 2 ]] ||
	die "expected exactly two application workers"
if kubectl get namespace "${TEST_NAMESPACE}" >/dev/null 2>&1; then
	die "test namespace already exists; inspect and remove it deliberately first"
fi

# The Namespace and its namespaced objects share one multi-document manifest.
# Server-side dry-run does not persist the Namespace before validating later
# documents, so use client-side dry-run here and let the real apply below
# create resources in manifest order.
kubectl apply --dry-run=client -f "${NETWORK_MANIFEST}" >/dev/null
kubectl apply --dry-run=client -f "${POLICY_MANIFEST}" >/dev/null

printf '\n===== create smoke resources =====\n'
kubectl apply -f "${NETWORK_MANIFEST}"
kubectl wait \
	--namespace "${TEST_NAMESPACE}" \
	--for=condition=Ready \
	pod/smoke-server \
	pod/smoke-client-allowed \
	pod/smoke-client-denied \
	--timeout=180s

kubectl get pods -n "${TEST_NAMESPACE}" -o wide
[[ "$(kubectl get pod -n "${TEST_NAMESPACE}" smoke-server -o jsonpath='{.spec.nodeName}')" == "worker-app-02" ]] ||
	die "smoke-server was not scheduled on worker-app-02"
[[ "$(kubectl get pod -n "${TEST_NAMESPACE}" smoke-client-allowed -o jsonpath='{.spec.nodeName}')" == "worker-app-01" ]] ||
	die "smoke-client-allowed was not scheduled on worker-app-01"

server_ip="$(kubectl get pod -n "${TEST_NAMESPACE}" smoke-server -o jsonpath='{.status.podIP}')"
[[ "${server_ip}" == 10.244.* ]] || die "unexpected smoke-server Pod IP: ${server_ip}"

printf '\n===== cross-node Pod IP =====\n'
pod_response="$(kubectl exec -n "${TEST_NAMESPACE}" smoke-client-allowed -- \
	wget -T 5 -qO- "http://${server_ip}:8080/")"
[[ "${pod_response}" == "${EXPECTED_RESPONSE}" ]] || die "Pod IP response mismatch"
printf 'pod_ip_response=%s\n' "${pod_response}"

printf '\n===== ClusterIP and EndpointSlice =====\n'
kubectl get service -n "${TEST_NAMESPACE}" smoke-server -o wide
kubectl get endpointslice -n "${TEST_NAMESPACE}" \
	-l kubernetes.io/service-name=smoke-server -o wide
service_response="$(kubectl exec -n "${TEST_NAMESPACE}" smoke-client-denied -- \
	wget -T 5 -qO- http://smoke-server/)"
[[ "${service_response}" == "${EXPECTED_RESPONSE}" ]] || die "ClusterIP response mismatch"
printf 'cluster_ip_response=%s\n' "${service_response}"

printf '\n===== cluster DNS =====\n'
kubectl exec -n "${TEST_NAMESPACE}" smoke-client-allowed -- \
	nslookup kubernetes.default.svc.cluster.local
kubectl exec -n "${TEST_NAMESPACE}" smoke-client-allowed -- \
	nslookup smoke-server.cloudsentinel-smoke.svc.cluster.local

printf '\n===== Calico NetworkPolicy =====\n'
kubectl apply -f "${POLICY_MANIFEST}"
sleep 3

allowed_response="$(kubectl exec -n "${TEST_NAMESPACE}" smoke-client-allowed -- \
	wget -T 5 -qO- http://smoke-server/)"
[[ "${allowed_response}" == "${EXPECTED_RESPONSE}" ]] ||
	die "allowed client could not reach the server"
printf 'allowed_client_response=%s\n' "${allowed_response}"

if kubectl exec -n "${TEST_NAMESPACE}" smoke-client-denied -- \
	wget -T 5 -qO- http://smoke-server/; then
	die "denied client unexpectedly reached the server"
fi
printf 'denied_client=BLOCKED\n'

printf '\n===== cleanup =====\n'
kubectl delete namespace "${TEST_NAMESPACE}" --wait=true --timeout=180s
if kubectl get namespace "${TEST_NAMESPACE}" >/dev/null 2>&1; then
	die "test namespace still exists after cleanup"
fi

printf 'CROSS_NODE_POD_NETWORK=PASS\n'
printf 'CLUSTER_IP_SERVICE=PASS\n'
printf 'CLUSTER_DNS=PASS\n'
printf 'CALICO_NETWORK_POLICY=PASS\n'
