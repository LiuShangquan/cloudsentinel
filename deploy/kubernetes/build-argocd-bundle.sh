#!/usr/bin/env bash

# Build an auditable, offline Argo CD installation bundle from the pinned
# upstream manifest and immutable ACR digests. This script never reads or
# writes registry credentials, kubeconfig files, or application Secrets.

set -Eeuo pipefail

readonly ARGOCD_VERSION="v3.5.0"
readonly DEX_VERSION="v2.45.0"
readonly REDIS_VERSION="8.2.3-alpine"
readonly UPSTREAM_MANIFEST_SHA256="a32bf36a437071a1f563ebf9e81c8a39fba9057c17db7d5d041afb7b6e3f4afe"
readonly ACR_DEPLOY_REGISTRY="crpi-1s64ln3ptbvgkqof-vpc.cn-beijing.personal.cr.aliyuncs.com"
readonly ACR_NAMESPACE="cloudsentinel0306"
readonly SCRIPT_REVISION="2026-08-15.2"

upstream_manifest=""
output_dir=""
argocd_digest=""
dex_digest=""
redis_digest=""

usage() {
	cat <<'EOF'
Usage:
  bash build-argocd-bundle.sh \
    --upstream-manifest /absolute/path/install.yaml \
    --output-dir /absolute/path/argocd-bundle \
    --argocd-digest sha256:64-lowercase-hex-characters \
    --dex-digest sha256:64-lowercase-hex-characters \
    --redis-digest sha256:64-lowercase-hex-characters

The upstream manifest must be the official Argo CD v3.5.0 non-HA
manifests/install.yaml with the checksum pinned in this script.
EOF
}

die() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

