# Design: Configurable Banners, Pages, and Per-Visitor Decisions

## Problem Statement

Site owners need to show legal information (cookie policy, privacy, etc.) on
their websites and record how each visitor responded. Rather than building a
special "cookie consent" feature, the generic banner system is extended with
configurable display/behavior logic:

- **Position**: `top`, `bottom`, or `fullpage` (opaque overlay)
- **Buttons** with named actions; each button declares whether it dismisses
  the banner
- **Pages**: per-context editable HTML pages served publicly, linkable from
  banner buttons (or rendered inline by full-page banners)
- **Per-visitor decisions**: every button action is recorded per visitor and
  viewable in the dashboard

The legal/cookie use case is then just one *configuration*: a full-page
banner with `Accept` (dismisses), `Decline` (does not dismiss → user can't
see the page), and a `Cookie Policy` button linking to a page. No legal-specific
code path exists anywhere.

## Goals

1. Extend the banner model with `position` and `buttons` (label, named
   action, optional link, `dismisses` flag)
2. New per-context `Page` entity (slug, title, HTML) with dashboard CRUD and
   a public `GET /page/:contextID/:slug` endpoint on the tracking server
3. Public `GET /banner/config/:contextID` endpoint returning active banners
   with full rendering config (buttons, position, optional inline page
   content)
4. Public `POST /banner/decision/:contextID` endpoint recording
   `{ banner_id, action, visitor_id }` per visitor (plus ip_hash, user_agent)
5. Snippet renders banners by position, wires buttons, persists the decision
   in a local cookie (site domain), and skips already-decided banners
6. Dashboard: banner editor (position + button builder), Pages view
   (CRUD + HTML editor), per-banner decision counts and recent-decisions list

## Non-Goals

- Gating the existing tracking snippet on consent (tracking continues as
  today; decisions are recorded as facts — explicit owner decision)
- Changing placement/CSS-selector injection (existing banners keep working;
  position is additive)
- Server-side rendering, i18n, multi-language pages
- Frequency capping / scheduling beyond existing banner date fields

## Model

### Banner (extended)

```go
type Button struct {
    ID       string
    Label    string
    Action   string // free name: "accept", "decline", "close", ...
    LinkURL  string // optional: page slug (/cookie-policy) or absolute URL
    Dismisses bool  // this button closes the banner
}

// New fields on Banner:
Position  string     // "top" | "bottom" | "fullpage" ("" = legacy: placement injection)
Buttons   []Button   // JSONB
```

Storage: `position VARCHAR(16) NOT NULL DEFAULT ''`,
`buttons JSONB NOT NULL DEFAULT '[]'` on `banner_banners` (additive migration).

### Page (new)

```go
type Page struct {
    ID        string
    ContextID string
    Slug      string  // unique per context: "cookie-policy", "privacy"
    Title     string
    Content   string  // HTML
    IsActive  bool
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

Storage: `page_pages` table, `UNIQUE(context_id, slug)`,
`slug VARCHAR(128)`, `title VARCHAR(255)`, `content TEXT`,
`is_active BOOLEAN DEFAULT TRUE`.

### Decision (new)

```go
type Decision struct {
    ID         string
    ContextID  string
    BannerID   string
    VisitorID  string
    Action     string
    IPHash     string
    UserAgent  string
    CreatedAt  time.Time
}
```

Storage: `banner_decisions` table,
index on `(context_id, banner_id)` and `(context_id, visitor_id)`.

## API

### Dashboard (auth, `/api` prefix)

- `GET/POST /context/pages`, `GET/PUT/DELETE /context/pages/:id` — CRUD
- `GET /context/banners/:id/decisions` — decision list for a banner
  (page/limit), plus aggregate counts by action
- Banner create/update payloads accept `position` and `buttons`

### Tracking server (public, no auth — same trust model as existing
`/track/banner/*` and `/form/*`)

- `GET /banner/config/:contextID` →
  `{ banners: [{ id, name, position, content, buttons: [...], page: <inline
  HTML or null> }] }` for active banners in date window
- `POST /banner/decision/:contextID` body `{ banner_id, action, visitor_id }`
  → 200/204; stores decision + increments banner decision counter
- `GET /page/:contextID/:slug` → page HTML (`Content-Type: text/html`)

## Snippet behavior (pkg/snippet, generic)

1. Fetch `/banner/config/:contextID`
2. For each banner: if local cookie `ab_<bannerID>` exists → skip (already
   decided; the snippet runs in the customer's page, so `document.cookie`
   is the site's domain)
3. Render by `position`:
   - `top` / `bottom`: fixed bar, `position:fixed`, `left:0;right:0`,
     `top:0`/`bottom:0`, high `z-index`
   - `fullpage`: fixed opaque overlay covering the viewport
4. Content: `banner.content` (HTML) plus inline `page` HTML when present
5. Buttons: on click → `POST /banner/decision/...` (fetch, keepalive,
   text/plain) → set `ab_<bannerID>=<action>` cookie (365d, SameSite=Lax)
   → if `dismisses`, remove the banner element; otherwise keep it visible
6. Log impression (existing `banner_id + visitor_id` pattern) when a banner
   is shown

Backward compatibility: banners with empty `position` and no buttons render
exactly as today (placement injection, link click tracking).

## Dashboard UI (Vue)

- `BannersView.vue`: editor gains a position select
  (legacy/top/bottom/fullpage) and a button builder (label, action name,
  link URL, "dismisses" toggle); decisions section (counts by action +
  recent table: visitor, action, time)
- New `PagesView.vue` at `/pages`: list + create/edit (slug, title, HTML
  textarea) + delete; route + sidebar entry

## Data flow (legal case = one configuration)

```
owner: PagesView → create page "cookie-policy"
owner: BannersView → fullpage banner, buttons:
        [Accept: action=accept, dismisses=true]
        [Decline: action=decline, dismisses=false]
        [Cookie Policy: link=/cookie-policy]
visitor: page loads → snippet fetches config → overlay with legal content
visitor: Decline → decision recorded, overlay stays (can't see anything)
visitor: Accept → decision recorded, overlay closes, cookie set, never again
owner: BannersView → sees accept/decline counts + per-visitor log
```

## Risks / notes

- All additive: existing banner rows (`position=''`, `buttons=[]`) render as
  before; no data migration of existing rows needed
- Decision endpoint trust model matches existing public tracking endpoints
  (anonymous visitor_id, ip_hash stored, no PII)
- `banner_decisions` grows with traffic; same class as `tracking_events`
  (no special retention beyond what the project already does)
- Snippet fetches config once per page load (existing pattern)