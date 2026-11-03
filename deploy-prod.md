# Production Deployment — Automata

Run these commands on a machine that has access to the production cluster.
Only one value is unknown ahead of time: your production domain (`<PROD-DOMAIN>`).

## Current release

| Artifact | Tag |
|---|---|
| Image | `ghcr.io/h2hsecure/automata:a139eab` |
| Chart | `2026.11.03-a139eab` (OCI: `oci://ghcr.io/h2hsecure/automata-chart/automata`) |

The release is CI-verified. New in this release: the merged pages system — banners are gone, a banner is just a page with a `position` and `buttons`. The `Page` entity absorbs the banner `Button` and `Decision`: `{slug, title, content, is_active, position, buttons}`, where `position` is `""` (no overlay), `top`, `bottom`, `left`, `right` (fixed ~320px edge strips) or `full` (opaque overlay). Buttons are `{label, href, action, variant}` with `action` ∈ `link` / `dismiss_accept` / `dismiss_decline`; a `full` page whose buttons are all `link` never dismisses. Per-visitor decisions are recorded on `page_decisions` and impressions on `page_impressions` (client IP stored only as a sha256 hash). Every `/banner/*` route is retired; the tracking server now serves `GET /page/config/:contextID` (JSON; inlines the page content of the first `href: "/"` button; records an impression when `?visitor_id=` is given), `POST /page/decision/:contextID` (`text/plain` body `{"page_id","decision","visitor_id"}` → 204), and `GET/HEAD /page/snippet/:contextID.js` (the self-rendering snippet — `pkg/snippet.GeneratePageConfigSnippet`; the legacy `GET /snippet/:uuid.js` is unchanged). Pages remain per-context legal content managed via `CRUD /api/context/pages`, served publicly as a minimal HTML doc at `GET /page/:contextID/:slug` (404 when missing or inactive). A cookie-consent banner is now just one page config: a `full` (or `bottom`) page with an accept (`dismiss_accept`), a decline (`dismiss_decline`) and a cookie-policy `link` button. In the WebUI the Banners view is deleted and its editor (position + buttons) and a per-page decisions panel are merged into the Pages view; the dashboard card is now "Pages". The MCP catalog drops the banner/placement/campaign tools (42 tools). **Companion website change:** merge `h2hsecure.github.io` branch `static-nginx` into `main` — the customer site's nginx tracking regex becomes `^/(snippet|track|form|page)` and the snippet is re-embedded as `<script src="/page/snippet/<uuid>.js">` in place of the old `/banner/snippet/` tag. Deploy order: (1) merge this branch and push → CI publishes the image + chart; (2) merge the website `static-nginx` branch → the customer site proxies `/page/` and re-embeds the snippet; (3) helm upgrade with the new tags; (4) create the cookie page in the WebUI (`position: full`, accept/decline/link buttons) and run the canaries below. Deploy canaries: `GET https://www.h2hsecure.com/page/config/<contextID>` → 200 JSON, `GET https://www.h2hsecure.com/page/snippet/<contextID>.js` → 200, `GET https://www.h2hsecure.com/page/<contextID>/cookie-policy` → 200 HTML (after the page exists), `POST https://www.h2hsecure.com/page/decision/<contextID>` (`text/plain`) → 204; retired `/banner/*` URLs → 404. **Note:** the image/chart tags in the table above are the last CI-published release; bump them to the new CI-published SHA after this release lands (CI publishes `ghcr.io/h2hsecure/automata:<sha8>` and chart `YYYY.MM.DD-<sha8>` on merge to `main`). The previous release (`670e91e`): (1) the Gateway HTTPRoutes are scoped to their listeners via `parentRefs.sectionName` — the HTTP listener (port 8000) now serves exclusively the 301 redirect to HTTPS; previously both routes bound to every listener and, with no Gateway API route priority, Traefik's deterministic router tiebreak let the main route win on plain HTTP; (2) `HEAD` is accepted on the public `/health` routes of the api (`/api/health`), bff, admin, and standalone webui servers — gin v1.12 no longer auto-maps HEAD to GET (the tracking server got the same fix in `b2746ec`). Deploy canaries: `http://m.h2hsecure.com/` → 301 to HTTPS and `HEAD https://m.h2hsecure.com/api/health` → 200. The previous release (`1f3e2e0`) made the Gateway listener ports configurable (`ingress.httpPort`/`ingress.httpsPort`; REQUIRED `8000`/`8443` on the prod cluster — see section 2) and restored `m.h2hsecure.com` TLS + routing after the Traefik entrypoint remap. The release before that (`b2746ec`) added `HEAD` support to the tracking server's public GET routes. The previous release (`2317fb6`) introduced the forms submissions page — the submissions modal in the forms list is replaced by a dedicated page at `forms/:id/submissions` (same one-column-per-field layout, attached files stay downloadable, pagination preserved); each submission can now be deleted from that page via a new context-scoped `DELETE /context/forms/:id/submissions/:sid` endpoint (204 on success, 404 when the submission is missing or belongs to another context, 401 without a context), and form creation now seeds a default `company` field that maps to the contact's Company. Also in this release the repo moved to `github.com/h2hsecure/automata`: the Go module path and all CI/helm references changed, so the image and chart now live under `ghcr.io/h2hsecure/...` (pull with a token that has access to the new account's packages, or make the packages public). The previous release (`cfc5fcf`) introduced the dashboard redesign —
an SVG world map of contact locations (last 90 days) with choropleth shading
and per-country count badges, a reworked Top Pages chart (horizontal bars with
rank, truncated URL, and count), the empty Events tab removed, and the Forms
and Banners cards now navigate to their pages; hidden contacts are always shown
in the contacts list and can be filtered (all / hidden / visible). The map is a
bundled static asset (no runtime chart dependency) and the dashboard fetches
contacts via a new `since` query param on `GET /context/contacts`. The previous
release (`51b532a`) fixed the production 404 after the Ingress→IngressRoute
conversion — the body-limit middleware now uses Traefik **v3**'s
`buffering.maxRequestBodyBytes`
(int64 bytes; the production cluster runs Traefik v3, where v2's
`requestBody.maxBodySize` was removed and is rejected with "invalid
middleware type or middleware does not exist", which took down every
router referencing the middleware). The HTTP→HTTPS redirect
IngressRoute also carries a placeholder BFF service, because this
Traefik build rejects service-less routers. Default `ingress.maxBodySize` is 100 MiB
(104857600 bytes), matching the old nginx Ingress's `proxy-body-size
100M`; override in `values.prod.yaml` as an integer byte count. The
previous release (`49ba51c`) introduced form submissions in the WebUI
and dual-audience form-integrated emails — each form row has a Submissions action
that opens a modal with one column per form field (labels from field
slugs), attached files downloadable through the authenticated client,
pagination (20 per page), and an empty state. Form-integrated email
templates now have an audience: submitter (the default, same as before) or
admin. On submission, the submitter email goes to the person who filled
out the form and admin emails go to per-form notification recipients set
in the form editor (comma-separated list); each audience can be toggled
per form, admin emails only send when at least one recipient is set, and
email failures never fail a submission (logged only). The previous release
(`63c8a40`) made duplicate API key names in the same context return a
clean 409 ("an API key with this name already exists in this context")
instead of a raw 500 Postgres constraint error. The release before that
(`f4024d2`) included contact CSV import/export plus tag CRUD (HTTP + MCP,
52-tool catalog), session revocation on password change (login tokens now
carry `iat`), `jti` on all issued JWTs, the BFF as a docker-compose
service, `form.submitted` webhook delivery to matching endpoints, and the
tracking snippet now flushes its event queue on page unload so pageviews
are not dropped when visitors navigate away before the 30s batch timer
fires. The release before that (`6a17eef`) included the stale-JWT fix, the PostgreSQL
readiness init container, the advisory-lock fix that serializes concurrent
automata+bff migrations, ingress TLS support, lenient handling of
`ingress.tls[].hosts` as a bare string or a list, the cookie-secret
fix that accepts any secret length of 16+ characters, the tracking
fix: ingress now routes `/snippet`, `/track`, `/form` to the tracking
server on port 8081, and `ServeSnippet` honors `X-Forwarded-Proto` so
snippets on HTTPS sites post back over https, and contact visitor info:
each hidden contact now records user-agent, parsed device/browser, client
IP, and offline GeoLite2 geolocation from the embedded
GeoLite2-City database — no runtime dependency on MaxMind — shown in the
WebUI contacts list and detail view; the embedded database can be
refreshed monthly with `MAXMIND_LICENSE_KEY=<key> make geoip-update`).

## 0) Point at the production cluster + helm access to GHCR (one-time)

```bash
kubectl config use-context <prod-context>
helm registry login ghcr.io
```

## 1) Namespace + image pull secret

```bash
kubectl create namespace automata
kubectl -n automata create secret docker-registry ghcr-registry \
  --docker-server=ghcr.io --docker-username=h2hsecure --docker-password="$GHCR_PAT"
```

`GHCR_PAT` is a GitHub token with read/push on `ghcr.io/h2hsecure/automata`
(the same one used for staging works).

## 2) Create `values.prod.yaml` (do NOT commit)

```yaml
ingress:
  hostname: m.h2hsecure.com
  gatewayClass: traefik        # kubectl get gatewayclass (default in chart)
  # REQUIRED on this cluster: Traefik's entrypoints were moved off 80/443
  # during the edge-nginx migration (websites' Gateways run on 8000/8443).
  # Without these, the Gateway listeners are rejected (PortUnavailable) and
  # every HTTPRoute is NotAllowedByListeners.
  httpPort: 8000
  httpsPort: 8443
  tls:
    - hosts: [m.h2hsecure.com]
      secretName: automata-tls   # existing Certificate/secret in the automata ns
  certificate:
    enabled: true                # REQUIRED when ingress.tls is set
    clusterIssuer: letsencrypt-prod
  # Max upload size, in bytes (int64) — chart default is 104857600 (100 MiB):
  # maxBodySize: 52428800   # 50 MiB
secretKey: "<random 32+ chars>"
cookieSecret: "<random 32+ chars>"
adminEmail: admin@<PROD-DOMAIN>
adminPassword: "<strong password>"
postgresql:
  auth:
    password: "<strong DB password>"
```

The chart renders Gateway API objects (`gateway.networking.k8s.io/v1`)
bound to the GatewayClass named in `ingress.gatewayClass` — a `Gateway`
(TLS listeners + cert refs when `ingress.tls` is set) and `HTTPRoute`s
whose rules carry `ExtensionRef` filters (pointing at a Traefik `Middleware`)
for the upload size limit — Gateway API has no standard body-limit filter.
With `ingress.tls` set, HTTP for the host is 301-redirected to HTTPS via
a redirect-only `HTTPRoute` on the `ingress.httpPort` listener.

TLS: `ingress.tls` requires `certificate.enabled: true` + `clusterIssuer`
(e.g. `letsencrypt-prod`) — cert-manager does not watch Gateway resources,
so the chart renders a `Certificate` that writes the `tls[].secretName`
secret. Before the first upgrade with this, delete the out-of-chart
`Certificate` named `automata-tls`
(`kubectl -n automata delete certificate automata-tls`) so Helm can own the
object — the `automata-tls` secret itself is kept and cert-manager adopts
it (no re-issue, no TLS blip).

> **Secrets must be fresh.** The staging `secretKey`, `cookieSecret`,
> admin password, and Postgres password are all public in git history —
> do NOT reuse them in production.
>
> Generate with: `openssl rand -hex 32`

## 2.5) Gateway API CRDs (one-time, before the first Gateway-based deploy)

The prod cluster's `gateway.networking.k8s.io` CRDs are owned by the
`traefik-crd` helm release (old generation). They must be refreshed to the
current Gateway API before this chart's `Gateway`/`HTTPRoute` objects are
accepted.

The obvious path — upgrading that release to the `traefik-crds` chart — is
broken: the chart renders a ~1.27 MiB manifest, which exceeds Helm's
1 MiB release-secret limit (`Secret ... is invalid: data: Too long`). It
breaks any release carrying those CRDs, so the CRDs have to be applied
out-of-band with kubectl instead:

```bash
# (check first: does the main traefik release also render CRDs? If so, the
# same "never upgrade" drift rule below applies to it too)
helm get manifest traefik -n kube-system | grep -c CustomResourceDefinition

curl -sfL https://traefik.github.io/charts/traefik-crds/traefik-crds-1.18.0.tgz -o /tmp/tc.tgz
tar xzf /tmp/tc.tgz -C /tmp
kubectl apply -f /tmp/traefik-crds/crds-files/gatewayAPI/gateway-standard-install.yaml
kubectl apply -f /tmp/traefik-crds/crds-files/traefik/
```

Then neutralize the stale release so it can never roll the schemas back.
If the live CRDs carry `helm.sh/resource-policy: keep` (check:
`kubectl get crd gateways.gateway.networking.k8s.io -o jsonpath='{.metadata.annotations.helm\.sh/resource-policy}'`),
delete the release — it removes the release and its history without
touching the CRDs:

```bash
helm delete traefik-crd -n kube-system
```

If the annotation is empty, leave the release alone and treat both
`traefik-crd` (and `traefik`, if it renders CRDs) as frozen — the CRDs are
kubectl-managed from now on.

Verify the refresh took effect — the v1 HTTPRoute filter enum must list 7
types (includes `CORS`):

```bash
kubectl get crd httproutes.gateway.networking.k8s.io -o json | python3 -c "
import json, sys
d = json.load(sys.stdin)
v = [v for v in d['spec']['versions'] if v['name'] == 'v1'][0]
f = v['schema']['openAPIV3Schema']['properties']['spec']['properties']['rules']['items']['properties']['filters']['items']['properties']['type']
print(sorted(f['enum']))
"
```

If the `traefik` main release *does* render CRDs too, upgrade its release
to the current traefik chart as well (it depends on `traefik-crds`) so the
two releases do not fight over the CRDs.

## 3) Deploy

```bash
helm upgrade -i automata oci://ghcr.io/h2hsecure/automata-chart/automata \
  --version 2026.11.03-a139eab -n automata \
  -f values.prod.yaml --set image.tag=a139eab
```

## 4) Verify rollout

```bash
kubectl -n automata rollout status deploy/automata-automata
kubectl -n automata rollout status deploy/automata-bff
kubectl -n automata rollout status statefulset/automata-postgresql
kubectl -n automata get pods
```

Then check health, the HTTP→HTTPS redirect, and TLS over the public domain:

```bash
curl -s https://<PROD-DOMAIN>/api/health        # expect {"status":"ok"}
curl -sI http://<PROD-DOMAIN>/                  # expect 301 -> https
curl -sI https://<PROD-DOMAIN>/api/health       # expect 200 over TLS
curl -sI -X HEAD https://<PROD-DOMAIN>/api/health   # expect 200 (gin 1.12: HEAD is not auto-mapped to GET)
# cert issued and ready (READY=True)
kubectl -n automata get certificate automata-tls
```

New page endpoints (tracking server, `m.h2hsecure.com`;
`<CTX>` = `422adc7a-5b07-49a5-bd00-c6a44d61a172` for the h2hsecure site —
replace with your context ID). These verify the ingress now forwards
`/page/` to tracking `:8081` (and that the retired `/banner/` routes 404):

```bash
curl -s https://m.h2hsecure.com/page/config/<CTX>   # 200 JSON (empty until a positioned page is created)
curl -sI https://m.h2hsecure.com/page/snippet/<CTX>.js   # 200, application/javascript
curl -s -o /dev/null -w '%{http_code}\n' https://m.h2hsecure.com/page/<CTX>/cookie-policy   # 404 until the page exists, 200 after
curl -s -o /dev/null -w '%{http_code}\n' -X POST -H 'Content-Type: text/plain' \
  -d '{"page_id":"<id>","decision":"accept","visitor_id":"v1"}' \
  https://m.h2hsecure.com/page/decision/<CTX>   # 204 (404 for an unknown page_id)
curl -s -o /dev/null -w '%{http_code}\n' https://m.h2hsecure.com/banner/config/<CTX>   # 404 (retired)
```

Customer-domain canaries (only after the website `static-nginx` merge,
see the release notes above): the same five URLs on `www.h2hsecure.com`.

## Notes

- Future releases: bump both tags to the new short SHA
  (`git rev-parse --short HEAD` — 7 chars, as CI publishes it).
  (`make deploy-staging` pattern: `IMG=<sha> CHART=<YYYY.MM.DD>-<sha>`).