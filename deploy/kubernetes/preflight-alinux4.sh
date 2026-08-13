#!/usr/bin/env bash

# Read-only inventory for the CloudSentinel Alibaba Cloud Linux 4 lab.
# This script does not install packages, load modules, change sysctls,
# edit files, restart services or expose credentials.

set -u

section() {
  printf '\n===== %s =====\n' "$1"
}

run_readonly() {
  printf '+ '
  printf '%q ' "$@"
  printf '\n'
  "$@" 2>&1 || true
}

section "identity"
run_readonly date --iso-8601=seconds
run_readonly hostname -s
run_readonly hostnamectl status
run_readonly cat /etc/os-release
run_readonly uname -a
run_readonly uname -m
run_readonly rpm -E '%{rhel}'

section "capacity"
run_readonly nproc
run_readonly free -m
run_readonly df -hT / /var /var/lib
run_readonly df -i / /var /var/lib
run_readonly lsblk -o NAME,SIZE,TYPE,FSTYPE,MOUNTPOINTS

section "kernel and cgroup"
kernel_numeric="$(uname -r | cut -d- -f1)"
kernel_major="${kernel_numeric%%.*}"
kernel_rest="${kernel_numeric#*.}"
kernel_minor="${kernel_rest%%.*}"
if [[ "${kernel_major}" =~ ^[0-9]+$ && "${kernel_minor}" =~ ^[0-9]+$ ]] &&
  ((kernel_major > 5 || (kernel_major == 5 && kernel_minor >= 10))); then
  echo "calico_kernel_gate=PASS (${kernel_numeric} >= 5.10)"
else
  echo "calico_kernel_gate=FAIL (${kernel_numeric} is below 5.10)"
fi
run_readonly stat -fc '%T' /sys/fs/cgroup
run_readonly findmnt -rn -o TARGET,FSTYPE,OPTIONS /sys/fs/cgroup
run_readonly swapon --show
run_readonly getenforce

section "required module availability"
for module in overlay br_netfilter vxlan ip_tables iptable_nat nf_conntrack; do
  if modinfo "${module}" >/dev/null 2>&1; then
    echo "module_available=${module}"
  else
    echo "module_missing=${module}"
  fi
done

section "network sysctls (read only)"
run_readonly sysctl net.ipv4.ip_forward
run_readonly sysctl net.bridge.bridge-nf-call-iptables
run_readonly sysctl net.bridge.bridge-nf-call-ip6tables

section "network and time"
run_readonly ip -4 -br address
run_readonly ip route
run_readonly cat /etc/resolv.conf
run_readonly timedatectl status
run_readonly chronyc tracking
run_readonly systemctl is-active chronyd
run_readonly systemctl is-active firewalld
run_readonly systemctl is-active NetworkManager

section "installed runtime packages"
run_readonly rpm -q chrony iptables ipset conntrack-tools socat ebtables ethtool containerd containerd.io kubelet kubeadm kubectl
run_readonly iptables --version

section "private TCP 22 reachability"
for target in \
  172.29.253.154 \
  172.29.253.155 \
  172.29.253.152 \
  172.16.41.182 \
  172.16.41.183 \
  172.29.253.150 \
  172.29.253.151 \
  172.29.253.153; do
  if timeout 3 bash -c ">/dev/tcp/${target}/22" 2>/dev/null; then
    echo "tcp22=${target}:PASS"
  else
    echo "tcp22=${target}:FAIL"
  fi
done

section "registry reachability"
registry="crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com"
run_readonly getent hosts "${registry}"
if command -v curl >/dev/null 2>&1; then
  http_code="$(curl --connect-timeout 8 --max-time 15 -sS -o /dev/null -w '%{http_code}' "https://${registry}/v2/" || true)"
  echo "acr_vpc_registry_http_code=${http_code} (200 or 401 proves HTTPS reachability)"
else
  echo "acr_vpc_registry_http_code=NOT_CHECKED (curl missing)"
fi

section "result boundary"
echo "This report is diagnostic evidence only. It does not install or approve Kubernetes components."
