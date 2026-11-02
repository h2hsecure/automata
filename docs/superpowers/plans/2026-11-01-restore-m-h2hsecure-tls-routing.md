# Restore m.h2hsecure.com TLS + routing (post-nginx-migration) — DONE

## Context (corrected topology)

`m.h2hsecure.com` was dead since 2026-10-30. Root cause (confirmed): the
edge migration moved Traefik's entrypoints off 80/443. The working website
Gateways proved the new scheme:

- HTTP → port **8000**
- HTTPS → port **8443** (TLS terminate, e.g. secret `h2hsecure-tls`)

The automata chart hardcoded Gateway listeners on 443/80 → `PortUnavailable`
→ Gateway `PROGRAMMED` empty → HTTPRoutes `NotAllowedByListeners`.

**Corrected edge model (supersedes the original plan's Task 3):** there is
**no VM-level edge nginx**. Traefik is the edge: the LB service
(`152.53.254.200`) maps external 80/443 → Traefik entrypoints 8000/8443
(documented in the website's `DEPLOYMENT.md`). The "nginx reverse proxy in
the project" is the website's **in-pod nginx front**
(`h2hsecure.github.io/charts/h2hsecure/templates/tracking.yaml`), which
proxies `/snippet|track|form|banner` → `automata-automata.automata.svc:8081`
via lazy DNS and bypasses the Gateway entirely. It was never affected.

The m.* cert error (CN=`*.pdaccess.com`) = Traefik serving its default
certificate because no Programmed listener matched the SNI.

## What shipped

### 1. Chart: configurable Gateway listener ports — `1f3e2e0` ✅

`helm/automata/values.yaml` + `templates/ingress.yaml`: `ingress.httpPort`
(default 80) / `ingress.httpsPort` (default 443); prod `values.prod.yaml`
sets **8000/8443**. `deploy-prod.md` documents them as REQUIRED on the prod
cluster.

### 2. Prod re-upgrade — chart `2026.11.02-1f3e2e0`, image `b2746ec` ✅

User ran the helm upgrade on the cluster machine. Verified in-cluster:
Gateway `PROGRAMMED=True`, both HTTPRoutes attached. External probes:

- SNI `m.h2hsecure.com` → **CN=m.h2hsecure.com** (Let's Encrypt, valid to
  Jan 21 2027) — was the `*.pdaccess.com` default ✅
- `GET /api/health` → **200** ✅
- `GET /snippet/422adc7a-….js` → **200** (JS) ✅
- `HEAD /snippet/….js` → **200** — the `b2746ec` tracking fix confirmed live ✅
- `HEAD /api/health` → **404** ❌ (latent bug, see §4)
- `http://m.h2hsecure.com/` → **200, no 301** ❌ (latent bug, see §4)

### 3. Task 3 (edge nginx server block) — VOID ✅

Voided: no edge nginx exists (see corrected topology). Nothing to do.

### 4. Latent bugs exposed by the restored routing — `670e91e`

Both predate the port fix; the restored routing simply made them visible.

**a. HTTP→HTTPS 301 silently lost (Gateway route tiebreak).**
`automata-route` and `automata-route-http` both referenced the Gateway
without `sectionName`, so both bound to both listeners. On the HTTP
listener they tied on host+path; Gateway API has no route priority, and
Traefik's deterministic router tiebreak (shorter router name sorts first)
let the main route win — plain HTTP served content. Fix: scope each route
to its listener via `parentRefs[].sectionName` (main → `https`,
redirect → `http`), making the redirect the sole route on the HTTP port.

**b. HEAD 404 on the non-tracking health routes (gin ≥ 1.12).**
`b2746ec` fixed the tracking server only; the same HEAD→GET gap existed on
`api` (`/api/health`), `cmd/bff` (`/health`), `cmd/automata` (admin
`/health`), and the standalone `webui` server. Fix: explicit `HEAD`
registration at all four sites + route-registration test in
`internal/adapter/api/server_test.go`.

### 5. TLS renewal — no action needed ✅

`letsencrypt-prod` ClusterIssuer solver is `http01` **ingress** class
`traefik` (not a gatewayHTTPRoute solver). cert-manager creates a
short-lived challenge Ingress; the existing `automata-tls` cert was issued
via this exact path, proving LB 80 → entrypoint 8000 → Traefik ingress
route → challenge pod works. No parentRef gap exists for the second
Gateway, and our HTTPRoutes don't claim `/.well-known/acme-challenge/*`.
Renewal window opens ~mid-Dec (cert expires Jan 21 2027).

## Final verification (after `670e91e` deploy) — run 2026-11-02, all green

```bash
curl -sI http://m.h2hsecure.com/                            # 301 -> https
curl -s -o /dev/null -w '%{http_code}\n' -X HEAD https://m.h2hsecure.com/api/health   # 200
curl -s -o /dev/null -w '%{http_code}\n' -X HEAD https://m.h2hsecure.com/health       # 200
curl -sI https://m.h2hsecure.com/api/health                 # 200 (regression)
curl -sI https://m.h2hsecure.com/snippet/422adc7a-5b07-49a5-bd00-c6a44d61a172.js      # 200
```

Results: 301 → `https://m.h2hsecure.com/`; `HEAD /api/health` 200; `HEAD /health` 200;
regressions all 200 (including `HEAD /snippet` 200); SNI `CN=m.h2hsecure.com`
(LE, valid to 2027-01-21); `www.h2hsecure.com` in-pod nginx snippet 200
(`server: nginx/1.31.8`) — website untouched.

## Risks / notes

- `670e91e` upgrade: re-renders 2 HTTPRoutes (no churn) + pod rollout on
  the new image. Rollback = previous chart version + `--set image.tag=b2746ec`.
- Pre-existing unrelated failures (`TestContextMiddleware` in
  `internal/adapter/api`, build failure in `internal/adapter/proxy`)
  predate this work and were deliberately left untouched.