#!/usr/bin/env bash
# Server-side validation of the rendered Helm chart against real CRD schemas.
#
# `helm lint`, `helm template` and `kubectl apply --dry-run=client` never
# consult CRD schemas — an invented CRD field or filter type sails through
# all of them and only fails at the production API server. This script
# renders both chart variants and server-side dry-runs them against a
# cluster that has the real CRDs installed (local k3s is fine).
#
# CRD provenance: the gateway-api and traefik.io CRDs come from the
# traefik-crds chart (1.18.0) — the same lineage the prod `traefik-crd`
# helm release owns, so validation matches what the cluster will have.
#
# Requires: kubectl with a reachable cluster, network access for the CRD
# downloads (cached in $TMPDIR/opencode/helm-validate).
set -euo pipefail

NS="automata-chart-validate"
CACHE="${TMPDIR:-/tmp}/opencode/helm-validate"
mkdir -p "$CACHE/crds"

# --- CRDs (cached) -----------------------------------------------------------
[ -d "$CACHE/crds/traefik-crds/crds-files" ] || {
  T="$CACHE/crds/traefik-crds-1.18.0.tgz"
  [ -s "$T" ] || curl -sfL "https://traefik.github.io/charts/traefik-crds/traefik-crds-1.18.0.tgz" -o "$T"
  tar xzf "$T" -C "$CACHE/crds"
}

CM="$CACHE/crds/cert-manager-crds"
[ -d "$CM" ] || {
  C="$CACHE/crds/cert-manager.crds.yaml"
  [ -s "$C" ] || curl -sfL "https://github.com/cert-manager/cert-manager/releases/download/v1.21.3/cert-manager.crds.yaml" -o "$C"
  mkdir -p "$CM"
  cp "$C" "$CM/cert-manager.crds.yaml"
}

# --- render both variants ----------------------------------------------------
NOTLS="$CACHE/render-notls.yaml"
TLS="$CACHE/render-tls.yaml"
helm template automata helm/automata --namespace "$NS" > "$NOTLS"
helm template automata helm/automata --namespace "$NS" \
  --set 'ingress.tls[0].hosts=m.example.com' \
  --set 'ingress.tls[0].secretName=automata-tls' \
  --set ingress.certificate.enabled=true \
  --set ingress.certificate.clusterIssuer=letsencrypt-prod \
  > "$TLS"

# --- idempotent CRD install, then server-side dry-run -------------------------
# Server-side dry-run rejects namespaced objects whose namespace does not
# exist, so make sure the scratch namespace is there.
kubectl get namespace "$NS" >/dev/null 2>&1 || kubectl create namespace "$NS" >/dev/null
kubectl apply -f "$CACHE/crds/traefik-crds/crds-files/gatewayAPI/gateway-standard-install.yaml" >/dev/null 2>&1
kubectl apply -f "$CACHE/crds/traefik-crds/crds-files/traefik/" >/dev/null 2>&1
kubectl apply -f "$CM/" >/dev/null 2>&1

kubectl apply --dry-run=server -n "$NS" -f "$NOTLS" >/dev/null 2>&1
echo "helm-validate: no-TLS variant valid against CRD schemas"
kubectl apply --dry-run=server -n "$NS" -f "$TLS" >/dev/null 2>&1
echo "helm-validate: TLS variant valid against CRD schemas"