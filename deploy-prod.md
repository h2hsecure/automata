# Production Deployment — Automata

Run these commands on a machine that has access to the production cluster.
Only one value is unknown ahead of time: your production domain (`<PROD-DOMAIN>`).

## Current release

| Artifact | Tag |
|---|---|
| Image | `ghcr.io/hekemen/automata:f4024d2` |
| Chart | `2026.10.26-f4024d2` (OCI: `oci://ghcr.io/hekemen/automata-chart/automata`) |

`f4024d2` is CI-verified. New in this release: contact CSV import/export
plus tag CRUD (HTTP + MCP, 52-tool catalog), session revocation on
password change (login tokens now carry `iat`), `jti` on all issued JWTs,
the BFF as a docker-compose service, `form.submitted` webhook delivery
to matching endpoints, and the tracking snippet now flushes its event
queue on page unload so pageviews are not dropped when visitors
navigate away before the 30s batch timer fires. The previous release (`6a17eef`) included the stale-JWT fix, the PostgreSQL
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
  --docker-server=ghcr.io --docker-username=hekemen --docker-password="$GHCR_PAT"
```

`GHCR_PAT` is a GitHub token with read/push on `ghcr.io/hekemen/automata`
(the same one used for staging works).

## 2) Create `values.prod.yaml` (do NOT commit)

```yaml
ingress:
  hostname: <PROD-DOMAIN>
  annotations:
    cert-manager.io/cluster-issuer: <your-issuer>  # or cert-manager.io/issuer: <name>
  tls:
    - hosts: [<PROD-DOMAIN>]
      secretName: automata-tls
secretKey: "<random 32+ chars>"
cookieSecret: "<random 32+ chars>"
adminEmail: admin@<PROD-DOMAIN>
adminPassword: "<strong password>"
postgresql:
  auth:
    password: "<strong DB password>"
```

The `tls` block + issuer annotation make cert-manager provision a
certificate into the `automata-tls` secret, which the ingress terminates.
(If your cluster only has a plain `Issuer` in the `automata` namespace, use
`cert-manager.io/issuer` with that issuer's name.)

> **Secrets must be fresh.** The staging `secretKey`, `cookieSecret`,
> admin password, and Postgres password are all public in git history —
> do NOT reuse them in production.
>
> Generate with: `openssl rand -hex 32`

## 3) Deploy

```bash
helm upgrade -i automata oci://ghcr.io/hekemen/automata-chart/automata \
  --version 2026.10.26-f4024d2 -n automata \
  -f values.prod.yaml --set image.tag=f4024d2
```

## 4) Verify rollout

```bash
kubectl -n automata rollout status deploy/automata-automata
kubectl -n automata rollout status deploy/automata-bff
kubectl -n automata rollout status statefulset/automata-postgresql
kubectl -n automata get pods
```

Then check health over the public domain:

```bash
curl -s https://<PROD-DOMAIN>/api/health   # expect {"status":"ok"}
```

## 5) Verify TLS

```bash
# cert issued and ready (READY=True)
kubectl -n automata get certificate automata-tls
curl -sI https://<PROD-DOMAIN>/api/health   # expect 200 over TLS
```

## Notes

- Future releases: bump both tags to the new 7-char SHA
  (`make deploy-staging` pattern: `IMG=<sha> CHART=<YYYY.MM.DD>-<sha>`).