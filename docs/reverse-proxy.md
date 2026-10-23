# Reverse Proxy: Public Endpoint Topology

Which Automata endpoints must be reverse-proxied, and which internal
service/port each public path maps to. Verified against the Helm chart and
the three HTTP servers (automata :8080, tracking :8081, bff :8082).

## Topology

```
Internet (TLS terminated at ingress)
  └── Ingress  host: <your-domain>
        └── /  (Prefix)  →  automata-bff :8082
              ├── serves the web UI (/, /login, /dashboard, static, SPA fallback)
              ├── own routes: POST /login, POST /logout, GET /health
              └── proxies /api/*  →  automata-automata :8080  (1:1 path, no rewrite)

        (public by design, NOT routed today — see "Gaps" below)
              /track*  /snippet*  /form*  →  automata-automata :8081
```

The only ingress rule the chart ships is `/` → BFF :8082
(`helm/automata/templates/ingress.yaml`). The `automata-automata` Service
exposes both 8080 (api) and 8081 (tracking); only 8080 is reachable, and
only through the BFF's `/api/*` proxy.

## Internal listeners

| Service | Port | Role | Directly exposed? |
|---|---|---|---|
| `automata-automata` | 8080 | Admin REST API, MCP, context-resolved routes | No — via BFF `/api/*` only |
| `automata-automata` | 8081 | Tracking (called from JS snippets embedded on customer sites) | Intended to be public; not routed today |
| `automata-bff` | 8082 | Web UI + session auth + `/api/*` proxy | Yes — sole ingress backend |

## What the BFF serves vs. proxies (`cmd/bff/main.go`)

**Serves itself:**

| Path | Handler |
|---|---|
| `GET /`, `GET /login` | UI pages |
| `GET /dashboard` | 302 → `/` |
| `POST /login`, `POST /logout` | auth (sets/clears session cookie) |
| `GET /health` | health check |
| everything else not under `/api/` | SPA fallback (static assets, client routes) |

**Proxies:** `/api/*` → `bff.api_backend_url`
(default `http://automata-automata:8080`). Path is forwarded verbatim. If a
valid `automata_session` cookie is present, the proxy injects
`Authorization: Bearer <user_id>`; otherwise the request goes through
unauthenticated.

## Automata :8080 API surface (reachable only via BFF `/api/*`)

| Public path | Notes |
|---|---|
| `/health` | health check |
| `/api/auth/*` | login, `me`, profile, password change, logout-all (no context) |
| `/api/admin/*` | admin-authenticated tenant/context/CRUD routes |
| `/api/mcp` | MCP streamable HTTP (stateless POST); `/api/mcp-skill.md` (auth) |
| `/api/context/...` | context-resolved routes (forms, email, settings, …) |
| `/api/form/*path`, `/api/snippet/*path`, `/api/static/*path` | `adapter/proxy` — **placeholder stubs** (see Gaps) |

Context resolution on context-resolved routes
(`internal/adapter/api/context_middleware.go`), in priority order:

1. `X-Context-ID` header
2. `context_id` query parameter (used by the SPA)
3. Subdomain (`<slug>.<domain>`, via the `Host` header; `www`/`api`/`mail` are skipped)
4. Path slug (`/context/<slug>`, `/t/<slug>`)
5. Fallback: first active context (single-context deployments)

## Tracking :8081 endpoints — public by design, no auth

These are called from JavaScript snippets embedded on customers' websites,
so they must be reachable from the public internet without a session:

| Path | Purpose |
|---|---|
| `POST /track`, `POST /track/:cid`, `POST /track/batch` | event ingestion |
| `GET /snippet/:id.js` | tracking JS for a context (derives its own base URL from the request `Host`) |
| `GET /track/banner/list`, `GET /track/banner/:id` | banner data for the snippet |
| `GET /track/placement/:id`, `GET /track/campaign/:id` | placement/campaign data for the snippet |
| `GET /form/list`, `GET /form/:id`, `POST /form/submit/:slug`, `GET /form/:id/submissions` | public form API (context via `X-Context-ID` header / `context_id` query) |
| `GET /form/definition/:contextID/:slug` | form definition JSON (fetched by the form snippet) |
| `GET /form/snippet/:contextID/:slug` | self-rendering form snippet (uses relative URLs on the same origin) |
| `GET /health` | health check |

## What the ingress needs (current state + target)

Currently (what the chart ships):

```yaml
- path: /
  pathType: Prefix
  backend:
    service: { name: automata-automata-bff, port: { number: 8082 } }
```

To make tracking work, add (no collisions — the BFF's proxied paths are all
under `/api/`, while these are top-level):

```yaml
- path: /track
  pathType: Prefix
  backend:
    service: { name: automata-automata, port: { number: 8081 } }
- path: /snippet
  pathType: Prefix
  backend:
    service: { name: automata-automata, port: { number: 8081 } }
- path: /form
  pathType: Prefix
  backend:
    service: { name: automata-automata, port: { number: 8081 } }
```

## Reverse-proxy requirements

Any proxy in front of Automata (ingress, nginx, cloud LB) must:

1. **Preserve the `Host` header.** Context resolution uses the subdomain
   (`<slug>.<domain>`), and the tracking snippet derives its base URL from
   it. Do not rewrite `Host`; wildcard DNS + a wildcard ingress rule are
   needed for subdomain-based contexts.
2. **Pass `X-Context-ID` and `context_id` through** — both are context
   resolution inputs and must not be stripped.
3. **Forward cookies untouched** — `automata_session` (encrypted session)
   and the refresh cookie carry authentication.
4. **No path rewriting** — the BFF forwards `/api/*` 1:1 to :8080 and the
   tracking server is addressed at its top-level paths; any rewrite breaks
   both.
5. **Terminate TLS at the edge**; all internal hops are plain HTTP.
6. No streaming/buffering special-cases are required — no SSE or chunked
   long-lived responses exist in the API, MCP (stateless POST), or tracking
   adapters.

## Gaps and known issues

1. **Tracking (:8081) is not routed in the ingress.** The Service exposes
   the port, but no ingress path reaches it, so customer-site snippets
   (`/track`, `/snippet/:id.js`, `/form/definition/...`) are unreachable in
   a standard deployment. Add the three paths above (or decide tracking is
   intentionally private and document that).
2. **Tracking snippet hardcodes `http://`.**
   `internal/adapter/tracking/snippet_handler.go` builds the snippet's
   tracking URL as `"http://" + Host`, ignoring the original scheme. On an
   HTTPS site the embedded snippet posts to `http://<host>` and browsers
   block it as mixed content. It should preserve the request scheme.
3. **`adapter/proxy` is a stub.** `/api/form/*`, `/api/snippet/*`, and
   `/api/static/*` on :8080 return placeholder HTML and a snippet pointing
   at `https://automata.example.com` — not a real implementation.