# Production Deployment — Automata

Run these commands on a machine that has access to the production cluster.
Only one value is unknown ahead of time: your production domain (`<PROD-DOMAIN>`).

## Current release

| Artifact | Tag |
|---|---|
| Image | `ghcr.io/hekemen/automata:64b2fc9` |
| Chart | `2026.10.23-64b2fc9` (OCI: `oci://ghcr.io/hekemen/automata-chart/automata`) |

`64b2fc9` is CI-verified and staging-proven (includes the stale-JWT fix and
the PostgreSQL readiness init container).

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
secretKey: "<random 32+ chars>"
cookieSecret: "<random 32+ chars>"
adminEmail: admin@<PROD-DOMAIN>
adminPassword: "<strong password>"
postgresql:
  auth:
    password: "<strong DB password>"
```

> **Secrets must be fresh.** The staging `secretKey`, `cookieSecret`,
> admin password, and Postgres password are all public in git history —
> do NOT reuse them in production.
>
> Generate with: `openssl rand -hex 32`

## 3) Deploy

```bash
helm upgrade -i automata oci://ghcr.io/hekemen/automata-chart/automata \
  --version 2026.10.23-64b2fc9 -n automata \
  -f values.prod.yaml --set image.tag=64b2fc9
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

## Notes

- If your prod ingress uses cert-manager, add the issuer annotation under
  `ingress.annotations` in `values.prod.yaml` for TLS, e.g.:

  ```yaml
  ingress:
    annotations:
      cert-manager.io/issuer: <your-issuer>
      nginx.ingress.kubernetes.io/proxy-body-size: "10m"
  ```

- Future releases: bump both tags to the new 7-char SHA
  (`make deploy-staging` pattern: `IMG=<sha> CHART=<YYYY.MM.DD>-<sha>`).