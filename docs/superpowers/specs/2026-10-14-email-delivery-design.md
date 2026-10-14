# Design Spec: Email Delivery Pipeline — SMTP Mailer Implementation

**Date:** 2026-10-14
**Status:** Approved
**Reference:** [Email System Plan — Known Gap Note](../plans/2026-09-25-email-system-plan.md)
**Module:** `github.com/hekemen/automata`

---

## 1. Problem Statement

The email system (queue, templates, send endpoints) was built and shipped on 2026-09-25, but the **delivery pipeline is dead code in production**:

1. **No `Mailer` implementation exists.** The `queue.Mailer` interface (`internal/infrastructure/queue/email_queue.go:28`) is only implemented by the test mock `cicd/support/smtp_mock.go`. There is no production SMTP client.
2. **The worker is never started.** `emailQueue.StartWorker` (`internal/infrastructure/queue/email_worker.go:26`) has zero production callers. Jobs enqueued via `email_jobs` are never processed.
3. **No SMTP configuration exists.** `config.example.yaml` and the config contract have no `smtp.*` keys.

Result: every email enqueued through the form-submission, contact-created, and test-send flows sits in `email_jobs` forever. `specs/roadmap.md` claims "SMTP delivery … Done (2026-09-25)", which is incorrect.

---

## 2. Goals & Non-Goals

### Goals

1. **Real SMTP delivery** — a production `Mailer` implementation that speaks SMTP (dial, optional STARTTLS, optional AUTH, MAIL FROM, RCPT TO, DATA, QUIT).
2. **Wire the worker into `cmd/automata/main.go`** so enqueued jobs are actually processed with the existing retry/backoff behavior.
3. **Configurable via the existing config system** — flat dot-notation keys (`smtp.host`, `smtp.port`, …) with automatic `AUTOMATA_SMTP_*` env overrides.
4. **Fail loudly when unconfigured** — the worker still starts; each send attempt returns an "smtp not configured" error, jobs are retried with backoff, and discarded after `max_retries` with error logs. No silent no-op.
5. **Correct RFC-822 message construction** — plain-text or `multipart/alternative` (when an HTML body is present), with subject newlines sanitized to prevent header injection.
6. **Testable without a network** — dialer/client seams so unit tests and the cicd suite can inject fakes.

### Non-Goals

1. **No changes to the `email_jobs` schema or the queue interface** — `Queue` (`Enqueue` + `StartWorker`) and `Mailer` (`Send`) stay as-is.
2. **No new API endpoints** — the send/test-send endpoints already enqueue jobs; this spec only makes delivery work.
3. **No BCC / per-recipient addressing strategy** — all recipients go in a single `To:` header and per-recipient `RCPT TO` commands (sufficient for v1).
4. **No multi-relay / fallback relays** — one relay configuration.
5. **No webhook worker fix** — `NewWebhookWorker` (`internal/infrastructure/queue/webhook_worker.go:29`) has the same dead-wiring gap; it is reported as a finding and tracked separately.
6. **No cross-replica lock fix** — see Open Questions; the existing `FOR UPDATE SKIP LOCKED` semantics are documented as a known limitation, not fixed here.

---

## 3. Architecture

### 3.1 Delivery Flow

```
 enqueue (existing)                    worker (existing, now started)
 ──────────────────                    ────────────────────────────────
 SendEmail usecase                     StartWorker(ctx, mailer)
        │                                   │ 100ms tick
        ▼                                   ▼
 email_jobs row                    processNextBatch
 (attempts=0,                   (next_retry <= NOW()
  max_retries=3)                 AND attempts < max_retries)
                                         │
                                         ▼
                                    sendJob → mailer.Send(to, subject, body, htmlBody)
                                                         │
                                                         ▼
                                          ┌──────────────────────────────┐
                                          │  SMTPMailer (new package)    │
                                          │  internal/infrastructure/    │
                                          │  email/smtp                  │
                                          │                              │
                                          │  1. cfg.IsConfigured()?      │
                                          │     no → err "smtp not      │
                                          │     configured" (retry/discard)
                                          │  2. dialer.Dial()           │
                                          │     → TCP + EHLO            │
                                          │     → STARTTLS (if enabled) │
                                          │     → AUTH    (if username) │
                                          │  3. Mail(from)              │
                                          │  4. Rcpt(r) for each to[]   │
                                          │  5. Data(buildMessage(...)) │
                                          │  6. Close() (deferred)      │
                                          └──────────────────────────────┘
                                                         │ success
                                                         ▼
                                    completeJob (DELETE row)
                                                         │ failure
                                                         ▼
                                    retryJob (backoff 2^n × 10s, cap 1h;
                                    delete after max_retries)
```

