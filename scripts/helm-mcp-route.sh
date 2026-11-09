#!/usr/bin/env bash
# Assert the rendered ingress routes the MCP endpoints directly to the API
# backend (:8080), ahead of the catch-all / -> BFF rule.
#
# Why this matters: the BFF strips client-supplied Authorization headers
# (token custodian — the session cookie is its only credential), so an MCP
# client sending "Authorization: Bearer <api key>" through the BFF reaches
# the backend with no key and gets 401 {"error":"missing api key"}. The MCP
# endpoints must bypass the BFF the same way the tracking prefixes do.
#
# Needs: helm (no cluster required — pure template render + assertions).
set -euo pipefail

CACHE="${TMPDIR:-/tmp}/opencode/helm-mcp-route"
mkdir -p "$CACHE"

render() { # $1=output, rest=extra helm flags
  local out="$1"; shift
  helm template automata helm/automata --namespace automata-chart-validate "$@" > "$out"
}

check() { # $1=file $2=variant
  local f="$1" v="$2"

  # 1. Both MCP paths are present as Exact matches (exact, not prefix —
  #    a prefix would also swallow /api/mcp-* admin-ish paths if any appear).
  #    "Exact" is the Gateway API enum value (there is no "ExactPath" — the
  #    cluster's CRD schema rejects it, which CI cannot catch without a
  #    server-side apply).
  grep -A1 'type: Exact$' "$f" | grep -q 'value: /api/mcp$' \
    || { echo "$v: FAIL — no Exact rule for /api/mcp" >&2; exit 1; }
  grep -A1 'type: Exact$' "$f" | grep -q 'value: /api/mcp-skill.md$' \
    || { echo "$v: FAIL — no Exact rule for /api/mcp-skill.md" >&2; exit 1; }

  # 1b. Every path match in the render uses a valid Gateway API enum value
  #     (Exact | PathPrefix | RegularExpression) — schema validation only
  #     happens server-side, so enforce the enum here.
  bad=$(awk '/^[[:space:]]*-?[[:space:]]*path:[[:space:]]*$/{p=1} p && /type:/{print $2; p=0}' "$f" \
    | grep -vE '^(Exact|PathPrefix|RegularExpression)$' | sort -u || true)
  [ -z "$bad" ] \
    || { echo "$v: FAIL — invalid path.match type(s): $bad" >&2; exit 1; }

  # 2. The /api/mcp rule targets the API backend port 8080 (not 8081
  #    tracking, not 8082 BFF).
  awk '
    /^ *value: \/api\/mcp$/ { found = 1; next }
    found && /port: 8080/   { ok = 1 }
    found && /^ *value: /   { exit }
    END { exit !ok }' "$f" \
    || { echo "$v: FAIL — /api/mcp rule does not target port 8080" >&2; exit 1; }

  # 3. The MCP rules appear before the catch-all "value: /" (BFF) rule.
  local mcp_line ca_line
  mcp_line=$(grep -n '^ *value: /api/mcp$' "$f" | head -1 | cut -d: -f1)
  ca_line=$(grep -n '^ *value: /$' "$f" | head -1 | cut -d: -f1)
  [ -n "${mcp_line:-}" ] && [ -n "${ca_line:-}" ] \
    || { echo "$v: FAIL — could not locate MCP/catch-all rules" >&2; exit 1; }
  [ "$mcp_line" -lt "$ca_line" ] \
    || { echo "$v: FAIL — /api/mcp rule is not before the catch-all / rule" >&2; exit 1; }

  echo "helm-mcp-route: $v OK (MCP → API :8080, ahead of BFF catch-all)"
}

NOTLS="$CACHE/render-notls.yaml"
TLS="$CACHE/render-tls.yaml"
render "$NOTLS"
render "$TLS" \
  --set 'ingress.tls[0].hosts=m.example.com' \
  --set 'ingress.tls[0].secretName=automata-tls' \
  --set ingress.certificate.enabled=true \
  --set ingress.certificate.clusterIssuer=letsencrypt-prod

check "$NOTLS" "no-TLS"
check "$TLS" "TLS"