# Email Templates, Contact Email Tracking, and IMAP Inbox — Design

Date: 2026-10-19
Status: Approved in conversation (design presented, user confirmed)

## Overview

Extend Automata with email marketing capabilities per website (context):

1. **Email templates** — two kinds: *standalone* (sent manually to a contact) and *form-integrated* (auto-sent to the submitter when a bound form is submitted). Both render from Go `text/template` placeholders using contact and form attributes.
2. **Contact email tracking** — every email sent to a contact is recorded as an `email_sent` activity on that contact.
3. **IMAP inbox** — each website's mailbox is polled via IMAP and its recent messages are readable in the UI.
4. **Dashboard cards** — email queue status and contact reach statistics.

**This extends the existing email template subsystem** (`email_templates` table, `internal/domain/email`, `internal/usecase/email`, `internal/infrastructure/email/repo`, admin CRUD routes at `/api/admin/email-templates`, 5 seeded default templates, Go `text/template` rendering, existing cicd spec `cicd/email_template_test.go`). No new template table, no new rendering engine.

All sending reuses the existing email queue (`email_jobs` + worker + per-context SMTP resolution). No new sending path, no bulk email, no open/click tracking.

## Non-Goals (YAGNI)

- No bulk/mass email (single contact per standalone send)
- No rich HTML editor (plain-text subject + body with placeholders)
- No open/click/track-pixel analytics
- No IMAP send/reply, no folder management (INBOX only)
- No per-app-user mailboxes (inbox is per website/context)
- No live IMAP coverage in the cicd suite (verified manually once against the real mailbox)

## Data Model

### `email_templates` (existing table — extended)

Existing columns (unchanged): `id`, `context_id`, `key` (display/identifier, unique per context), `subject`, `body_text`, `body_html`, `is_default`, `created_at`, `updated_at`.

New columns (idempotent `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`):

| Column | Type | Notes |
|---|---|---|
| kind | text NOT NULL DEFAULT `'standalone'` | `standalone` or `form` |
| form_id | uuid NULL | required when kind = `form`, NULL for `standalone` |

Seeded default templates keep `kind = 'standalone'` (column default).

### `email_inbox`

| Column | Type | Notes |
|---|---|---|
| id | uuid PK | |
| context_id | uuid | |
| uid | bigint | IMAP UID; unique with context_id |
| from | text | |
| to | text | |
| subject | text | |
| body_text | text | plain-text part only |
| message_date | timestamptz | |
| is_read | boolean | default false |
| created_at | timestamptz | first-seen time |

Unique constraint: `(context_id, uid)`.

### Activity

New type `ActivityEmailSent = "email_sent"` in `internal/domain/contact/activity.go`.
`Data`: `subject` (string), `template_name` (string).
Title: `Email sent: <subject>`; description: template name.

## Templates and Placeholders

Templates render with the existing Go `text/template` engine (`EmailUsecase.RenderTemplate`). The variables map is generalized from `map[string]string` to `map[string]interface{}` (backward compatible; nested maps enable dotted lookups). Missing variables render as `<no value>` (existing Go template behavior — visible to the sender as a reminder they forgot a field).

**Standalone templates** (contact `c`):

| Variable | Value |
|---|---|
| `{{ .first_name }}` | `c.FirstName` |
| `{{ .last_name }}` | `c.LastName` |
| `{{ .email }}` | `c.Email` |
| `{{ .company }}` | `c.Company` |
| `{{ .phone }}` | `c.Phone` |
| `{{ .custom.<field> }}` | `c.CustomFields[field]` (nested map) |

