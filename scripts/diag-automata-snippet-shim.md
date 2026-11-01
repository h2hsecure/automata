# Production troubleshooting: `www.h2hsecure.com/snippet/*` returns 500

Companion to `scripts/diag-automata-snippet-shim.sh`. Run that script in a shell
whose `kubectl` context points at the **production** cluster, then work the
decision tree below.

## Symptom

```
curl -I https://www.h2hsecure.com/snippet/422adc7a-5b07-49a5-bd00-c6a44d61a172.js
HTTP/2 500
content-length: 0
```

A bare **500 with an empty body** (no `content-type`, no `server`, no JSON
error). This is **Traefik's upstream failure signature**, not a gin/app response
— the automata snippet handler always returns a body (JSON on error, JS on
success). The Nuxt site's own 404s, by contrast, carry full security headers and
a JSON body (`content-security-policy`, `x-frame-options: DENY`, …), confirming
`www` is the Nuxt app, and the 500 is generated at the routing layer.

## The route chain

`www.h2hsecure.com` is the **Nuxt marketing site** (repo `h2hsecure.github.io`),
deployed to namespace `websites`, Helm release `h2hsecure-prod`. The tracking
snippet is injected server-side into the SSR `<head>` as
`<script src="/snippet/<uuid>.js">` (same origin). That request is routed
cross-namespace to the automata tracking backend:

```
www.h2hsecure.com/snippet/…
  → Gateway  h2hsecure-prod-h2hsecure            (ns websites)
  → HTTPRule /snippet,/track,/form,/banner       (charts/h2hsecure/templates/httproute.yaml)
  → Service  h2hsecure-prod-automata-tracking    (ns websites, type ExternalName)
  → automata-automata.automata.svc.cluster.local:8081   (ns automata)
```

The ExternalName bridge exists because Gateway `backendRefs` can only target
Services in the **same** namespace; `automata-automata.automata.svc.cluster.local`
is a stable DNS name across automata redeploys (the ClusterIP is not).

**Hard constraint — the snippet must be same-origin.** The `<script>` is served
from `www.h2hsecure.com` itself (not from `m.h2hsecure.com`) on purpose: a
cross-origin tracking script is a well-known adblocker/privacy-blocker trigger,
so serving it from the page's own origin is what lets it load. This is *why* the
ExternalName shim exists at all. Any fix must preserve same-origin serving; the
cross-origin option is **not acceptable**.

