#!/usr/bin/env bash

# Install the frozen CloudSentinel Kubernetes node baseline on Alibaba Cloud
# Linux 4. This script intentionally does not run kubeadm init/join, modify
# Alibaba Cloud security groups, disable firewalld, or install Calico.

set -Eeuo pipefail

readonly KUBERNETES_VERSION="v1.35.7"
readonly KUBERNETES_RPM_VERSION="1.35.7-150500.1.1"
readonly KUBERNETES_MINOR="v1.35"
readonly KUBERNETES_REPOSITORY="https://pkgs.k8s.io/core:/stable:/v1.35/rpm/"
readonly CONTAINERD_RPM_VERSION="1.7.34-1.alnx4"
readonly CRI_TOOLS_RPM_VERSION="1.35.0-150500.1.1"
readonly CNI_PLUGINS_RPM_VERSION="1.3.0-2.alnx4"
readonly NLB_HOST="nlb-axdxf21jombh758m09.cn-beijing.nlb.aliyuncsslb.com"
readonly SCRIPT_REVISION="2026-08-13.4"

apply=false
expected_hostname=""

usage() {
  cat <<'EOF'
Usage:
  bash bootstrap-alinux4-node.sh --apply --expected-hostname <node-name>

Allowed Kubernetes nodes:
  master-01 master-02 master-03
  worker-app-01 worker-app-02 worker-monitor worker-data-01

The script installs and configures containerd, kubeadm, kubelet and kubectl.
It does NOT run kubeadm init/join, disable firewalld, or install Calico.
EOF
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

section() {
  printf '\n===== %s =====\n' "$1"
}

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
    --help|-h)
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
  die "no changes made; pass --apply to approve node modification"
}

[[ "${EUID}" -eq 0 ]] || die "run this script as root"
[[ -n "${expected_hostname}" ]] || die "--expected-hostname is required"

declare -A node_private_ip=(
  [master-01]="172.29.253.154"
  [master-02]="172.29.253.155"
  [master-03]="172.29.253.152"
  [worker-app-01]="172.16.41.182"
  [worker-app-02]="172.16.41.183"
  [worker-monitor]="172.29.253.150"
  [worker-data-01]="172.29.253.151"
)

[[ -n "${node_private_ip[${expected_hostname}]:-}" ]] ||
  die "${expected_hostname} is not an approved Kubernetes node"

actual_hostname="$(hostname -s)"
[[ "${actual_hostname}" == "${expected_hostname}" ]] ||
  die "hostname mismatch: expected ${expected_hostname}, got ${actual_hostname}"

expected_private_ip="${node_private_ip[${expected_hostname}]}"
ip -4 -o address show scope global |
  awk '{print $4}' |
  cut -d/ -f1 |
  grep -Fxq "${expected_private_ip}" ||
  die "${expected_hostname} does not own expected private IP ${expected_private_ip}"

[[ ! -e /etc/kubernetes/admin.conf ]] ||
  die "kubeadm state already exists; this bootstrap script refuses to continue"

if [[ -d /etc/kubernetes/manifests ]] &&
  find /etc/kubernetes/manifests -mindepth 1 -maxdepth 1 -type f -print -quit |
    grep -q .; then
  die "static Pod manifests already exist; refusing to overwrite an initialized node"
fi

source /etc/os-release
[[ "${ID:-}" == "alinux" && "${VERSION_ID:-}" == "4" ]] ||
  die "Alibaba Cloud Linux 4 is required"
[[ "$(uname -m)" == "x86_64" ]] || die "x86_64 architecture is required"
[[ "$(stat -fc %T /sys/fs/cgroup)" == "cgroup2fs" ]] ||
  die "unified cgroup v2 is required"

log_file="/root/cloudsentinel-node-bootstrap-${actual_hostname}-$(date +%Y%m%dT%H%M%S).log"
exec > >(tee -a "${log_file}") 2>&1

on_exit() {
  rc=$?
  trap - EXIT
  if [[ -n "${containerd_config_candidate:-}" && -e "${containerd_config_candidate}" ]]; then
    rm -f -- "${containerd_config_candidate}"
  fi
  if ((rc == 0)); then
    printf '\nBOOTSTRAP_RESULT=PASS\n'
  else
    printf '\nBOOTSTRAP_RESULT=FAIL exit_code=%s\n' "${rc}" >&2
  fi
  printf 'BOOTSTRAP_LOG=%s\n' "${log_file}"
  exit "${rc}"
}
trap on_exit EXIT
trap 'printf "ERROR: command failed at line %s: %s\n" "${LINENO}" "${BASH_COMMAND}" >&2' ERR

section "approved target"
printf 'node=%s\nprivate_ip=%s\n' "${actual_hostname}" "${expected_private_ip}"
printf 'script_revision=%s\n' "${SCRIPT_REVISION}"
printf 'kubernetes=%s\ncontainerd_rpm=%s\n' \
  "${KUBERNETES_VERSION}" "${CONTAINERD_RPM_VERSION}"
