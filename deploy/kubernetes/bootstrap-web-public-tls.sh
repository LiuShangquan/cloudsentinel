#!/usr/bin/env bash
set -Eeuo pipefail

readonly SOURCE_NAMESPACE="cloudsentinel-secret-source"
readonly SOURCE_SECRET="cloudsentinel-web-public-tls"
readonly PUBLIC_IP="${CLOUDSENTINEL_WEB_PUBLIC_IP:-}"
readonly NODE_PORT="${CLOUDSENTINEL_WEB_PUBLIC_NODE_PORT:-30443}"
readonly CERT_DAYS="${CLOUDSENTINEL_WEB_PUBLIC_CERT_DAYS:-30}"
readonly REQUEST_TIMEOUT="${KUBECTL_REQUEST_TIMEOUT:-20s}"
readonly API_SERVER="${KUBERNETES_API_SERVER:-}"

temporary_directory=""

cleanup() {
  if [[ -n "$temporary_directory" && -d "$temporary_directory" ]]; then
    rm -f \
      "$temporary_directory/tls.crt" \
      "$temporary_directory/tls.key" \
      "$temporary_directory/public-url"
    rmdir "$temporary_directory"
  fi
}

trap cleanup EXIT

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

validate_ipv4() {
  local address="$1"
  local octet
  local -a octets

  [[ "$address" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || return 1
  IFS='.' read -r -a octets <<< "$address"
  for octet in "${octets[@]}"; do
    (( 10#$octet <= 255 )) || return 1
  done
}

[[ -n "$PUBLIC_IP" ]] ||
  fail 'set CLOUDSENTINEL_WEB_PUBLIC_IP to the selected application worker public IPv4 address'
validate_ipv4 "$PUBLIC_IP" || fail 'CLOUDSENTINEL_WEB_PUBLIC_IP is not a valid IPv4 address'
[[ "$NODE_PORT" == "30443" ]] ||
  fail 'this GitOps revision fixes CLOUDSENTINEL_WEB_PUBLIC_NODE_PORT at 30443'
[[ "$CERT_DAYS" =~ ^[0-9]+$ ]] ||
  fail 'CLOUDSENTINEL_WEB_PUBLIC_CERT_DAYS must be an integer'
(( CERT_DAYS >= 1 && CERT_DAYS <= 397 )) ||
  fail 'CLOUDSENTINEL_WEB_PUBLIC_CERT_DAYS must be between 1 and 397'

command -v kubectl >/dev/null 2>&1 || fail 'kubectl is required'
command -v openssl >/dev/null 2>&1 || fail 'openssl is required'

kube() {
  local -a arguments=(--request-timeout="$REQUEST_TIMEOUT")
  if [[ -n "$API_SERVER" ]]; then
    arguments+=(--server="$API_SERVER")
  fi
  kubectl "${arguments[@]}" "$@"
}

echo '===== preflight ====='
kube get namespace "$SOURCE_NAMESPACE" >/dev/null
kube get clustersecretstore cloudsentinel-secret-store >/dev/null

umask 077
temporary_directory="$(mktemp -d /root/cloudsentinel-web-public-tls.XXXXXX)"

echo '===== generate self-signed TLS certificate ====='
openssl req \
  -x509 \
  -newkey rsa:3072 \
  -nodes \
  -sha256 \
  -days "$CERT_DAYS" \
  -subj "/CN=$PUBLIC_IP" \
  -addext "subjectAltName=IP:$PUBLIC_IP" \
  -keyout "$temporary_directory/tls.key" \
  -out "$temporary_directory/tls.crt" \
  >/dev/null 2>&1

openssl x509 \
  -in "$temporary_directory/tls.crt" \
  -noout \
  -checkend 86400 \
  >/dev/null || fail 'generated certificate expires too soon'

openssl x509 \
  -in "$temporary_directory/tls.crt" \
  -noout \
  -ext subjectAltName |
  grep -F "IP Address:$PUBLIC_IP" >/dev/null ||
  fail 'certificate IP SAN verification failed'

public_url="https://${PUBLIC_IP}:${NODE_PORT}/"
printf '%s' "$public_url" > "$temporary_directory/public-url"
[[ "$(wc -c < "$temporary_directory/public-url")" -eq "${#public_url}" ]] ||
  fail 'public URL file contains unexpected trailing bytes'

echo '===== create protected source Secret ====='
kube \
  -n "$SOURCE_NAMESPACE" \
  create secret generic "$SOURCE_SECRET" \
  --from-file=tls.crt="$temporary_directory/tls.crt" \
  --from-file=tls.key="$temporary_directory/tls.key" \
  --from-file=public-url="$temporary_directory/public-url" \
  --dry-run=client \
  -o yaml |
  kube \
    apply \
    --server-side \
    --field-manager=cloudsentinel-web-public-bootstrap \
    -f -

echo '===== metadata-only verification ====='
kube \
  -n "$SOURCE_NAMESPACE" \
  get secret "$SOURCE_SECRET" \
  -o go-template='name={{.metadata.name}} type={{.type}} key_count={{len .data}}{{"\n"}}'

echo '===== certificate identity ====='
openssl x509 \
  -in "$temporary_directory/tls.crt" \
  -noout \
  -subject \
  -dates \
  -fingerprint \
  -sha256
echo "public_url=$public_url"
echo 'CLOUDSENTINEL_WEB_PUBLIC_TLS_BOOTSTRAP=PASS'
