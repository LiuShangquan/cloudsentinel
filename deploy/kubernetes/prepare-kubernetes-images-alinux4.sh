#!/usr/bin/env bash

# Pull the frozen kubeadm image set from the CloudSentinel ACR VPC endpoint
# and align containerd's sandbox image. This script intentionally does not run
# kubeadm init/join, install Calico, or modify cloud security groups.

set -Eeuo pipefail

readonly KUBERNETES_VERSION="v1.35.7"
readonly KUBERNETES_RPM_VERSION="1.35.7-150500.1.1"
readonly CONTAINERD_RPM_VERSION="1.7.34-1.alnx4"
readonly CRI_SOCKET="unix:///run/containerd/containerd.sock"
readonly ACR_IMAGE_REPOSITORY="crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com/cloudsentinel0306"
readonly SCRIPT_REVISION="2026-08-13.1"

apply=false
expected_hostname=""
log_file=""

usage() {
	cat <<'EOF'
Usage:
  bash prepare-kubernetes-images-alinux4.sh \
    --apply --expected-hostname <node-name>

Allowed Kubernetes nodes:
  master-01 master-02 master-03
  worker-app-01 worker-app-02 worker-monitor worker-data-01

The script pulls the frozen kubeadm images from the ACR VPC endpoint and
aligns containerd's sandbox image. It does NOT run kubeadm init/join, install
Calico, or modify Alibaba Cloud security groups.
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
		printf '\nIMAGE_PREPARE_RESULT=PASS node=%s\n' "${expected_hostname}"
	else
		printf '\nIMAGE_PREPARE_RESULT=FAIL node=%s exit_code=%s\n' \
			"${expected_hostname:-unknown}" "${exit_code}" >&2
	fi
	if [[ -n "${log_file}" ]]; then
		printf 'IMAGE_PREPARE_LOG=%s\n' "${log_file}"
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

log_file="/root/cloudsentinel-image-prepare-${expected_hostname}-$(date +%Y%m%dT%H%M%S).log"
exec > >(tee -a "${log_file}") 2>&1

section "execution boundary"
printf 'script_revision=%s\n' "${SCRIPT_REVISION}"
printf 'node=%s\n' "${expected_hostname}"
printf 'image_repository=%s\n' "${ACR_IMAGE_REPOSITORY}"
printf 'This script will not run kubeadm init/join or install Calico.\n'

section "pre-change gates"
[[ -r /etc/os-release ]] || die "/etc/os-release is missing"
# shellcheck disable=SC1091
source /etc/os-release
[[ "${ID:-}" == "alinux" && "${VERSION_ID:-}" == "4" ]] ||
	die "Alibaba Cloud Linux 4 is required"
[[ "$(uname -m)" == "x86_64" ]] || die "x86_64 is required"

installed_kubeadm="$(rpm -q --qf '%{VERSION}-%{RELEASE}' kubeadm)"
[[ "${installed_kubeadm}" == "${KUBERNETES_RPM_VERSION}" ]] ||
	die "unexpected kubeadm RPM: ${installed_kubeadm}"

installed_containerd="$(rpm -q --qf '%{VERSION}-%{RELEASE}' containerd)"
[[ "${installed_containerd}" == "${CONTAINERD_RPM_VERSION}" ]] ||
	die "unexpected containerd RPM: ${installed_containerd}"

[[ "$(kubeadm version -o short)" == "${KUBERNETES_VERSION}" ]] ||
	die "unexpected kubeadm version"
[[ -S /run/containerd/containerd.sock ]] || die "containerd CRI socket is missing"
[[ "$(systemctl is-active containerd)" == "active" ]] ||
	die "containerd is not active"
crictl version

section "required ACR images"
mapfile -t required_images < <(
	kubeadm config images list \
		--kubernetes-version "${KUBERNETES_VERSION}" \
		--image-repository "${ACR_IMAGE_REPOSITORY}"
)
printf '%s\n' "${required_images[@]}"
[[ "${#required_images[@]}" -eq 7 ]] ||
	die "expected 7 kubeadm images, got ${#required_images[@]}"

section "pull pinned images from ACR VPC endpoint"
timeout 900 kubeadm config images pull \
	--kubernetes-version "${KUBERNETES_VERSION}" \
	--image-repository "${ACR_IMAGE_REPOSITORY}" \
	--cri-socket "${CRI_SOCKET}"

section "verify image cache"
for image in "${required_images[@]}"; do
	crictl inspecti "${image}" >/dev/null 2>&1 ||
		die "required image is missing after pull: ${image}"
	printf 'IMAGE_PRESENT=%s\n' "${image}"
done

pause_image="$(
	printf '%s\n' "${required_images[@]}" |
		awk '/\/pause:/{print; exit}'
)"
[[ -n "${pause_image}" ]] || die "pause image was not found"

section "align containerd sandbox image"
sandbox_image_count="$(
	grep -Ec '^[[:space:]]*sandbox_image[[:space:]]*=' \
		/etc/containerd/config.toml || true
)"
[[ "${sandbox_image_count}" -eq 1 ]] ||
	die "expected exactly one containerd sandbox_image setting"

current_pause_image="$(
	sed -nE \
		's/^[[:space:]]*sandbox_image[[:space:]]*=[[:space:]]*"([^"]+)".*/\1/p' \
		/etc/containerd/config.toml
)"

if [[ "${current_pause_image}" != "${pause_image}" ]]; then
	backup="/etc/containerd/config.toml.before-acr-pause.$(date +%Y%m%dT%H%M%S)"
	cp -a /etc/containerd/config.toml "${backup}"
	sed -ri \
		"s#^([[:space:]]*sandbox_image[[:space:]]*=[[:space:]]*).*\$#\1\"${pause_image}\"#" \
		/etc/containerd/config.toml
	systemctl restart containerd
	printf 'containerd_config_backup=%s\n' "${backup}"
else
	printf 'sandbox_image_already_aligned=true\n'
fi

section "final verification"
[[ "$(systemctl is-active containerd)" == "active" ]] ||
	die "containerd is not active after sandbox image alignment"

effective_config="$(containerd config dump)"
grep -Fq "sandbox_image = \"${pause_image}\"" <<<"${effective_config}" ||
	die "effective sandbox image does not match ACR pause image"
grep -Eq '^[[:space:]]*SystemdCgroup = true$' <<<"${effective_config}" ||
	die "effective SystemdCgroup is not true"

printf 'sandbox_image=%s\n' "${pause_image}"
printf 'SystemdCgroup=true\n'
crictl version
crictl images
printf 'KUBERNETES_ACR_IMAGE_BASELINE=PASS node=%s\n' "${expected_hostname}"