### 3.2 Component Changes

```
internal/
└── infrastructure/
    └── email/
        ├── repo/            ← unchanged (templates + migrations)
        └── smtp/            ← NEW package
            ├── smtp.go      ← Config, Client, Dialer, SMTPMailer,
            │                  NewMailer, IsConfigured, DefaultDialer,
            │                  netDialer, buildMessage
            └── smtp_test.go ← unit tests (stdlib testing, same package)

cmd/
└── automata/
    └── main.go              ← MOVE ctx/cancel creation earlier;
                               build smtp.Config; NewMailer; StartWorker

cicd/
└── email_delivery_test.go   ← NEW integration tests (delivery, multipart,
                               fast discard, large body)

config.example.yaml          ← NEW smtp: section
specs/roadmap.md             ← fix incorrect "SMTP delivery Done" claim
```

### 3.3 Interface Design

The new package defines two small interfaces that mirror `net/smtp`'s shape, so the cicd mock satisfies them directly and the real client via a thin adapter:

```go
// Client mirrors the net/smtp.Client command surface.
type Client interface {
    Mail(from string) error
    Rcpt(to string) error
    Data(data []byte) error
    Close() error
}

// Dialer opens a ready-to-use SMTP session (post-AUTH, post-STARTTLS).
type Dialer interface {
    Dial() (Client, error)
}
```

- `*support.SMTPClient` (cicd mock) satisfies `Client` (same method set).
- The stdlib `*smtp.Client` does NOT satisfy `Client` directly: its `Data()` returns `(io.WriteCloser, error)`, and `smtp.Auth` is an interface (use `smtp.PlainAuth`). A thin `stdClient` adapter in the package wraps it and implements `Data([]byte) error` by writing the whole message and closing the writer.
- Production `netDialer` folds **dial + STARTTLS + AUTH** inside `Dial()`, so `Send` only deals with an authenticated session.
- The production dialer sets a 30s deadline on the connection after TCP connect, bounding the greeting/AUTH/STARTTLS handshake so a stalled relay cannot block the worker indefinitely (unit-tested with a real listener that accepts but never greets).

`SMTPMailer` implements the existing `queue.Mailer` interface — no queue changes required:

```go
type SMTPMailer struct {
    cfg    Config
    dialer Dialer
}

func (m *SMTPMailer) Send(to []string, subject, body, htmlBody string) error
```

Constructors: `NewMailer(cfg)` wires the production `netDialer`; `NewMailerWithDialer(cfg, dialer)` is the exported test seam that lets cicd tests inject the in-memory mock dialer (the `dialer` field is unexported, so cross-package injection needs this constructor).

---

## 4. Data Model

**No schema changes.** The `email_jobs` table (created by `internal/infrastructure/email/repo` migrations) already stores everything the delivery pipeline needs:

| Column | Used by delivery as |
|--------|--------------------|
| `id` | row identity; deleted on completion/discard |
| `context_id` | tenant scoping (not sent; informational) |
| `to_addresses` | `RCPT TO` list + `To:` header |
| `subject` | `Subject:` header (newlines sanitized) |
| `body` | plain-text part |
| `html_body` | HTML part (multipart when non-empty) |
| `attempts` / `max_retries` | retry budget (existing behavior) |
| `next_retry` | poll visibility (existing behavior) |

**Message construction (`buildMessage`):**

