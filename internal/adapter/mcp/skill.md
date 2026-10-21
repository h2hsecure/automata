# Automata MCP

MCP server for managing a single Automata tenant: contacts, forms, banners, placements, campaigns, tracking, email, and webhooks.

## Connecting

- Endpoint: `POST <base>/api/mcp` (JSON-RPC over MCP streamable HTTP, stateless).
- Auth: `Authorization: Bearer <API key>`. Create keys in the web UI under Settings → API Keys.
- One API key = one tenant. All tools are scoped to that tenant; no tool takes a `context_id`.
- Skill file: `GET /api/mcp-skill.md` (same API key auth).

## Paging

List tools accept `page` (1-based) and `limit` (default 20, max 100). Paged responses are objects: `items`, `total`, `page`, `limit`, `has_more`.

## Errors

Tool failures come back as MCP tool errors (`isError: true`) with the message in the text content — read it.

## Contacts

- `contacts_list` — list contacts. Args: `page`, `limit`, `search`.
- `contacts_get` — get one contact. Args: `contact_id`.
- `contacts_create` — create a contact. Args: `email` (required), `first_name`, `last_name`, `company`, `phone`, `tags`, `custom_fields`, `source`.
- `contacts_update` — update a contact; provided fields only. Args: `contact_id` + contact fields.
- `contacts_delete` — delete a contact. Args: `contact_id`.
- `contacts_merge` — merge one contact into another. Args: `keep_contact_id`, `merge_contact_id`.
- `contacts_activity` — activity feed for a contact. Args: `contact_id`, `activity_type`, `page`, `limit`.

## Forms

- `forms_list` — list forms. Args: `page`, `limit`.
- `forms_get` — get a form by slug. Args: `form_id` (the slug).
- `forms_create` — create a form. Args: `slug`, `name`, `fields`, `settings`.
- `forms_update` — update a form by ID. Args: `form_id` (the form ID) + fields.
- `forms_delete` — delete a form by ID. Args: `form_id` (the form ID).
- `forms_submit` — submit data to a form. Args: `form_id` (the slug), `data` (field values).
- `form_submissions_list` — list submissions for a form. Args: `form_id` (the form ID), `page`, `limit`.
- `form_submissions_get` — get one submission. Args: `submission_id`.

## Banners

- `banners_list` — list banners. Args: `page`, `limit`, `is_active` (when `true`, only active banners).
- `banners_get` — get a banner. Args: `banner_id`.
- `banners_create` — create a banner. Args: `name` (required), `image_url`, `alt_text`, `link_url`, `is_active`.
- `banners_update` — update a banner; provided fields only. Args: `banner_id` + banner fields.
- `banners_delete` — delete a banner. Args: `banner_id`.

## Placements

- `placements_list` — list placements. Args: `page`, `limit`.
- `placements_get` — get a placement. Args: `placement_id`.
- `placements_create` — create a placement. Args: `name` (required), `css_selector` (required), `max_banners`, `is_active`.
- `placements_update` — update a placement; provided fields only. Args: `placement_id` + placement fields.
- `placements_delete` — delete a placement. Args: `placement_id`.

## Campaigns

- `campaigns_list` — list campaigns. Args: `page`, `limit`, `is_active`.
- `campaigns_get` — get a campaign. Args: `campaign_id`.
- `campaigns_create` — create a campaign. Args: `name` (required), `placements` (placement IDs), `start_date`, `end_date`, `is_active`.
- `campaigns_update` — update a campaign; provided fields only. Args: `campaign_id` + campaign fields.
- `campaigns_delete` — delete a campaign. Args: `campaign_id`.

## Tracking

- `tracking_dashboard` — aggregate metrics (page views, visitors, top pages/events). Args: `start`, `end`, `top_n`, `sort`.
- `tracking_events` — list tracked events. Args: `start`, `end`, `event_name`, `type`, `url`, `page`, `limit`.
- `tracking_visitors` — list visitors. Args: `page`, `limit`, `active_min`.
- `tracking_visitor_events` — events for one visitor. Args: `visitor_id` (from `tracking_visitors`), `page`, `limit`.
- `tracking_visitor_get` — visitor detail (profile + recent events). Args: `visitor_id`.

## Email

- `email_templates_list` — list email templates.
- `email_templates_get` — get a template. Args: `template_id`.
- `email_templates_create` — create a template. Args: `key` (required), `subject` (required), `body_text`, `body_html` (at least one body), `is_default`, `kind` (`standalone`/`form`), `form_id` (required when `kind` is `form`).
- `email_templates_update` — update a template; provided fields only. Args: `template_id` + template fields (`form_id: null` clears it).
- `email_templates_delete` — delete a template. Args: `template_id`.
- `email_send` — render a template and enqueue delivery. Args: `template` (key), `to`/`to_array`, `variables`, `subject_override`, `body_override`, `html`. Returns the job ID.
- `email_send_to_contact` — send a standalone template to a contact, using the contact's fields as variables. Args: `template_id`, `contact_id`. Returns the job ID.
- `email_test_send` — render a template and return the subject and body preview without storing delivery history. Args: `to`, `template` (key), `variables`.

Templates use Go `text/template` syntax with dot fields, e.g. `Hello {{ .name }}`.

## Webhooks

- `webhooks_list` — list webhook endpoints (secrets masked; `secret_set` shows presence).
- `webhooks_get` — get an endpoint. Args: `webhook_id`.
- `webhooks_create` — create an endpoint. Args: `name`, `url`, `events` (e.g. `form.submitted`), `secret` (8–256 chars), `active`.
- `webhooks_update` — update an endpoint; provided fields only. Args: `webhook_id` + endpoint fields.
- `webhooks_delete` — delete an endpoint. Args: `webhook_id`.
- `webhooks_test` — send a test delivery and return the delivery message. Args: `webhook_id`.