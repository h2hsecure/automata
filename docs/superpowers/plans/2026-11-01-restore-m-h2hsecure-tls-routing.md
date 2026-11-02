# Restore m.h2hsecure.com TLS + routing (post-nginx-migration)

## Context

`m.h2hsecure.com` has been dead since 2026-10-30. Root cause (confirmed): the
nginx-based reverse-proxy migration remapped Traefik's entrypoints off 80/443.
The working website Gateways prove the new scheme:

- HTTP  → port **8000**
- HTTPS → port **8443** (TLS Terminate, e.g. secret `h2hsecure-tls`)

The automata chart hardcodes Gateway listeners on 443/80 →
`PortUnavailable` ("no matching entryPoint for port 443 and protocol HTTPS")
→ Gateway `PROGRAMMED` empty → HTTPRoute `NotAllowedByListeners`.

Traefik LB: `152.53.254.200` (svc ports 80:30834, 443:32238, 2222:30946;
entrypoints 8000/8443 not on the svc — the edge nginx on the VM reaches them
some other way; see Task 3, open question).

The HEAD-404 fix is already deployed (`b2746ec`). This plan is routing/TLS only.

## Tasks

### Task 1 — Chart: configurable Gateway listener ports

Files: `helm/automata/values.yaml`, `helm/automata/templates/ingress.yaml`

1. `values.yaml`, under `ingress:` (after `gatewayClass`):

   ```yaml
     # Gateway listener ports. The cluster's Traefik entrypoints were moved
     # off 80/443 (edge nginx migration): HTTP on 8000, HTTPS on 8443.
     # Defaults keep classic 80/443 for clusters where Traefik listens there.
     httpPort: 80
     httpsPort: 443
   ```

2. `templates/ingress.yaml` — three hardcoded port lines:
   - line ~51 (HTTPS listener, tls branch): `port: 443` →
     `port: {{ .Values.ingress.httpsPort | int }}`
   - line ~60 (HTTP listener, tls branch): `port: 80` →
     `port: {{ .Values.ingress.httpPort | int }}`
   - line ~66 (HTTP listener, non-tls branch): `port: 80` →
     `port: {{ .Values.ingress.httpPort | int }}`

   The `-route-http` redirect route stays as-is (it targets the HTTP listener
   by parentRef, no port in the route).

3. Verify (helm template, no cluster needed):

   ```bash
   helm template t helm/automata | grep -A3 "kind: Gateway" | grep port   # → 443/80 defaults
   helm template t helm/automata --set ingress.httpPort=8000 --set ingress.httpsPort=8443 \
     --set ingress.tls[0].hosts=m.h2hsecure.com --set ingress.tls[0].secretName=automata-tls \
     --set ingress.certificate.enabled=true --set ingress.certificate.clusterIssuer=letsencrypt-prod \
     | grep -B2 "port: 8"   # → 8443 + 8000
   ```

4. Commit: `helm: make Gateway listener ports configurable (ingress.httpPort/httpsPort)`

### Task 2 — Ship chart + prod re-upgrade (user's machine)

1. Push → CI publishes chart `YYYY.MM.DD-<newsha>` (image tag unchanged: `b2746ec`).
2. Update `deploy-prod.md`:
   - "Current release" table: chart row → new version (image stays `b2746ec`).
   - Section 2 `values.prod.yaml` example: add under `ingress:`

     ```yaml
     # Traefik entrypoints were moved off 80/443 (edge nginx migration):
     # httpPort: 8000
     # httpsPort: 8443
     ```

     with a one-line note that these are REQUIRED on the prod cluster (chart
     defaults 80/443 are rejected with `PortUnavailable` there).
   - Section 3 helm command: `--version <new chart version>`, `--set image.tag=b2746ec` unchanged.
   - Commit + push (runbook doc, CI re-run harmless) — or amend into Task 1's push.
3. User (cluster machine): add `httpPort: 8000` + `httpsPort: 8443` to
   `values.prod.yaml`, then:

   ```bash
   helm upgrade -i automata oci://ghcr.io/h2hsecure/automata-chart/automata \
     --version <new-chart-version> -n automata \
     -f values.prod.yaml --set image.tag=b2746ec
   ```

4. Verify in-cluster (user runs, pastes back):

   ```bash
   kubectl -n automata get gateway automata
   kubectl -n automata get gateway automata -o jsonpath='{.status.conditions}' | python3 -m json.tool
   kubectl -n automata get httproute automata-route automata-route-http
   ```

   Success: Gateway `PROGRAMMED=True`, listeners `Accepted=True` (no
   `PortUnavailable`), both HTTPRoutes `Accepted=True` on both parents.

### Task 3 — Edge nginx: m.h2hsecure.com server block (user's VM)

Blocked on open question: **how does the existing working
`www.h2hsecure.com` nginx block reach Traefik's 8443 entrypoint?**
(LB svc doesn't expose 8443 — it must be hostPort, direct node/pod port, or a
side service.) Ask the user to paste the working `www.h2hsecure.com` server
block + upstream.

Then deliver a mirrored config:

- HTTP :80 `server` for `m.h2hsecure.com` → proxy to Traefik upstream port 8000
  (Traefik's `-route-http` does the 301 to HTTPS).
- HTTPS :443 `server` for `m.h2hsecure.com`:
  - real cert for `m.h2hsecure.com` (currently SNI serves default
    `*.pdaccess.com` — no m.* cert at the edge; user must obtain one, e.g.
    certbot on the VM, mirroring how `www.h2hsecure.com`'s cert was obtained);
  - `proxy_pass` to the same Traefik upstream as the website, port **8443**,
    SNI `m.h2hsecure.com` (`proxy_ssl_server_name on; proxy_set_header Host $host;`).
- Reload nginx, no other hostnames touched.

### Task 4 — External verification (text/HTTP only, no images)

```bash
# SNI serves a real m.* cert, not the *.pdaccess.com default
openssl s_client -connect 152.53.254.200:443 -servername m.h2hsecure.com </dev/null 2>/dev/null | openssl x509 -noout -subject

# health (also proves new-image routing end-to-end)
curl -sI https://m.h2hsecure.com/api/health        # 200
curl -sI -X HEAD https://m.h2hsecure.com/api/health  # 200 (HEAD canary, b2746ec)
curl -s https://m.h2hsecure.com/snippet/422adc7a-5b07-49a5-bd00-c6a44d61a172.js | head -5  # 200 JS

# http → 301
curl -sI http://m.h2hsecure.com/                   # 301 Location: https://...
```

All four green = done.

## Risks / notes

- No pod changes: `helm upgrade` re-renders only Gateway/HTTPRoute objects
  (ports changed); expect no pod restarts, no data impact, Postgres untouched.
- Rollback = helm upgrade back to chart `2026.11.01-b2746ec` (restores 80/443,
  i.e. the broken state — only useful if the nginx work breaks something else).
- Pre-existing unrelated test failures (`TestContextMiddleware`, proxy build)
  must not be "fixed" as part of this; they predate the change.