printf 'nlb=%s:6443\n' "${NLB_HOST}"
printf 'started_at=%s\n' "$(date --iso-8601=seconds)"

section "pre-change repository checks"
dnf --refresh list --showduplicates containerd.x86_64 |
  grep -F "${CONTAINERD_RPM_VERSION}"

dnf --refresh \
  --repofrompath=cloudsentinel-kubernetes-check,"${KUBERNETES_REPOSITORY}" \
  --repo=cloudsentinel-kubernetes-check \
  list --showduplicates kubeadm.x86_64 kubelet.x86_64 kubectl.x86_64 |
  grep -F "${KUBERNETES_RPM_VERSION}"

getent ahostsv4 "${NLB_HOST}"

section "swap and SELinux"
if [[ ! -e /etc/fstab.before-cloudsentinel-kubernetes ]]; then
  cp -a /etc/fstab /etc/fstab.before-cloudsentinel-kubernetes
fi
swapoff -a
sed -ri \
  '/^[[:space:]]*#/! { /[[:space:]]swap[[:space:]]/ s/^/# disabled-for-kubernetes / }' \
  /etc/fstab
[[ -z "$(swapon --noheadings --show)" ]] || die "swap remains enabled"

if command -v setenforce >/dev/null 2>&1; then
  setenforce 0 || true
fi
if [[ -f /etc/selinux/config ]]; then
  if [[ ! -e /etc/selinux/config.before-cloudsentinel-kubernetes ]]; then
    cp -a /etc/selinux/config /etc/selinux/config.before-cloudsentinel-kubernetes
  fi
  sed -ri 's/^[[:space:]]*SELINUX=.*/SELINUX=permissive/' /etc/selinux/config
fi

section "kernel modules and sysctls"
cat >/etc/modules-load.d/cloudsentinel-kubernetes.conf <<'EOF'
overlay
br_netfilter
vxlan
EOF

modprobe overlay
modprobe br_netfilter
modprobe vxlan

cat >/etc/sysctl.d/99-cloudsentinel-kubernetes.conf <<'EOF'
net.ipv4.ip_forward = 1
net.bridge.bridge-nf-call-iptables = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

sysctl --system

section "NetworkManager Calico interface boundary"
install -d -m 0755 /etc/NetworkManager/conf.d
cat >/etc/NetworkManager/conf.d/calico.conf <<'EOF'
[keyfile]
unmanaged-devices=interface-name:cali*;interface-name:tunl*;interface-name:vxlan.calico;interface-name:vxlan-v6.calico;interface-name:wireguard.cali;interface-name:wg-v6.cali
EOF
echo "NetworkManager was not restarted or reloaded by this script."

section "install containerd"
dnf install -y \
  "containerd-${CONTAINERD_RPM_VERSION}.x86_64" \
  conntrack-tools \
  socat \
  ipset \
  ethtool

installed_containerd="$(rpm -q --qf '%{VERSION}-%{RELEASE}' containerd)"
[[ "${installed_containerd}" == "${CONTAINERD_RPM_VERSION}" ]] ||
  die "unexpected containerd RPM: ${installed_containerd}"

install -d -m 0755 /etc/containerd
if [[ -s /etc/containerd/config.toml ]]; then
  if [[ ! -e /etc/containerd/config.toml.before-cloudsentinel-kubernetes ]]; then
    cp -a /etc/containerd/config.toml \
      /etc/containerd/config.toml.before-cloudsentinel-kubernetes
  fi
fi

containerd_config_candidate="$(mktemp /etc/containerd/config.toml.cloudsentinel.XXXXXX)"
containerd config default >"${containerd_config_candidate}"

if grep -Eq '^[[:space:]]*disabled_plugins[[:space:]]*=.*"cri"' \
  "${containerd_config_candidate}"; then
  die "containerd CRI plugin is disabled"
fi

systemd_cgroup_false_count="$(grep -Ec '^[[:space:]]*SystemdCgroup[[:space:]]*=[[:space:]]*false' "${containerd_config_candidate}" || true)"
systemd_cgroup_true_count="$(grep -Ec '^[[:space:]]*SystemdCgroup[[:space:]]*=[[:space:]]*true' "${containerd_config_candidate}" || true)"
if [[ "${systemd_cgroup_false_count}" -eq 1 ]]; then
  sed -ri \
    's/^([[:space:]]*SystemdCgroup[[:space:]]*=[[:space:]]*)false/\1true/' \
    "${containerd_config_candidate}"
elif [[ "${systemd_cgroup_true_count}" -ne 1 || "${systemd_cgroup_false_count}" -ne 0 ]]; then
  die "could not locate exactly one containerd SystemdCgroup setting"