- Plain (no HTML): single-part `text/plain; charset="utf-8"` with `From:`, `To:`, `Subject:`, `Date:`, `MIME-Version:`, `Content-Type:` headers.
- Multipart (HTML present): `multipart/alternative` with a unique boundary; plain part first, HTML part second.
- Subject is sanitized: `\n` and `\r` replaced with spaces (prevents header injection).
- `To:` header joins all recipients with `, ` (matches the mock parser's single-line expectation).
- Line endings are CRLF throughout (RFC 5321 requires CRLF in the DATA payload); `Date:` is RFC 1123 UTC (RFC 5322 requires a Date header).

---

## 5. Config Contract

New keys (flat dot-notation, per `internal/infrastructure/config/config.go`):

| Key | Default | Env override | Description |
|-----|---------|--------------|-------------|
| `smtp.host` | `""` (unconfigured) | `AUTOMATA_SMTP_HOST` | Relay hostname |
| `smtp.port` | `587` | `AUTOMATA_SMTP_PORT` | Relay port |
| `smtp.username` | `""` | `AUTOMATA_SMTP_USERNAME` | AUTH username; empty = no AUTH |
| `smtp.password` | `""` | `AUTOMATA_SMTP_PASSWORD` | AUTH password |
| `smtp.from` | `""` (unconfigured) | `AUTOMATA_SMTP_FROM` | MAIL FROM / `From:` header |
| `smtp.starttls` | `true` | `AUTOMATA_SMTP_STARTTLS` | `false` disables STARTTLS (VPC relays on port 25) |

**Configuration semantics:**

- `IsConfigured()` is true only when **both** `smtp.host` and `smtp.from` are non-empty.
- Port is parsed from string; parse failure falls back to `587`.
- `starttls` defaults to `true`; only the literal string `"false"` disables it.
- Env overrides apply only to keys present in the YAML file (existing `applyEnvOverrides` behavior) — all six keys are included in `config.example.yaml`, so all six env overrides work out of the box.

**Unconfigured behavior (approved decision: "Start worker, fail loudly"):**

- The worker always starts.
- Each `Send` returns `errors.New("smtp not configured")`.
- The existing `retryJob` backoff (2^attempts × 10s, capped 1h) retries, then discards the job after `max_retries` (default 3) with an error log.
- `main.go` logs a startup warning when SMTP is unconfigured.
- The API contract is unchanged: send endpoints still return the job ID.

---

## 6. Key Design Decisions

### 6.1 Dialer folds AUTH/STARTTLS into `Dial()`

**Decision:** `Dialer.Dial()` returns a session that is already TLS-upgraded (if enabled) and authenticated (if a username is set). `Send` never issues SMTP commands directly.

**Rationale:**
- Keeps `Send` linear and trivially testable: dial → Mail → Rcpt × N → Data → Close.
- Matches the shape of the existing cicd mock (`SMTPDialer.Dial()` returns a ready `*SMTPClient`), so the mock drops in with a one-line adapter.
- Failure modes (bad credentials, TLS handshake) surface as `Dial` errors → standard retry path.

### 6.2 Unconfigured SMTP fails loudly instead of silently dropping

**Decision:** No `nil` mailer, no "skip if unconfigured" shortcut in the worker. The mailer is always passed; `Send` errors when unconfigured; the queue's existing retry/discard machinery handles it.

**Rationale:**
- The current silent path (worker never started; `sendJob` deletes the job when `mailer == nil`) hid the gap for three weeks. Failing loudly produces visible error logs and a startup warning.
- Reuses the existing retry/backoff/discard code — zero changes to `email_worker.go`.
- Jobs are discarded (not lost silently) and the event is logged at each attempt.

### 6.3 Interfaces mirror `net/smtp` rather than introducing a new mailer abstraction

**Decision:** `Client`/`Dialer` are minimal 4-method / 1-method interfaces mirroring the `net/smtp` command surface.

**Rationale:**
- `*support.SMTPClient` (mock) satisfies `Client` directly; the stdlib `*smtp.Client` needs a thin `stdClient` adapter because its `Data()` returns `(io.WriteCloser, error)` (the adapter makes the whole message one `Data([]byte)` call).
- No third-party SMTP dependency added (stdlib is sufficient for v1 relays).
- The `queue.Mailer` boundary (`Send(to, subject, body, htmlBody)`) stays the only interface the queue knows — hexagonal layering preserved (the smtp package is a driving adapter under `infrastructure/`).

### 6.4 Multipart only when HTML is present

**Decision:** `htmlBody == ""` → single-part text; otherwise `multipart/alternative` (plain first, HTML second).

**Rationale:**
- Matches how templates are rendered (`body_text` + `body_html`); clients pick the best part.
- Plain-first ordering keeps plain-text-only clients (and the cicd mock's naive parser) reading the text body.

### 6.5 Subject newline sanitization

**Decision:** Replace `\n` and `\r` in the subject with spaces before writing the `Subject:` header.

**Rationale:**
- The subject is user-influenced (template `subject_override`). A raw newline would allow injecting arbitrary headers (e.g., a forged `To:` or `Bcc:`) into the SMTP message.
- Body content is not sanitized — it sits below the header block, inside the (part) body, where newlines are legitimate.

### 6.6 Worker wiring in `main.go`, not in `NewServer`

**Decision:** `cmd/automata/main.go` creates the mailer and calls `queue.NewEmailQueue(pool).StartWorker(ctx, mailer)`. `api.NewServer` keeps its internal queue instance for enqueueing; its signature is unchanged.

**Rationale:**
- `NewServer` takes no context and is also called by cicd tests — threading a worker start through it would change a widely-used signature for no benefit.
- A second `emailQueue` instance in `main.go` is harmless: only the `main.go` instance runs a worker; the server's instance only enqueues.
- `ctx, cancel` creation (currently at the bottom of `main.go`) moves up so the worker can be started before the signal handler is registered.

---

## 7. Implementation Notes

- **Package:** `internal/infrastructure/email/smtp` (stdlib `net/smtp`, `crypto/tls`, `net`, `strconv`, `strings`; no new dependencies).
- **Unit tests** (`smtp_test.go`, same package, stdlib `testing`): in-test fake `Dialer`/`Client` record call order; cover message construction (plain, multipart, subject sanitization), unconfigured, no-recipients, happy-path call order, and each failure point (dial, rcpt, data). Unit tests must not import `cicd/`.
- **Cicd tests** (`cicd/email_delivery_test.go`): wrap `support.SMTPDialer` in a test-local adapter (`Dial() (smtp.Client, error)` — `*support.SMTPClient` already satisfies the interface). Use the package-level `mockSMTP`/`pool` variables from `cicd/email_queue_test.go`. **Single recipient per test** — the mock's `Data` parser captures the whole `To:` line as one string, so `HasEmailTo` fails for multi-recipient messages.
- **Fast discard technique:** direct DB insert with `max_retries=1, next_retry=NOW()` → discarded on the first worker tick (~100ms). The full `max_retries=3` path takes ~60s of backoff — too slow for cicd.
- **Manual relay verification:** after wiring, verify once against a real relay (STARTTLS + AUTH) by starting the app with a relay config and sending a test email through the API. Unit/cicd tests cannot exercise real TLS/AUTH.
- **Known limitation (documented, not fixed):** `processNextBatch`'s `FOR UPDATE SKIP LOCKED` runs in pgx autocommit mode — the row lock is released when the SELECT statement ends, before `sendJob` deletes the row. With **two API replicas** sharing one database, both replicas can pick the same job → double delivery. Single-process deployments (current setup, one goroutine, sequential ticks) are safe. Fixing this requires an explicit transaction spanning select→delete; tracked as a follow-up.

---

## 8. Open Questions

1. **Cross-replica double delivery** — acceptable for v1 (single-process deployment). If horizontal scaling of the API server is ever needed, wrap select→send→delete in one transaction (or move to a queue-based claim with lease expiry).
2. **Webhook worker dead wiring** — `NewWebhookWorker` has zero production callers (same class of bug). Out of scope here; needs its own spec/plan.
3. **`starttls: false`** — supported for unencrypted VPC relays (port 25). If a future deployment requires opportunistic-only or mTLS, the `tls.Config` in `netDialer` is the single extension point.
4. **Recipient display in `To:` header** — all recipients are visible to all (no BCC). Acceptable for v1 notification emails; BCC is a candidate follow-up if privacy requirements emerge.

---

## 9. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Credential leakage (password in config file) | Env override `AUTOMATA_SMTP_PASSWORD` recommended for production; config file is git-ignored in deployment |
| MitM on relay connection | STARTTLS enabled by default; `tls.Config{ServerName: host}` validates the certificate against the configured host |
| SMTP header injection via subject | Subject newlines (`\n`, `\r`) replaced with spaces before header emission |
| Credential leakage in logs | `netDialer` and `SMTPMailer` log no credentials; only error messages and host |
| Silent mail loss | Unconfigured SMTP fails loudly (startup warning + per-attempt error logs + discard log) |
| Body/header confusion | Body content only ever appears below the header block / inside multipart parts |