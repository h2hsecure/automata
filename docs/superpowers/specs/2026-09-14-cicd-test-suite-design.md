# Design: CICD Integration Test Suite

## Problem Statement

The codebase has grown to 95+ Go files across domain, infrastructure, use case, and adapter layers, but lacks a comprehensive integration test suite. Without integration tests, changes to database schemas, API handlers, or infrastructure can break functionality without detection. Unit tests alone cannot verify database interactions, HTTP handler behavior, email worker processing, or webhook delivery. A shared test infrastructure with PostgreSQL test containers, in-memory mock SMTP, and embedded HTTP servers is needed to validate the full stack in CI/CD pipelines.

## Goals

1. Create a Ginkgo v2 test suite with shared PostgreSQL test container lifecycle
2. Test all repository layers: context, auth, contact, form, banner, tracking
3. Test email queue worker processing with in-memory mock SMTP server
4. Test webhook delivery with embedded httptest.Server
5. Test API handlers via Gin test context
6. Test snippet generation and validation logic
7. Achieve fast execution: all tests complete in under 5 seconds
8. Provide Makefile target `make cicd-tests` for CI/CD integration

## Non-Goals

- End-to-end browser testing (handled by Playwright separately)
- Load/stress testing
- Property-based testing
- Test coverage reporting (though >80% is the target)
- Mocking database interactions (real PostgreSQL via testcontainers)

## Architecture

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                    CICD Test Suite Architecture                       │
│                                                                       │
│  cicd/                                                               │
│  ├── suite_test.go      # Ginkgo BeforeSuite/AfterSuite             │
│  ├── support/                                           │
│  │   ├── testdb.go       # Test container + pgxpool + migration     │
│  │   ├── smtp_mock.go    # In-memory SMTP server                     │
│  │   ├── webhook_mock.go # Embedded httptest.Server                  │
│  │   └── fixtures.go     # Test data generators                      │
│  ├── email_queue_test.go # Email worker: enqueue, process, retry    │
│  ├── webhook_queue_test.go # Webhook: enqueue, deliver, retry       │
│  ├── banner_repo_test.go  # Banner CRUD, campaign, stats            │
│  ├── tracking_repo_test.go # Visitor, event, analytics              │
│  ├── contact_repo_test.go # Contact CRUD, tags, search, merge       │
│  ├── form_repo_test.go    # Form CRUD, submissions, validation       │
│  ├── snippet_test.go      # JS snippet generation                    │
│  ├── validation_test.go   # Field validation logic                    │
│  ├── handler_test.go      # API handler integration tests            │
│  ├── context_repo_test.go # Context CRUD, isolation                  │
│  ├── config_repo_test.go  # Config CRUD                              │
│  ├── config_handler_test.go # Config API handler                     │
│  ├── context_http_test.go   # Context HTTP handler                   │
│  ├── contact_http_test.go   # Contact HTTP handler                   │
│  ├── form_http_test.go      # Form HTTP handler                      │
│  ├── tracking_http_test.go  # Tracking HTTP handler                  │
│  ├── tracking_server_test.go  # Tracking server                      │
│  ├── banner_campaign_http_test.go # Banner campaign HTTP            │
│  ├── contact_form_integration_test.go # Form→contact flow           │
│  └── global_login_test.go    # End-to-end login flow                │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │  Test PostgreSQL Container (shared lifecycle)                   │  │
│  │  - Migration runner loads all .sql files from migration dirs    │  │
│  │  - pgxpool available for all tests                              │  │
│  └─────────────────────────────────────────────────────────────────┘  │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │  Mock SMTP Server (in-memory)                                   │  │
│  │  - Captures sent emails                                         │  │
│  │  - Start()/Stop()/Emails()/Clear() API                          │  │
│  └─────────────────────────────────────────────────────────────────┘  │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │  Mock Webhook Server (httptest)                                 │  │
│  │  - Captures POST requests                                       │  │
│  │  - URL()/Requests()/Clear()/Close() API                         │  │
│  └─────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────┘
```

### Test Infrastructure

#### Test Database (`cicd/support/testdb.go`)

- Launches a PostgreSQL test container via `testcontainers-go`
- Runs migrations from all directories: `migration.sql`, `migration_contexts.sql`, `migration_users.sql`, plus feature-specific migrations
- Provides `*pgxpool.Pool` to all tests
- Shared singleton lifecycle (BeforeSuite / AfterSuite)

#### Mock SMTP (`cicd/support/smtp_mock.go`)

- In-memory SMTP server that captures emails without external dependencies
- Provides `Start()`, `Stop()`, `Emails()`, `Clear()` methods
- Used by email worker tests to verify email content and delivery

#### Mock Webhook (`cicd/support/webhook_mock.go`)

- Embedded `httptest.NewServer()` for webhook receiver testing
- Captures POST requests with request body and headers
- Provides `URL()`, `Requests()`, `Clear()`, `Close()` methods
- Used by webhook queue tests to verify delivery and retry behavior

#### Fixtures (`cicd/support/fixtures.go`)

- Test data generators for contacts, forms, events, visitors
- `NewTestTenantID()` generates unique context UUIDs per test
- Pre-built fixture functions: `NewTestContact()`, `NewTestForm()`, `NewTestEvent()`

### Test Organization

Tests are organized by layer and feature:

| Category | Files | Purpose |
|----------|-------|---------|
| Infrastructure | `email_queue_test.go`, `webhook_queue_test.go` | Queue worker processing, retry logic |
| Repository | `banner_repo_test.go`, `tracking_repo_test.go`, `contact_repo_test.go`, `form_repo_test.go`, `context_repo_test.go`, `config_repo_test.go` | CRUD, filters, cascading deletes |
| Logic | `snippet_test.go`, `validation_test.go` | Pure function tests |
| HTTP Handlers | `handler_test.go`, `contact_http_test.go`, `form_http_test.go`, `tracking_http_test.go`, `context_http_test.go`, `config_handler_test.go`, `config_repo_test.go`, `banner_campaign_http_test.go`, `contact_form_integration_test.go`, `global_login_test.go` | Full request/response cycle with database |

### Test Execution

```bash
# Run all tests
go test ./cicd/... -v

