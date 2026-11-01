#!/usr/bin/env bash
# Diagnostics: why www.h2hsecure.com/snippet/* 500s.
# The Nuxt prod site (ns websites) routes /snippet via an ExternalName shim to
# automata-automata.automata.svc.cluster.local:8081 (ns automata). A no-body 500
# means Traefik cannot reach that backend. This script isolates the cause.
#
# Run in a shell whose kubectl context points at the PRODUCTION cluster:
#   bash scripts/diag-automata-snippet-shim.sh
#
# Or run each section by hand (see the A/B/C/D blocks below) and paste outputs.
set -u

SNIPPET_UUID=422adc7a-5b07-49a5-bd00-c6a44d61a172
FQDN=automata-automata.automata.svc.cluster.local
POD="nettest-$(date +%s)"

echo "===== A. Is automata in THIS cluster? ====="
kubectl get ns automata
kubectl -n automata get svc automata-automata
kubectl -n automata get endpoints automata-automata

echo
echo "===== B. Resolve + reach from inside the websites ns (Traefik's path) ====="
kubectl -n websites run "$POD" --image=busybox --restart=Never -- sleep 300 \
  && kubectl -n websites wait "pod/$POD" --for=condition=Ready --timeout=30s \
  && kubectl -n websites exec "$POD" -- sh -c "
       echo '--- nslookup $FQDN ---';
       nslookup $FQDN;
       echo '--- wget http://$FQDN:8081/snippet/<uuid>.js (first 300 bytes) ---';
       wget -qO- --timeout=5 http://$FQDN:8081/snippet/$SNIPPET_UUID.js | head -c 300; echo
     " || echo "(nettest pod failed to start / exec)"
kubectl -n websites delete pod "$POD" --ignore-not-found --force --grace-period=0 >/dev/null 2>&1

echo
echo "===== C. Traefik logs (failure lines) ====="
kubectl -n kube-system logs deploy/traefik --tail=300 \
  | grep -iE "500|snippet|automata|upstream|refused|no server|dial" \
  || echo "(no matching traefik log lines)"

echo
echo "===== D. NetworkPolicy (relevant namespaces) ====="
kubectl get networkpolicy -A \
  | grep -iE "automata|websites|kube-system|NAMESPACE" \
  || echo "(no matching network policies)"