Reference values:
- Snippet UUID (chart `tracking.snippetUuid`): `422adc7a-5b07-49a5-bd00-c6a44d61a172`
- Tracking port: `8081` (chart `tracking.servicePort`; automata Service port name `tracking`)
- `m.h2hsecure.com/snippet/…` → **200** (the same tracking server, reached via the
  automata chart's own Gateway) — proves the backend on `:8081` is healthy.

## Root cause (CONFIRMED)

**Traefik rejects `ExternalName` Services as backendRefs.** The `kubernetesgateway`
provider logs, for the very route in question:

```
h2hsecure-prod-automata-tracking: type ExternalName is not supported for
  Kubernetes Service reference   (providerName=kubernetesgateway)
```

The CRD provider logs the same class of failure for the sibling `pdaccess` site:

```
externalName services not allowed: websites/pdaccess-prod-automata-tracking
  (providerName=kubernetescrd)
```

So the `/snippet` HTTPRule has no usable backend → Traefik returns a bare 500.
The shim Service, DNS, NetworkPolicy, and backend health are all fine (A passed:
ready endpoint `10.42.0.177:8081`; `m.h2hsecure.com/snippet` → 200) — the ExternalName
type itself is unsupported by this Traefik Gateway-API provider. This is why the
candidate list collapsed: it is none of 1/2/3; it is a **provider limitation**.

## Diagnosis (run the script, then read results)

| Step | Command | What it proves |
|------|---------|----------------|
| **A** | `kubectl get ns automata`; `kubectl -n automata get svc automata-automata`; `kubectl -n automata get endpoints automata-automata` | automata in this cluster + has a ready endpoint. |
| **B** | busybox pod in `websites`: `nslookup automata-automata.automata.svc.cluster.local` + `wget http://…:8081/snippet/<uuid>.js` | FQDN resolves from `websites` + backend directly reachable. |
| **C** | `kubectl -n kube-system logs deploy/traefik --tail=300 \| grep -iE '500\|snippet\|automata\|upstream\|refused\|no server\|dial\|ExternalName'` | Traefik's own view of the failure. |
| **D** | `kubectl get networkpolicy -A \| grep -iE 'automata\|websites\|kube-system'` | Any policy blocking the path. |

Step **C is the decisive one**: grep for `ExternalName`. If Traefik logs
`type ExternalName is not supported` (kubernetesgateway provider) or
`externalName services not allowed` (kubernetescrd provider) for the shim, the
root cause is the provider limitation — A/B/D become irrelevant.

### Decision tree

- **C shows `ExternalName ... not supported / not allowed`** → **root cause:
  Traefik provider rejects ExternalName backendRefs.** Skip the rest.
- **A fails** (no `automata` ns / service / endpoints) → **cluster mismatch.**
- **A passes, B's nslookup fails** → **DNS** failure from `websites`.
- **A passes, B's nslookup OK, wget times out/refused** → **NetworkPolicy**
  blocking `websites`→automata:8081.
- **A + B fully pass, site still 500s** → Traefik-specific (policy
  Traefik→automata, or Traefik's ExternalName DNS). Check C and D.

## Status (as of 2026-10-31)

- **A: PASSES.** `automata` ns active; `automata-automata` ClusterIP
  `10.43.209.89` with ports 8080/8081; ready endpoint `10.42.0.177:8080,8081`.
- **C: root cause captured.** `kubernetesgateway` provider logs
  `h2hsecure-prod-automata-tracking: type ExternalName is not supported for
  Kubernetes Service reference`; the CRD provider logs the same class of
  failure for the sibling `pdaccess` site (`externalName services not
  allowed: websites/pdaccess-prod-automata-tracking`).
- **D: no blocking policy.** Only `automata/automata-postgresql` exists.
- **B: moot.** The shim is rejected before any backend traffic; pod-level
  reachability is irrelevant.
- **Scope note:** the failure is systemic — every site using this ExternalName
  shim pattern (h2hsecure, pdaccess) is affected, not just h2hsecure.

## Fix (confirmed root cause: Traefik rejects ExternalName backendRefs)

Replace the ExternalName shim with a **cross-namespace `backendRef` +
`ReferenceGrant`** — standard Gateway API v1, keeps the request same-origin, and
is stable across automata redeploys (referenced by name, not ClusterIP).

**This is already implemented** in `h2hsecure.github.io` commit `095bf2b`
("fix(chart): route tracking via cross-namespace backendRef + ReferenceGrant",
pushed to `main` 2026-10-30 21:34 UTC):

- `charts/h2hsecure/templates/tracking.yaml` now renders a `ReferenceGrant`
  **into the backend namespace** (`automata`), named
  `{{ .Release.Name }}-track-automata`, permitting the release's HTTPRoute to
  reference `automata-automata:8081`.
- `charts/h2hsecure/templates/httproute.yaml` points the
  `/snippet,/track,/form,/banner` rule directly at
  `namespace: automata, name: automata-automata, port: 8081`
  (values: `tracking.{backendNamespace,backendService,servicePort}`); the
  ExternalName Service is gone from the chart.

### What remains: get it deployed

The fix is on `main` but **not in the cluster** — Traefik kept logging the
ExternalName rejection until 2026-10-31 00:01 UTC, 3.5 h after the push.
`deploy.yml` runs on the Gitea Actions runner in the prod cluster, so:

1. Check the Gitea Actions run for `095bf2b` (or the latest `main` deploy run):
   did it run, and did `helm upgrade` succeed?
2. Verify cluster state:
   ```sh
   kubectl -n websites get svc h2hsecure-prod-automata-tracking   # should be GONE
   kubectl -n automata get referencegrant                         # should show h2hsecure-prod-track-automata
   kubectl -n websites get httproute h2hsecure-prod-h2hsecure-route -o yaml | grep -A5 backendRefs
   ```
3. If the run failed, fix and re-run; if it never ran, re-run `deploy.yml`
   (workflow_dispatch) for production.
4. Repeat for the sibling `pdaccess` site (same systemic fix, same pattern).
5. Final acceptance:
   `curl -sI https://www.h2hsecure.com/snippet/422adc7a-5b07-49a5-bd00-c6a44d61a172.js`
   → `HTTP/2 200` + `content-type: application/javascript`.

This preserves the hard same-origin constraint: the response is still served
from `www.h2hsecure.com`, so adblocker bypass is unaffected.

## Secondary finding (2026-10-31): tracking server 404s HEAD requests

While live-testing the snippet, `HEAD /snippet/<uuid>.js` on
`m.h2hsecure.com` returned **404** while `GET` returned 200 (reproduced locally:
gin v1.12.0 no longer auto-maps HEAD to GET, so every GET-only route 404s
HEAD — static and parameterized alike).

- **Browser impact: none.** `<script src>` uses GET; tracking works.
- **Tooling impact: real.** `curl -I`/HEAD-based health checks and monitoring
  get false 404s on every tracking GET route (`/snippet`, `/track/banner/*`,
  `/track/placement/*`, `/track/campaign/*`, `/form/*`, `/health`).

Fix (commit pending in `internal/adapter/tracking/server.go`): register each
public GET route for HEAD as well; covered by
`internal/adapter/tracking/server_test.go`
(`TestSnippetRoute_HeadServes200LikeGet`, `TestHealthRoute_HeadServes200LikeGet`).

## Test gap (why it shipped broken)

`e2e/tracking.spec.ts` (Nuxt repo) only asserts the `<script src="/snippet/…">`
tag is **injected into the HTML** — it never fetches `/snippet/` and asserts 200.
So a dead tracking route is invisible to the suite. Fix: add an e2e that requests
the snippet URL and expects `200` + `application/javascript`.