fi

[[ "$(grep -Ec '^[[:space:]]*SystemdCgroup[[:space:]]*=[[:space:]]*true' "${containerd_config_candidate}")" -eq 1 ]] ||
  die "failed to enable SystemdCgroup in candidate config"

install -o root -g root -m 0644 \
  "${containerd_config_candidate}" /etc/containerd/config.toml
rm -f -- "${containerd_config_candidate}"

section "configure Kubernetes RPM repository"
cat >/etc/yum.repos.d/kubernetes.repo <<EOF
[kubernetes]
name=Kubernetes ${KUBERNETES_MINOR}
baseurl=${KUBERNETES_REPOSITORY}
enabled=1
gpgcheck=1
gpgkey=${KUBERNETES_REPOSITORY}repodata/repomd.xml.key
exclude=kubelet kubeadm kubectl cri-tools kubernetes-cni
EOF

dnf makecache --refresh

section "install Kubernetes packages"
dnf install -y \
  "kubeadm-${KUBERNETES_RPM_VERSION}.x86_64" \
  "kubelet-${KUBERNETES_RPM_VERSION}.x86_64" \
  "kubectl-${KUBERNETES_RPM_VERSION}.x86_64" \
  "cri-tools-${CRI_TOOLS_RPM_VERSION}.x86_64" \
  "containernetworking-plugins-${CNI_PLUGINS_RPM_VERSION}.x86_64" \
  --disableexcludes=kubernetes

for package in kubeadm kubelet kubectl; do
  installed_version="$(rpm -q --qf '%{VERSION}-%{RELEASE}' "${package}")"
  [[ "${installed_version}" == "${KUBERNETES_RPM_VERSION}" ]] ||
    die "unexpected ${package} RPM: ${installed_version}"
done

installed_cri_tools="$(rpm -q --qf '%{VERSION}-%{RELEASE}' cri-tools)"
[[ "${installed_cri_tools}" == "${CRI_TOOLS_RPM_VERSION}" ]] ||
  die "unexpected cri-tools RPM: ${installed_cri_tools}"

installed_cni_plugins="$(rpm -q --qf '%{VERSION}-%{RELEASE}' containernetworking-plugins)"
[[ "${installed_cni_plugins}" == "${CNI_PLUGINS_RPM_VERSION}" ]] ||
  die "unexpected containernetworking-plugins RPM: ${installed_cni_plugins}"

section "align containerd sandbox image"
pause_image="$(
  kubeadm config images list --kubernetes-version "${KUBERNETES_VERSION}" |
    awk '/pause/{print; exit}'
)"
[[ -n "${pause_image}" ]] || die "kubeadm did not report a pause image"
printf 'pause_image=%s\n' "${pause_image}"

sandbox_image_count="$(grep -Ec '^[[:space:]]*sandbox_image =' /etc/containerd/config.toml || true)"
[[ "${sandbox_image_count}" -eq 1 ]] ||
  die "could not locate exactly one containerd sandbox_image setting"
sed -ri \
  "s#^([[:space:]]*sandbox_image = ).*#\1\"${pause_image}\"#" \
  /etc/containerd/config.toml

cat >/etc/crictl.yaml <<'EOF'
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10
debug: false
EOF

section "enable services"
systemctl daemon-reload
systemctl enable containerd
systemctl restart containerd
systemctl enable kubelet

section "verification"
systemctl is-active containerd
systemctl is-enabled containerd
systemctl is-enabled kubelet
test -S /run/containerd/containerd.sock

containerd --version
kubeadm version -o short
kubelet --version
kubectl version --client=true
rpm -q containerd kubeadm kubelet kubectl cri-tools containernetworking-plugins

[[ "$(stat -fc %T /sys/fs/cgroup)" == "cgroup2fs" ]]
[[ "$(sysctl -n net.ipv4.ip_forward)" == "1" ]]
[[ "$(sysctl -n net.bridge.bridge-nf-call-iptables)" == "1" ]]
[[ "$(sysctl -n net.bridge.bridge-nf-call-ip6tables)" == "1" ]]
[[ "$(grep -Ec '^[[:space:]]*SystemdCgroup = true' /etc/containerd/config.toml)" -eq 1 ]]
grep -Fq "sandbox_image = \"${pause_image}\"" /etc/containerd/config.toml

crictl info >"/root/crictl-info-${actual_hostname}.json"
grep -E '"runtimeName"|"runtimeVersion"|"runtimeApiVersion"' \
  "/root/crictl-info-${actual_hostname}.json" || true

section "remaining gates"
printf 'firewalld_active=%s\n' "$(systemctl is-active firewalld 2>/dev/null || true)"
echo "This script did not run kubeadm init/join or modify cloud security groups."
echo "Review firewalld and node security-group rules before kubeadm/Calico deployment."
echo "NODE_BASELINE=PASS"
