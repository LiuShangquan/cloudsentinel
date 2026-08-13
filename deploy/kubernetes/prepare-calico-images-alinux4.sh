#!/usr/bin/env bash

# Pull the frozen Calico Linux image set from the CloudSentinel ACR VPC
# endpoint. This script intentionally does not run kubeadm, install the Tigera
# Operator, apply CRDs, or modify cloud security groups.

set -Eeuo pipefail

readonly CALICO_VERSION="v3.32.1"
readonly TIGERA_OPERATOR_VERSION="v1.42.3"
readonly ACR_IMAGE_ROOT="crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com/cloudsentinel0306"
readonly SCRIPT_REVISION="2026-08-13.1"

apply=false
expected_hostname=""
log_file=""

usage() {
	cat <<'EOF'
Usage:
  bash prepare-calico-images-alinux4.sh \
    --apply --expected-hostname <node-name>

Allowed Kubernetes nodes:
  master-01 master-02 master-03
  worker-app-01 worker-app-02 worker-monitor worker-data-01

The script pulls the frozen Calico Linux images from the ACR VPC endpoint.
It does NOT run kubeadm, install Calico, apply CRDs, or modify Alibaba Cloud
security groups.
EOF
}

die() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

section() {
	printf '\n===== %s =====\n' "$1"
}

on_exit() {
	local exit_code=$?
	if [[ "${exit_code}" -eq 0 ]]; then
		printf '\nCALICO_IMAGE_PREPARE_RESULT=PASS node=%s\n' "${expected_hostname}"
	else
		printf '\nCALICO_IMAGE_PREPARE_RESULT=FAIL node=%s exit_code=%s\n' \
			"${expected_hostname:-unknown}" "${exit_code}" >&2
	fi
	if [[ -n "${log_file}" ]]; then
		printf 'CALICO_IMAGE_PREPARE_LOG=%s\n' "${log_file}"
	fi
}

trap on_exit EXIT

while (($# > 0)); do
	case "$1" in
	--apply)
		apply=true
		shift
		;;
	--expected-hostname)
		(($# >= 2)) || die "--expected-hostname requires a value"
		expected_hostname="$2"
		shift 2
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

[[ "${apply}" == "true" ]] || {
	usage
	die "no changes made; pass --apply to approve image preparation"
}

[[ "${EUID}" -eq 0 ]] || die "run this script as root"
[[ -n "${expected_hostname}" ]] || die "--expected-hostname is required"

case "${expected_hostname}" in
master-01 | master-02 | master-03 | \
	worker-app-01 | worker-app-02 | worker-monitor | worker-data-01) ;;
*)
	die "node is outside the approved Kubernetes inventory: ${expected_hostname}"
	;;
esac

actual_hostname="$(hostname -s)"
[[ "${actual_hostname}" == "${expected_hostname}" ]] ||
	die "hostname mismatch: expected ${expected_hostname}, got ${actual_hostname}"

log_file="/root/cloudsentinel-calico-image-prepare-${expected_hostname}-$(date +%Y%m%dT%H%M%S).log"
exec > >(tee -a "${log_file}") 2>&1

section "execution boundary"
printf 'script_revision=%s\n' "${SCRIPT_REVISION}"
printf 'node=%s\n' "${expected_hostname}"
printf 'calico_version=%s\n' "${CALICO_VERSION}"
printf 'tigera_operator_version=%s\n' "${TIGERA_OPERATOR_VERSION}"
printf 'image_root=%s\n' "${ACR_IMAGE_ROOT}"
printf 'This script will not run kubeadm or install Calico.\n'

section "pre-change gates"
[[ -r /etc/os-release ]] || die "/etc/os-release is missing"
# shellcheck disable=SC1091
source /etc/os-release
[[ "${ID:-}" == "alinux" && "${VERSION_ID:-}" == "4" ]] ||
	die "Alibaba Cloud Linux 4 is required"
[[ "$(uname -m)" == "x86_64" ]] || die "x86_64 is required"
[[ -S /run/containerd/containerd.sock ]] || die "containerd CRI socket is missing"
[[ "$(systemctl is-active containerd)" == "active" ]] ||
	die "containerd is not active"
command -v crictl >/dev/null 2>&1 || die "crictl is not installed"
crictl version

readonly -a required_images=(
	"${ACR_IMAGE_ROOT}/calico-operator:${TIGERA_OPERATOR_VERSION}"
	"${ACR_IMAGE_ROOT}/calico-node:${CALICO_VERSION}"
	"${ACR_IMAGE_ROOT}/calico-cni:${CALICO_VERSION}"
	"${ACR_IMAGE_ROOT}/calico-kube-controllers:${CALICO_VERSION}"
	"${ACR_IMAGE_ROOT}/calico-typha:${CALICO_VERSION}"
	"${ACR_IMAGE_ROOT}/calico-csi:${CALICO_VERSION}"
	"${ACR_IMAGE_ROOT}/calico-node-driver-registrar:${CALICO_VERSION}"
	"${ACR_IMAGE_ROOT}/calico-pod2daemon-flexvol:${CALICO_VERSION}"
	"${ACR_IMAGE_ROOT}/calico-key-cert-provisioner:${CALICO_VERSION}"
)

section "required ACR images"
printf '%s\n' "${required_images[@]}"
[[ "${#required_images[@]}" -eq 9 ]] ||
	die "expected 9 Calico images, got ${#required_images[@]}"

section "pull pinned images from ACR VPC endpoint"
for image in "${required_images[@]}"; do
	timeout 600 crictl pull "${image}"
done

section "verify image cache"
for image in "${required_images[@]}"; do
	crictl inspecti "${image}" >/dev/null 2>&1 ||
		die "required image is missing after pull: ${image}"
	printf 'CALICO_IMAGE_PRESENT=%s\n' "${image}"
done

section "final verification"
[[ "$(systemctl is-active containerd)" == "active" ]] ||
	die "containerd is not active after image preparation"
crictl version
printf 'CALICO_ACR_IMAGE_BASELINE=PASS node=%s\n' "${expected_hostname}"
