# Automata — Project Roadmap

Automata is a self-hosted marketing automation tool for small-to-mid businesses. It helps users track website visitors, collect leads via forms, manage contacts, and display banners on their websites.

## Architecture

- Multi-tenant: one Automata instance manages multiple websites. Each website (tenant) has its own contacts, forms, tracking data, banners, and configuration.
- Reverse proxy: routes tenant traffic via subdomain or path.
- AI MCP interface: exposes all features via MCP protocol for AI tool integration.
- No UI in Phase 1 — focus on API, MCP, and backend layers.

## Feature Areas

### 1. Core Platform
Multi-tenant authentication, reverse proxy, MCP server foundation, shared infrastructure (logging, config, email queue).

### 2. Contact Management
Store and manage contacts collected via forms, tracking events, and manual entry. Deduplication, segmentation, import/export, activity timeline.

### 3. Form Management
Form builder, form rendering via reverse proxy, submission handling, webhook and email notifications, spam protection.

### 4. User Tracking
JavaScript snippet for website embedding, page view and event tracking, visitor identification, analytics dashboard data.

### 5. Banner Management
Banner CRUD, placement rules, A/B testing, impression and click tracking.

## Build Order (Sequential)

1. Core Platform — foundation for everything else
2. Contact Management — first feature, simplest domain
3. Form Management — builds on core, introduces email queue
4. User Tracking — separate driver adapter, shared with Banners
5. Banner Management — extends tracking, adds placement logic

## Testing Strategy

- Unit and integration tests using Ginkgo + testcontainers
- E2E tests for API and MCP layers in `cicd/` directory
- Mail and IMAP mocks for email/IMAP integration validation