# Run with Ginkgo CLI
ginkgo ./cicd/...

# Makefile target
make cicd-tests
```

Test execution uses `ginkgo`'s parallel flag for speed: `ginkgo -p ./cicd/...`.

### Known Fixes Applied

| Fix | File | Description |
|-----|------|-------------|
| Email worker scan error | `email_worker.go` | Changed `TEXT[]` scan to use `pgtype.Array[string]` |
| Email worker goroutine | `email_worker.go` | Wrapped `StartWorker` in `go func()` for async execution |
| Ticker interval | `email_worker.go` | Reduced from 5s to 100ms for faster test execution |
| Webhook ErrorMsg type | `webhook_queue.go` | Changed from `string` to `*string` for NULL handling |
| Banner cascade delete | `migration.sql` | Added FK constraints with `ON DELETE CASCADE` |
| Pagination SQL | Multiple | Removed `ORDER BY` from `COUNT(*)` queries |
| Migration loading | `testdb.go` | Load all `.sql` files from migration directories |
| Test cleanup | Multiple | Fixed foreign key constraint violations in AfterEach hooks |
| Context cancellation | `email_queue_test.go` | Added proper context cancellation for worker tests |
| Type assertions | `snippet_test.go` | Fixed float64 type assertions for options |

## Data Model (Tested Tables)

All existing tables are tested via this suite:

```sql
-- Context platform
contexts, context_users, api_keys, admin_configs

-- Email queue
email_jobs (status: pending, sending, sent, failed)

-- Webhook queue
webhook_deliveries (status: pending, retrying, delivered, failed)

-- Contacts
contacts, contact_tags, contact_tag_memberships, contact_field_definitions

-- Forms
forms, form_submissions

-- Banners
banner_banners, banner_placements, banner_campaigns,
banner_impressions, banner_clicks

-- Tracking
tracking_visitors, tracking_events
```

## API Contracts (Tested Endpoints)

| Method | Endpoint | Tested In |
|--------|----------|-----------|
| `POST` | `/api/auth/login` | `global_login_test.go`, `handler_test.go` |
| `GET` | `/api/auth/me` | `handler_test.go` |
| `POST` | `/api/auth/logout` | `handler_test.go` |
| `GET/POST/PUT/DELETE` | `/api/contexts` | `context_http_test.go`, `handler_test.go` |
| `GET/POST/PUT/DELETE` | `/api/users` | `handler_test.go` |
| `POST` | `/api/api-keys` | `handler_test.go` |
| `GET/PUT` | `/api/admin/configs/:id` | `config_handler_test.go` |
| `GET/POST` | `/api/contacts` | `contact_http_test.go`, `handler_test.go` |
| `POST` | `/api/forms/:slug/submit` | `form_http_test.go` |
| `GET` | `/api/forms/:slug` | `form_http_test.go` |
| `POST` | `/api/tracking` | `tracking_http_test.go` |
| `GET` | `/api/banners/:placement` | `banner_campaign_http_test.go` |

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Test framework | Ginkgo v2 + Gomega | Existing project standard; expressive DSL; table-driven tests |
| Database | Real PostgreSQL via testcontainers | No mocks for DB; tests validate real SQL and migrations |
| SMTP mock | In-memory server | No external dependencies; captures email content exactly |
| Webhook mock | httptest.Server | Standard library; captures request body and headers |
| Shared container | Single lifecycle | Fast startup; all tests share one container |
| Migration loading | All .sql files | Feature-specific migrations auto-loaded; no manual SQL execution |
| Execution time target | < 5 seconds | Developer experience; CI/CD fast feedback |
| Test isolation | AfterEach cleanup | Each test cleans its data; no cross-test interference |
