# Production Deployment — Automata

Run these commands on a machine that has access to the production cluster.
Only one value is unknown ahead of time: your production domain (`<PROD-DOMAIN>`).

## Current release

| Artifact | Tag |
|---|---|
| Image | `ghcr.io/hekemen/automata:9882c97` |
| Chart | `2026.10.23-9882c97` (OCI: `oci://ghcr.io/hekemen/automata-chart/automata`) |

`9882c97` is CI-verified (includes the stale-JWT fix, the PostgreSQL
readiness init container, the advisory-lock fix that serializes concurrent
automata+bff migrations, ingress TLS support, and lenient handling of
`ingress.tls[].hosts` as a bare string or a list).

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
  --version 2026.10.23-9882c97 -n automata \
  -f values.prod.yaml --set image.tag=9882c97
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