while (($# > 0)); do
	case "$1" in
	--upstream-manifest)
		(($# >= 2)) || die "--upstream-manifest requires a value"
		upstream_manifest="$2"
		shift 2
		;;
	--output-dir)
		(($# >= 2)) || die "--output-dir requires a value"
		output_dir="$2"
		shift 2
		;;
	--argocd-digest)
		(($# >= 2)) || die "--argocd-digest requires a value"
		argocd_digest="$2"
		shift 2
		;;
	--dex-digest)
		(($# >= 2)) || die "--dex-digest requires a value"
		dex_digest="$2"
		shift 2
		;;
	--redis-digest)
		(($# >= 2)) || die "--redis-digest requires a value"
		redis_digest="$2"
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

command -v kubectl >/dev/null 2>&1 || die "kubectl is required"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required"
[[ -n "${upstream_manifest}" && -f "${upstream_manifest}" ]] ||
	die "--upstream-manifest must reference a regular file"
[[ -n "${output_dir}" ]] || die "--output-dir is required"
[[ ! -e "${output_dir}" ]] || die "output path already exists: ${output_dir}"

for digest_name in argocd_digest dex_digest redis_digest; do
	digest_value="${!digest_name}"
	[[ "${digest_value}" =~ ^sha256:[0-9a-f]{64}$ ]] ||
		die "${digest_name} is not an immutable sha256 digest"
done

actual_sha256="$(sha256sum "${upstream_manifest}" | awk '{print $1}')"
[[ "${actual_sha256}" == "${UPSTREAM_MANIFEST_SHA256}" ]] ||
	die "official Argo CD manifest checksum does not match the approved release"

readonly source_argocd="quay.io/argoproj/argocd:${ARGOCD_VERSION}"
readonly source_dex="ghcr.io/dexidp/dex:${DEX_VERSION}"
readonly source_redis="public.ecr.aws/docker/library/redis:${REDIS_VERSION}"
readonly deploy_argocd="${ACR_DEPLOY_REGISTRY}/${ACR_NAMESPACE}/cloudsentinel-argocd"
readonly deploy_dex="${ACR_DEPLOY_REGISTRY}/${ACR_NAMESPACE}/cloudsentinel-argocd-dex"
readonly deploy_redis="${ACR_DEPLOY_REGISTRY}/${ACR_NAMESPACE}/cloudsentinel-argocd-redis"

[[ "$(grep -Fc "image: ${source_argocd}" "${upstream_manifest}")" -eq 8 ]] ||
	die "unexpected Argo CD image reference count in upstream manifest"
[[ "$(grep -Fc "image: ${source_dex}" "${upstream_manifest}")" -eq 1 ]] ||
	die "unexpected Dex image reference count in upstream manifest"
[[ "$(grep -Fc "image: ${source_redis}" "${upstream_manifest}")" -eq 1 ]] ||
	die "unexpected Redis image reference count in upstream manifest"
[[ "$(grep -c '^kind: Deployment$' "${upstream_manifest}")" -eq 6 ]] ||
	die "unexpected Deployment count in upstream manifest"
[[ "$(grep -c '^kind: StatefulSet$' "${upstream_manifest}")" -eq 1 ]] ||
	die "unexpected StatefulSet count in upstream manifest"

mkdir -p "${output_dir}"
cp -- "${upstream_manifest}" "${output_dir}/argocd.upstream.yaml"

cat >"${output_dir}/namespace.yaml" <<'EOF'
apiVersion: v1
kind: Namespace
metadata:
  name: argocd
EOF

cat >"${output_dir}/registry-external-secret.yaml" <<'EOF'
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: platform-acr-registry
  namespace: argocd
spec:
  refreshInterval: 1h
  secretStoreRef:
    kind: ClusterSecretStore
    name: cloudsentinel-secret-store
  target:
    name: platform-acr-registry
    creationPolicy: Owner
    deletionPolicy: Retain
    template:
      type: kubernetes.io/dockerconfigjson
      engineVersion: v2
      data:
        .dockerconfigjson: "{{ .dockerconfigjson }}"
  data:
    - secretKey: dockerconfigjson
      remoteRef:
        key: cloudsentinel-registry
        property: .dockerconfigjson
EOF

cat >"${output_dir}/kustomization.yaml" <<EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: argocd
resources:
  - namespace.yaml
  - registry-external-secret.yaml
  - argocd.upstream.yaml
images:
  - name: quay.io/argoproj/argocd
    newName: ${deploy_argocd}
    digest: ${argocd_digest}
  - name: ghcr.io/dexidp/dex
    newName: ${deploy_dex}
    digest: ${dex_digest}
  - name: public.ecr.aws/docker/library/redis
    newName: ${deploy_redis}
    digest: ${redis_digest}
patches:
  - target:
      group: apps
      version: v1
      kind: Deployment
    patch: |-
      - op: add
        path: /spec/template/spec/imagePullSecrets
        value:
          - name: platform-acr-registry
      - op: add
        path: /spec/template/spec/nodeSelector/node-role
        value: app
      - op: add
        path: /spec/template/spec/containers/0/resources
        value:
          requests:
            cpu: 25m
            memory: 96Mi
          limits:
            cpu: 500m
            memory: 512Mi
  - target:
      group: apps
      version: v1
      kind: StatefulSet
    patch: |-
      - op: add
        path: /spec/template/spec/imagePullSecrets
        value:
          - name: platform-acr-registry
      - op: add
        path: /spec/template/spec/nodeSelector/node-role
        value: app
      - op: add
        path: /spec/template/spec/containers/0/resources
        value:
          requests:
            cpu: 25m
            memory: 96Mi
          limits:
            cpu: 500m
            memory: 512Mi
  - target:
      version: v1
      kind: ConfigMap
      name: argocd-cmd-params-cm
    patch: |-
      - op: add
        path: /data/controller.status.processors
        value: "1"
      - op: add
        path: /data/controller.operation.processors
        value: "1"
      - op: add
        path: /data/controller.kubectl.parallelism.limit
        value: "1"
      - op: add
        path: /data/reposerver.parallelism.limit
        value: "1"
EOF

readonly release_url="https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_VERSION}/manifests/install.yaml"
cat >"${output_dir}/metadata.env" <<EOF
ARGOCD_VERSION=${ARGOCD_VERSION}
DEX_VERSION=${DEX_VERSION}
REDIS_VERSION=${REDIS_VERSION}
UPSTREAM_MANIFEST_URL=${release_url}
UPSTREAM_MANIFEST_SHA256=${UPSTREAM_MANIFEST_SHA256}
SOURCE_ARGOCD_IMAGE=${source_argocd}
SOURCE_DEX_IMAGE=${source_dex}
SOURCE_REDIS_IMAGE=${source_redis}
DEPLOY_ARGOCD_IMAGE=${deploy_argocd}@${argocd_digest}
DEPLOY_DEX_IMAGE=${deploy_dex}@${dex_digest}
DEPLOY_REDIS_IMAGE=${deploy_redis}@${redis_digest}
PLATFORM=linux/amd64
BUNDLE_BUILDER_REVISION=${SCRIPT_REVISION}
EOF

kubectl kustomize "${output_dir}" >"${output_dir}/argocd.rendered.yaml"

rendered="${output_dir}/argocd.rendered.yaml"
[[ "$(grep -Fc "image: ${deploy_argocd}@${argocd_digest}" "${rendered}")" -eq 8 ]] ||
	die "rendered bundle has an unexpected Argo CD image count"
[[ "$(grep -Fc "image: ${deploy_dex}@${dex_digest}" "${rendered}")" -eq 1 ]] ||
	die "rendered bundle has an unexpected Dex image count"
[[ "$(grep -Fc "image: ${deploy_redis}@${redis_digest}" "${rendered}")" -eq 1 ]] ||
	die "rendered bundle has an unexpected Redis image count"
[[ "$(grep -c 'name: platform-acr-registry' "${rendered}")" -eq 9 ]] ||
	die "all workloads and both ExternalSecret name fields must use the registry name"
[[ "$(grep -c 'node-role: app' "${rendered}")" -eq 7 ]] ||
	die "all seven Argo CD workloads must target app nodes"
for expected_parameter in \
	'controller.status.processors: "1"' \
	'controller.operation.processors: "1"' \
	'controller.kubectl.parallelism.limit: "1"' \
	'reposerver.parallelism.limit: "1"'; do
	grep -Fq "${expected_parameter}" "${rendered}" ||
		die "rendered bundle is missing constrained parameter: ${expected_parameter}"
done
[[ "$(grep -c '^kind: ExternalSecret$' "${rendered}")" -eq 1 ]] ||
	die "registry ExternalSecret is missing from rendered bundle"
if grep -Eq '^[[:space:]]*image:[[:space:]]+(quay\.io|ghcr\.io|public\.ecr\.aws|docker\.io)/' \
	"${rendered}"; then
	die "rendered bundle still contains a public registry image"
fi

(
	cd "${output_dir}"
	sha256sum \
		argocd.upstream.yaml \
		namespace.yaml \
		registry-external-secret.yaml \
		kustomization.yaml \
		metadata.env \
		argocd.rendered.yaml \
		>checksums.sha256
)

printf 'ARGOCD_BUNDLE_BUILD=PASS output_dir=%s\n' "${output_dir}"