**Form templates** (submission `s` of form `f`, contact `c` looked up by `s`'s email if present):

| Variable | Value |
|---|---|
| `{{ .form.<field> }}` | `s.Data[field]` (nested map) |
| `{{ .form_name }}` | `f.Name` |
| `{{ .submit_date }}` | `s.CreatedAt` (RFC 1123) |
| contact variables | same as standalone, when a contact matches the submission email (empty when none) |

## Send Flows

### Standalone

`POST /api/admin/contexts/:id/email-templates/:tid/send` body `{"contact_id": "..."}` (admin group)

1. Load template (must be `standalone`, must belong to context) → 404 otherwise
2. Load contact (must belong to context) → 404 otherwise; contact must have an email → 400 if not
3. Render subject + body from contact variables
4. Enqueue via the existing email queue (per-context SMTP resolution, retries, history)
5. Log `email_sent` activity on the contact
6. Respond 202 `{"job_id": "..."}`

### Form-integrated

Hook in `FormHandler.SubmitForm` (`internal/adapter/api/handler/form_handler.go`) — the single point both the main API and the tracking server (`POST /form/submit/:slug`) converge on. After the submission is stored:

1. Load templates of kind `form` bound to the form (`form_id = submission's form`)
2. For each: determine recipient from the submission's email field (the form field whose type is `email`, first match). If the submission has no email value, skip that template (no error)
3. Render subject + body with submission data (+ contact variables if a contact matches the email, via `contactRepo.FindByEmail`)
4. Enqueue via the existing email queue
5. If a contact matches the email, log `email_sent` activity on it

Template send failures never fail the form submission (errors are logged). The `FormHandler` gains an optional email usecase dependency, wired in `server.go`/`main.go`.

## Inbox (IMAP)

**Poller** (started in `cmd/automata/main.go` alongside the email worker):

- Ticker: 60s
- For each context with resolvable SMTP credentials (same resolution as sending: config → global_settings → context_settings):
  - IMAP client (stdlib `net/smtp` is SMTP-only; use `github.com/emersion/go-imap`), connect to `host:993` (IMAPS), LOGIN with the SMTP username/password, EXAMINE `INBOX`
  - Fetch messages with UID greater than the max stored `uid` for that context (first run: last 50)
  - Insert rows (dedupe on `(context_id, uid)`), plain-text body only
  - Per-context errors are logged and skipped (one broken mailbox never blocks others)

**API** (admin group):

- `GET /api/admin/contexts/:id/inbox?limit=50` → `{"messages":[...],"count":n}` — newest first, fields: `id, from, to, subject, body_text, message_date, is_read, created_at`
- `POST /api/admin/contexts/:id/inbox/:mid/read` → 200 (sets `is_read = true`)

## Dashboard Cards

Two new cards on the existing Dashboard Overview tab, fed by one small stats endpoint:

- **Email queue card:** `pending` (email_jobs count), `sent_24h` and `failed_24h` (email_history by status in last 24h). Links to the per-website email panel.
- **Reach card:** `contacts_reached` (distinct contacts with an `email_sent` activity), `emails_sent_total` (email_history total), `reached_7d` (distinct contacts with `email_sent` in last 7 days).

Implementation: new context-resolved endpoint `GET /api/context/email-stats` (context-resolved like the tracking dashboard, so any authenticated user of the context can render the dashboard; the tracking dashboard usecase is a pure function over the tracking repo and is not coupled to email tables). The dashboard UI already aggregates multiple endpoints client-side, so this matches the existing pattern.

## UI

New **Email** page in the sidebar (between Forms and Settings), route `/email`, scoped to the active context:

- **Templates tab:** table (key, kind, form, updated) + "New template" button. Create/edit modal: key, kind (standalone/form), form picker (shown when kind=form), subject, body textarea, placeholder hint text. Standalone rows have a **Send** button → contact picker modal (search + select) → confirms and fires the send, shows the resulting job state.
- **Inbox tab:** message list (from, subject, date, unread badge), click to expand body, read marks the message read.

Contact detail: `email_sent` renders in the existing activity feed via the new title/description formatting.

## API Summary

Existing (extended with `kind` + `form_id` in request/response bodies):

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/admin/email-templates?context_id=...` | list templates |
| POST | `/api/admin/email-templates` | create |
| GET | `/api/admin/email-templates/:id` | get |
| PUT | `/api/admin/email-templates/:id` | update |
| DELETE | `/api/admin/email-templates/:id` | delete |
| POST | `/api/admin/email-templates/:id/test` | test send |

New:

| Method | Path | Purpose |
|---|---|---|
| POST | `/api/admin/contexts/:id/email-templates/:tid/send` | standalone send `{contact_id}` |
| GET | `/api/admin/contexts/:id/inbox?limit=N` | list inbox messages |
| POST | `/api/admin/contexts/:id/inbox/:mid/read` | mark read |
| GET | `/api/context/email-stats` | email queue + reach stats (context-resolved) |

Admin routes live in the existing `admin` group (same as settings and email-queue); the stats route is context-resolved (same pattern as the tracking dashboard). All routes follow the existing auth/middleware pattern.

## Testing

- **cicd specs** (existing `cicd/email_template_test.go` stays green; new specs added):
  - Template kind/form binding: create with kind/form_id, validation (kind=form requires form_id), seeded defaults are `standalone`
  - Standalone send: job appears in queue, `email_sent` activity recorded on contact, 404/400 paths
  - Form-integrated send: submitting a form with a bound template enqueues an email to the submitter; submission without email skips gracefully; template error does not fail submission
  - Inbox store: insert/dedupe on `(context_id, uid)`, list ordering, mark-read
  - Email stats: fields present and correct against seeded data
- **Manual:** one live check of the poller against the real mailbox (`hello@pdaccess.com` via `mail.h2hsecure.com:993`) after deploy
- **UI gate:** `vite build` (project does not pass clean `vue-tsc --noEmit`)

## Deployment

- Migrations: idempotent `ALTER TABLE email_templates ADD COLUMN IF NOT EXISTS ...` (kind, form_id) + new `email_inbox` table (`CREATE TABLE IF NOT EXISTS`), matching existing migration style
- New Go dependency: `github.com/emersion/go-imap`
- New image deployed to the `automata` namespace (staging — no production exists yet)
- No config required: IMAP host/port derived from the resolved SMTP config (host + 993)