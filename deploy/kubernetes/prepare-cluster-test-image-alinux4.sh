#!/usr/bin/env bash

# Pull and verify the fixed cluster smoke-test toolbox image on the two
# application workers. The script does not deploy Pods or change Kubernetes.

set -Eeuo pipefail

readonly TOOLBOX_VERSION="1.37.0"
readonly TOOLBOX_IMAGE="crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com/cloudsentinel0306/cloudsentinel-toolbox:${TOOLBOX_VERSION}"
readonly SCRIPT_REVISION="2026-08-14.1"

apply=false
expected_hostname=""
expected_digest=""
log_file=""

usage() {
	cat <<'EOF'
Usage:
  bash prepare-cluster-test-image-alinux4.sh \
    --apply \
    --expected-hostname worker-app-01-or-worker-app-02 \
    --expected-digest sha256:64-hex-digest

The script only pulls and verifies the fixed smoke-test image. It does not
create Kubernetes resources or modify the containerd configuration.
EOF
}

die() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

on_exit() {
	local exit_code=$?
	if [[ "${exit_code}" -eq 0 ]]; then
		printf '\nCLUSTER_TEST_IMAGE_PREPARE_RESULT=PASS node=%s\n' \
			"${expected_hostname}"
	else
		printf '\nCLUSTER_TEST_IMAGE_PREPARE_RESULT=FAIL node=%s exit_code=%s\n' \
			"${expected_hostname:-unknown}" "${exit_code}" >&2
	fi
	[[ -z "${log_file}" ]] || printf 'CLUSTER_TEST_IMAGE_PREPARE_LOG=%s\n' "${log_file}"
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
	--expected-digest)
		(($# >= 2)) || die "--expected-digest requires a value"
		expected_digest="$2"
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

[[ "${apply}" == "true" ]] || die "no changes made; pass --apply"
[[ "${EUID}" -eq 0 ]] || die "run this script as root"
case "${expected_hostname}" in
worker-app-01 | worker-app-02) ;;
*) die "only the two application workers are allowed" ;;
esac
[[ "${expected_digest}" =~ ^sha256:[0-9a-f]{64}$ ]] ||
	die "--expected-digest must be a sha256 digest"
[[ "$(hostname -s)" == "${expected_hostname}" ]] ||
	die "hostname mismatch: expected ${expected_hostname}, got $(hostname -s)"

log_file="/root/cloudsentinel-cluster-test-image-${expected_hostname}-$(date +%Y%m%dT%H%M%S).log"
exec > >(tee -a "${log_file}") 2>&1

printf 'script_revision=%s\n' "${SCRIPT_REVISION}"
printf 'node=%s\n' "${expected_hostname}"
printf 'image=%s\n' "${TOOLBOX_IMAGE}"
printf 'expected_digest=%s\n' "${expected_digest}"

[[ -S /run/containerd/containerd.sock ]] || die "containerd CRI socket is missing"
[[ "$(systemctl is-active containerd)" == "active" ]] || die "containerd is not active"
command -v crictl >/dev/null 2>&1 || die "crictl is not installed"

timeout 600 crictl pull "${TOOLBOX_IMAGE}"

inspect_output="$(crictl inspecti "${TOOLBOX_IMAGE}")"
grep -Fq "${expected_digest}" <<<"${inspect_output}" ||
	die "cached image digest does not match ${expected_digest}"

printf 'CLUSTER_TEST_IMAGE_PRESENT=%s@%s\n' \
	"${TOOLBOX_IMAGE}" "${expected_digest}"
