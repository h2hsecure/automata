# Context Rename and Login Simplification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Rename tenant to context throughout the codebase, restructure users from tenant-scoped to platform-level with context memberships, simplify login to email+password only, and add admin bootstrap on first startup.

**Architecture:** Phase 1 is a terminology rename (tenant to context) with schema column rename (tenant_id to context_id) and table rename (tenants to contexts). Phase 2 introduces the new users table, user_contexts join table, and rewrites the login flow. Phase 3 adds admin bootstrap. All phases maintain test compatibility.

**Tech Stack:** Go 1.26, PostgreSQL, Gin (HTTP), Ginkgo (testing), testcontainers, JWT (golang-jwt), bcrypt

**Spec:** docs/superpowers/specs/2026-09-22-context-rename-and-login-simplification.md

## Global Constraints

- Go version: 1.26.2
- PostgreSQL: 16+ (for gen_random_uuid())
- JWT library: github.com/golang-jwt/jwt/v5
- All tenant_id database columns become context_id
- All tenants database tables become contexts
- All tenant_users database tables become context_users
- API header X-Tenant-ID becomes X-Context-ID
- Route /api/admin/tenants/* becomes /api/admin/contexts/*
- Context key tenant_id becomes context_id in Gin context
- Context key tenant becomes context in Gin context
- JWT claim tenant_slug becomes context_slug
- MCP tool tenants.list becomes contexts.list
- JS snippet variable tenantId becomes contextId
- JS snippet header X-Tenant-ID becomes X-Context-ID

---

## Phase 1: Terminology Rename (tenant to context)

This phase renames all code, API, and database references from tenant to context without changing the data model structure. Users remain scoped to contexts (via context_users table, formerly tenant_users).

### Task 1.1: Rename Database Tables and Columns

**Files to modify:**
- internal/infrastructure/database/migration.sql
- internal/infrastructure/database/migration_contexts.sql (new)
- internal/infrastructure/contact/migration.sql
- internal/infrastructure/form/migration.sql
- internal/infrastructure/banner/repo/migration.sql
- internal/infrastructure/tracking/repo/migration.sql
- internal/infrastructure/queue/migration.sql
- internal/infrastructure/queue/webhook_migration.sql

**Steps:**

- [ ] **Step 1: Create migration file for table/column renames**

Create internal/infrastructure/database/migration_contexts.sql with SQL that:
- ALTER TABLE tenants RENAME TO contexts
- ALTER TABLE tenant_users RENAME TO context_users
- ALTER TABLE api_keys RENAME COLUMN tenant_id TO context_id
- ALTER TABLE admin_configs RENAME COLUMN tenant_id TO context_id
- ALTER TABLE context_users RENAME COLUMN tenant_id TO context_id
- Drop and recreate indexes with new names

- [ ] **Step 2: Update all other migration files to use context_id instead of tenant_id**

For each migration file, replace tenant_id with context_id in CREATE TABLE statements and index definitions.

- [ ] **Step 3: Run migration against test database to verify**

Run: docker compose down && docker compose up -d && sleep 5 && psql -U automata -d automata -f internal/infrastructure/database/migration.sql && psql -U automata -d automata -f internal/infrastructure/database/migration_contexts.sql

Expected: No errors, tables renamed successfully.

- [ ] **Step 4: Commit**

git add internal/infrastructure/database/migration*.sql internal/infrastructure/contact/migration.sql internal/infrastructure/form/migration.sql internal/infrastructure/banner/repo/migration.sql internal/infrastructure/tracking/repo/migration.sql internal/infrastructure/queue/migration.sql internal/infrastructure/queue/webhook_migration.sql
git commit -m "db: rename tenants to contexts, tenant_users to context_users, tenant_id to context_id"

### Task 1.2: Rename Domain Layer Package and Types

**Files to modify:**
- Rename: internal/domain/tenant/ to internal/domain/context/
- Modify: internal/domain/auth/auth.go
- Modify: internal/domain/tracking/event.go
- Modify: internal/domain/tracking/visitor.go
- Modify: internal/domain/contact/contact.go
- Modify: internal/domain/contact/tag.go
- Modify: internal/domain/banner/banner.go
- Modify: internal/domain/form/form.go
- Modify: internal/domain/config/config.go

**Steps:**

- [ ] **Step 1: Rename the directory and files**

mv internal/domain/tenant internal/domain/context

- [ ] **Step 2: Update package declarations and type names in renamed files**

In internal/domain/context/context.go:
- Change package tenant to package context
- Change type Tenant struct to type Context struct
- Update method receivers from t *Tenant to c *Context

In internal/domain/context/context_test.go:
- Change package tenant_test to package context_test
- Update all Tenant references to Context

- [ ] **Step 3: Update Repository interface**

In internal/domain/context/context_repository.go (rename from repository.go):
- Change Tenant to Context in all method signatures

- [ ] **Step 4: Update User struct - rename TenantID to ContextID**

In internal/domain/context/user.go:
- Change TenantID field to ContextID

- [ ] **Step 5: Update UserRepository interface**

In internal/domain/context/user_repository.go:
- Change GetByEmail(tenantID, email) to GetByEmail(contextID, email)

- [ ] **Step 6: Update domain auth types**

In internal/domain/auth/auth.go:
- Change TenantInfo to ContextInfo
- Change TenantID to ContextID in APIKey struct
- Update AuthService interface: Login returns contexts []ContextInfo
- Update VerifyToken returns contextID string
- Update CreateAPIKey parameter contextID string

- [ ] **Step 7: Update all domain entity structs - rename TenantID to ContextID**

For each file below, replace TenantID field with ContextID:
- internal/domain/tracking/event.go
- internal/domain/tracking/visitor.go
- internal/domain/contact/contact.go (also update Validate() error message)
- internal/domain/contact/tag.go
- internal/domain/banner/banner.go (Banner, Placement, Campaign, Impression, Click)
- internal/domain/form/form.go
- internal/domain/config/config.go

- [ ] **Step 8: Update all go.mod imports**

Find and replace all github.com/hekemen/automata/internal/domain/tenant imports with github.com/hekemen/automata/internal/domain/context across all Go files.

- [ ] **Step 9: Verify compilation**

Run: go build ./...

Expected: No errors.

- [ ] **Step 10: Commit**

git add internal/domain/
git commit -m "domain: rename tenant to context package, Tenant to Context types, TenantID to ContextID fields"

### Task 1.3: Rename Infrastructure Layer

**Files to modify:**
- Rename: internal/infrastructure/tenant/ to internal/infrastructure/context/
- Modify: internal/infrastructure/auth/repo/api_key_postgres.go
- Modify: internal/infrastructure/config/repo/postgres.go
- Modify: internal/infrastructure/tracking/repo/postgres.go
- Modify: internal/infrastructure/banner/repo/postgres.go
- Modify: internal/infrastructure/contact/repo/postgres.go
- Modify: internal/infrastructure/form/repo/postgres.go
- Modify: internal/infrastructure/queue/email_queue.go
- Modify: internal/infrastructure/queue/webhook_queue.go

**Steps:**

- [ ] **Step 1: Rename the directory and update imports**

mv internal/infrastructure/tenant internal/infrastructure/context

Update all imports from github.com/hekemen/automata/internal/infrastructure/tenant/repo to github.com/hekemen/automata/internal/infrastructure/context/repo.

- [ ] **Step 2: Update context repository implementations**

In internal/infrastructure/context/repo/postgres.go:
- Update all SQL queries: tenants to contexts, tenant_id to context_id
- Update struct field references: t.TenantID to t.ContextID

In internal/infrastructure/context/repo/user_postgres.go:
- Update all SQL queries: context_users, context_id
- Update struct field references: u.ContextID

- [ ] **Step 3: Update all repository implementations to use context_id**

For each repository file, replace tenant_id with context_id in SQL queries and tenantID with contextID in function parameters:
- internal/infrastructure/auth/repo/api_key_postgres.go
- internal/infrastructure/config/repo/postgres.go
- internal/infrastructure/tracking/repo/postgres.go
- internal/infrastructure/banner/repo/postgres.go
- internal/infrastructure/contact/repo/postgres.go
- internal/infrastructure/form/repo/postgres.go

- [ ] **Step 4: Update queue structs**

In internal/infrastructure/queue/email_queue.go:
- Change TenantID field to ContextID

In internal/infrastructure/queue/webhook_queue.go:
- Change TenantID field to ContextID

- [ ] **Step 5: Verify compilation**

Run: go build ./...

Expected: No errors.

- [ ] **Step 6: Commit**

git add internal/infrastructure/
git commit -m "infra: rename tenant to context, update all SQL queries to use context_id"

### Task 1.4: Rename Adapter Layer (Middleware, Handlers, Routes)

**Files to modify:**
- Rename: internal/adapter/api/tenant_middleware.go to context_middleware.go
- Rename: internal/adapter/api/tenant_middleware_test.go to context_middleware_test.go
- Rename: internal/adapter/api/handler/tenant_handler.go to context_handler.go
- Modify: internal/adapter/api/handler/auth_handler.go
- Modify: internal/adapter/api/handler/banner_handler.go
- Modify: internal/adapter/api/handler/config_handler.go
- Modify: internal/adapter/api/handler/contact_handler.go
- Modify: internal/adapter/api/handler/form_handler.go
- Modify: internal/adapter/api/server.go
- Modify: internal/adapter/api/middleware/auth.go
- Modify: internal/adapter/proxy/proxy.go
- Rename: internal/adapter/tracking/tenant_resolver.go to context_resolver.go
- Modify: internal/adapter/tracking/track_handler.go
- Modify: internal/adapter/tracking/snippet_handler.go
- Rename: internal/adapter/banner/tenant_resolver.go to context_resolver.go
- Modify: internal/adapter/mcp/server.go

**Steps:**

- [ ] **Step 1: Rename tenant_middleware.go to context_middleware.go**

In internal/adapter/api/context_middleware.go:
- Change TenantContextKey = "tenant" to ContextKey = "context"
- Change ErrNoTenant to ErrNoContext
- Rename TenantResolver to ContextResolver
- Update header check: X-Context-ID instead of X-Tenant-ID
- Update context setting: c.Set("context", *ctx) and c.Set("context_id", ctx.ID)

- [ ] **Step 2: Rename tenant_handler.go to context_handler.go**

In internal/adapter/api/handler/context_handler.go:
- Change TenantHandler to ContextHandler
- Change NewTenantHandler to NewContextHandler
- Update all method receivers and JSON response keys

- [ ] **Step 3: Update auth_handler.go**

In internal/adapter/api/handler/auth_handler.go:
- Update field types to use context.UserRepository and context.ContextRepository
- Update Login handler: remove tenant field from request body parsing
- Update response: contexts instead of tenants in JSON

- [ ] **Step 4: Update all handler files to use X-Context-ID**

Replace X-Tenant-ID with X-Context-ID and tenantID with contextID in:
- internal/adapter/api/handler/banner_handler.go
- internal/adapter/api/handler/config_handler.go
- internal/adapter/api/handler/contact_handler.go
- internal/adapter/api/handler/form_handler.go

- [ ] **Step 5: Update server.go routes**

In internal/adapter/api/server.go:
- Update imports to use context package
- Update function signature: contextRepo context.ContextRepository, userRepo context.UserRepository
- Update route registration: admin.Group("/contexts") instead of admin.Group("/tenants")
- Update config routes: :contextId instead of :tenantId
- Update middleware: ContextResolver(contextRepo) instead of TenantResolver(tenantRepo)

- [ ] **Step 6: Update auth middleware**

In internal/adapter/api/middleware/auth.go:
- Change c.Set("tenant_id", apiKey.TenantID) to c.Set("context_id", apiKey.ContextID)

- [ ] **Step 7: Update proxy**

In internal/adapter/proxy/proxy.go:
- Update import to use context package
- Update struct field: contextRepo context.ContextRepository
- Update context key: c.Get("context") instead of c.Get("tenant")

- [ ] **Step 8: Rename and update tracking tenant_resolver.go to context_resolver.go**

In internal/adapter/tracking/context_resolver.go:
- Rename ResolveTenant to ResolveContext
- Update header: X-Context-ID instead of X-Tenant-ID
- Rename tenantID to contextID

- [ ] **Step 9: Rename and update banner tenant_resolver.go to context_resolver.go**

In internal/adapter/banner/context_resolver.go:
- Rename ResolveTenant to ResolveContext
- Update header: X-Context-ID instead of X-Tenant-ID
- Rename tenantID to contextID

- [ ] **Step 10: Update MCP server**

In internal/adapter/mcp/server.go:
- Update import to use context package
- Update struct field: contextRepo context.ContextRepository
- Update tool registration: contexts.list instead of tenants.list

- [ ] **Step 11: Verify compilation**

Run: go build ./...

Expected: No errors.

- [ ] **Step 12: Commit**

git add internal/adapter/
git commit -m "adapter: rename tenant to context, X-Tenant-ID to X-Context-ID, routes /contexts/*"

### Task 1.5: Update Usecase Layer

**Files to modify:**
- internal/usecase/banner/banner.go
- internal/usecase/contact/create_contact.go
- internal/usecase/contact/list_contacts.go
- internal/usecase/contact/delete_contact.go
- internal/usecase/contact/update_contact.go
- internal/usecase/contact/merge_contacts.go
- internal/usecase/contact/get_contact.go
- internal/usecase/contact/get_activity.go
- internal/usecase/contact/tags.go
- internal/usecase/contact/segment_contacts.go
- internal/usecase/contact/export_contacts.go
- internal/usecase/contact/import_contacts.go
- internal/usecase/form/create_form.go
- internal/usecase/form/list_forms.go
- internal/usecase/form/update_form.go
- internal/usecase/form/delete_form.go
- internal/usecase/form/get_form.go
- internal/usecase/form/submit_form.go
- internal/usecase/form/list_submissions.go
- internal/usecase/form/get_submission.go
- internal/usecase/tracking/track_event.go
- internal/usecase/tracking/get_events.go
- internal/usecase/tracking/get_dashboard.go

**Steps:**

- [ ] **Step 1: Rename all tenantID parameters to contextID in usecase files**

For each file, replace tenantID parameter with contextID and update error messages.

- [ ] **Step 2: Verify compilation**

Run: go build ./...

Expected: No errors.

- [ ] **Step 3: Commit**

git add internal/usecase/
git commit -m "usecase: rename tenantID to contextID parameters in all functions"

### Task 1.6: Update Entry Point and Configuration

**Files to modify:**
- cmd/automata/main.go
- config.example.yaml
- internal/infrastructure/auth/service.go
- pkg/snippet/generator.go

**Steps:**

- [ ] **Step 1: Update main.go**

In cmd/automata/main.go:
- Update import to use context_repo
- Update variable names: contextRepo, userRepo
- Update SQL query: SELECT COUNT(*) FROM contexts
- Update NewServer call

- [ ] **Step 2: Update auth service**

In internal/infrastructure/auth/service.go:
- Update import to use context package
- Update struct field: userRepo context.UserRepository
- Update Login: build ContextInfo list from memberships
- Update response: contexts instead of tenants
- Update VerifyToken: rename tenantSlug to contextSlug
- Update CreateAPIKey: rename tenantID to contextID

- [ ] **Step 3: Update snippet generator**

In pkg/snippet/generator.go:
- Update function signatures: contextID instead of tenantID
- Update JS output: X-Context-ID header, contextId variable

- [ ] **Step 4: Update config.example.yaml**

In config.example.yaml:
- Rename keys: context.resolution, context.default_path
- Add auth.admin_email

- [ ] **Step 5: Verify compilation**

Run: go build ./...

Expected: No errors.

- [ ] **Step 6: Commit**

git add cmd/automata/main.go internal/infrastructure/auth/service.go pkg/snippet/generator.go config.example.yaml
git commit -m "main: update entry point, auth service, snippet generator for context rename"

### Task 1.7: Update Tests

**Files to modify:**
- Rename: cicd/tenant_http_test.go to context_http_test.go
- Rename: cicd/tenant_repo_test.go to context_repo_test.go
- Modify: cicd/support/fixtures.go
- Modify: cicd/banner_campaign_http_test.go
- Modify: cicd/tracking_http_test.go
- Modify: cicd/form_http_test.go
- Modify: cicd/contact_http_test.go
- Modify: cicd/handler_test.go
- Modify: internal/adapter/api/context_middleware_test.go
- Modify: internal/infrastructure/context/repo/postgres_test.go

**Steps:**

- [ ] **Step 1: Rename test files and update all references**

Rename cicd/tenant_http_test.go to cicd/context_http_test.go:
- Change package tenant_test to package context_test
- Change NewTestTenantID() to NewTestContextID()
- Change NewTestTenant() to NewTestContext()
- Change tenant_id map keys to context_id
- Change X-Tenant-ID header to X-Context-ID
- Change /api/admin/tenants routes to /api/admin/contexts

- [ ] **Step 2: Update fixtures**

In cicd/support/fixtures.go:
- Change NewTestTenantID() to NewTestContextID()
- Change NewTestTenant() to NewTestContext()

- [ ] **Step 3: Update all test files**

For each test file, replace:
- tenant_id with context_id in map keys
- tenantID with contextID in variable names
- X-Tenant-ID with X-Context-ID in headers
- Tenant with Context in type names
- tenants with contexts in route paths

- [ ] **Step 4: Run tests**

Run: go test ./cicd/... -v

Expected: All tests pass.

- [ ] **Step 5: Commit**

git add cicd/ internal/adapter/api/context_middleware_test.go internal/infrastructure/context/repo/postgres_test.go
git commit -m "tests: rename tenant to context references in all test files"

---

## Phase 2: Platform-Level Users with Context Memberships

This phase introduces the new users table (platform-level accounts), the user_contexts join table, and rewrites the login flow to be email+password only.

### Task 2.1: New Database Schema

**Files to create/modify:**
- internal/infrastructure/database/migration_users.sql (new)
- internal/infrastructure/database/migration.sql (update)

**Steps:**

- [ ] **Step 1: Create migration for users and user_contexts tables**

Create internal/infrastructure/database/migration_users.sql with:
- CREATE TABLE users (id UUID, email VARCHAR UNIQUE, password_hash TEXT, is_admin BOOLEAN, is_active BOOLEAN)
- CREATE TABLE user_contexts (id UUID, user_id UUID, context_id UUID, role TEXT, UNIQUE(user_id, context_id))
- Indexes on user_contexts(user_id) and user_contexts(context_id)
- Migration script to migrate existing context_users to users + user_contexts

- [ ] **Step 2: Update migration.sql to include new schema**

Add the users and user_contexts tables to migration.sql (the initial setup script).

- [ ] **Step 3: Update migration.sql to remove old context_users table**

DROP TABLE IF EXISTS context_users;

- [ ] **Step 4: Verify migration**

Run migration against test database.

- [ ] **Step 5: Commit**

git add internal/infrastructure/database/migration_users.sql internal/infrastructure/database/migration.sql
git commit -m "db: add users table, user_contexts join table, remove context_users"

### Task 2.2: New Domain Types for Platform Users

**Files to create/modify:**
- internal/domain/context/user.go (rewrite)
- internal/domain/context/user_repository.go (rewrite)
- internal/domain/context/context_membership.go (new)
- internal/domain/context/context_membership_repository.go (new)
- internal/domain/auth/auth.go (update)

**Steps:**

- [ ] **Step 1: Rewrite User struct in domain**

In internal/domain/context/user.go:
- User represents a platform-level user account (no context_id)
- Add ContextMembership struct with UserID, ContextID, Role

- [ ] **Step 2: Rewrite UserRepository interface**

In internal/domain/context/user_repository.go:
- GetByEmail(email string) - no contextID parameter
- Add ContextMembershipRepository interface

- [ ] **Step 3: Update auth domain types**

In internal/domain/auth/auth.go:
- Add BootstrapAdmin(email, password string) error to AuthService interface

- [ ] **Step 4: Commit**

git add internal/domain/context/user.go internal/domain/context/user_repository.go
git add internal/domain/context/context_membership.go internal/domain/context/context_membership_repository.go
git add internal/domain/auth/auth.go
git commit -m "domain: add platform User, ContextMembership types, update auth interface"

### Task 2.3: New Repository Implementations

**Files to create/modify:**
- internal/infrastructure/context/repo/user_postgres.go (rewrite)
- internal/infrastructure/context/repo/context_membership_postgres.go (new)
- internal/infrastructure/auth/repo/api_key_postgres.go (update)

**Steps:**

- [ ] **Step 1: Rewrite user_postgres.go**

In internal/infrastructure/context/repo/user_postgres.go:
- INSERT INTO users (id, email, password_hash, is_admin, is_active)
- SELECT from users WHERE email = 

- [ ] **Step 2: Create context_membership_postgres.go**

In internal/infrastructure/context/repo/context_membership_postgres.go:
- INSERT INTO user_contexts (id, user_id, context_id, role)
- SELECT from user_contexts WHERE user_id = 

- [ ] **Step 3: Update api_key_postgres.go**

In internal/infrastructure/auth/repo/api_key_postgres.go:
- Update foreign key references: user_id now references users(id)

- [ ] **Step 4: Commit**

git add internal/infrastructure/context/repo/user_postgres.go
git add internal/infrastructure/context/repo/context_membership_postgres.go
git add internal/infrastructure/auth/repo/api_key_postgres.go
git commit -m "infra: implement user and context_membership repositories"

### Task 2.4: Rewrite Auth Service

**Files to modify:**
- internal/infrastructure/auth/service.go (rewrite)

**Steps:**

- [ ] **Step 1: Rewrite Login method**

In internal/infrastructure/auth/service.go:
- Find user by email only (no contextID)
- Verify password
- Get user contexts from user_contexts join table
- Build ContextInfo list from memberships
- Generate JWT with context_slug claim

- [ ] **Step 2: Update VerifyToken**

- Parse JWT, extract userID and contextID
- Validate context membership exists

- [ ] **Step 3: Add BootstrapAdmin**

- Check if any users exist
- If not, create admin user with random password
- Log credentials to stdout
- Create marker file .admin_initialized

- [ ] **Step 4: Commit**

git add internal/infrastructure/auth/service.go
git commit -m "auth: rewrite login for platform users, add BootstrapAdmin"

### Task 2.5: Update Auth Handler and Middleware

**Files to modify:**
- internal/adapter/api/handler/auth_handler.go
- internal/adapter/api/server.go
- internal/adapter/api/middleware/auth.go

**Steps:**

- [ ] **Step 1: Update auth_handler.go**

In internal/adapter/api/handler/auth_handler.go:
- Remove tenant field from LoginRequest struct
- Update Login handler to call platform-level login
- Update response to include contexts list

- [ ] **Step 2: Update server.go**

In internal/adapter/api/server.go:
- Update NewServer to use new userRepo type
- Update authHandler constructor

- [ ] **Step 3: Update auth middleware**

In internal/adapter/api/middleware/auth.go:
- Update context key to use context_id
- Validate context membership for the request

- [ ] **Step 4: Commit**

git add internal/adapter/api/handler/auth_handler.go internal/adapter/api/server.go internal/adapter/api/middleware/auth.go
git commit -m "adapter: update auth handler and middleware for platform users"

### Task 2.6: Update Context Handler for Membership Management

**Files to modify:**
- internal/adapter/api/handler/context_handler.go
- internal/usecase/context/* (new)

**Steps:**

- [ ] **Step 1: Add context membership management**

Create usecase for managing user-context memberships:
- AddContextMember(userID, contextID, role string)
- RemoveContextMember(userID, contextID string)
- ListContextMembers(contextID string)

- [ ] **Step 2: Update context_handler.go**

In internal/adapter/api/handler/context_handler.go:
- Add POST /contexts/:id/members endpoint
- Add DELETE /contexts/:id/members/:userId endpoint
- Add GET /contexts/:id/members endpoint

- [ ] **Step 3: Commit**

git add internal/adapter/api/handler/context_handler.go internal/usecase/context/
git commit -m "adapter: add context membership management endpoints"

### Task 2.7: Update Tests for Phase 2

**Files to modify:**
- cicd/support/fixtures.go
- cicd/context_http_test.go
- cicd/handler_test.go
- internal/infrastructure/context/repo/postgres_test.go

**Steps:**

- [ ] **Step 1: Update fixtures for platform users**

In cicd/support/fixtures.go:
- Add CreateUser fixture
- Add CreateMembership fixture
- Update NewTestContext to also create membership

- [ ] **Step 2: Update auth tests**

In cicd/context_http_test.go:
- Update Login test: no tenant field in request
- Add test for multi-context login response
- Add test for BootstrapAdmin

- [ ] **Step 3: Update membership tests**

In cicd/context_http_test.go:
- Add tests for POST /contexts/:id/members
- Add tests for DELETE /contexts/:id/members/:userId
- Add tests for GET /contexts/:id/members

- [ ] **Step 4: Run tests**

Run: go test ./cicd/... -v

Expected: All tests pass.

- [ ] **Step 5: Commit**

git add cicd/ internal/infrastructure/context/repo/postgres_test.go
git commit -m "tests: update for platform users and context memberships"

---

## Phase 3: Admin Bootstrap

**Files to create/modify:**
- internal/infrastructure/auth/service.go (add BootstrapAdmin)
- internal/adapter/api/handler/auth_handler.go (add bootstrap endpoint)
- internal/adapter/api/server.go (add bootstrap route)
- cmd/automata/main.go (call BootstrapAdmin on startup)

**Steps:**

- [ ] **Step 1: Implement BootstrapAdmin in auth service**

In internal/infrastructure/auth/service.go:
- Check if any users exist in users table
- If not, generate random email and password
- Create admin user with is_admin = true
- Create marker file .admin_initialized
- Return credentials

- [ ] **Step 2: Add bootstrap endpoint**

In internal/adapter/api/handler/auth_handler.go:
- Add POST /auth/bootstrap endpoint (unauthenticated, one-time)
- Returns generated credentials

- [ ] **Step 3: Add bootstrap route**

In internal/adapter/api/server.go:
- Add api.POST("/auth/bootstrap", authHandler.Bootstrap)

- [ ] **Step 4: Call BootstrapAdmin on startup**

In cmd/automata/main.go:
- After initializing auth service, call BootstrapAdmin if no admin exists
- Log credentials to stdout

- [ ] **Step 5: Update config.example.yaml**

Add:
- auth.admin_email: admin@automata.local (suggested default)
- auth.admin_password: (leave blank for auto-generation)

- [ ] **Step 6: Run tests**

Run: go test ./... -v

Expected: All tests pass.

- [ ] **Step 7: Commit**

git add internal/infrastructure/auth/service.go internal/adapter/api/handler/auth_handler.go internal/adapter/api/server.go cmd/automata/main.go config.example.yaml
git commit -m "auth: add admin bootstrap on first startup"

---

## Phase 4: Integration and Final Verification

**Steps:**

- [ ] **Step 1: Full build and test**

Run: go build ./... && go test ./... -v

Expected: All builds and tests pass.

- [ ] **Step 2: Docker Compose integration test**

Run: docker compose down && docker compose up -d && sleep 10 && curl http://localhost:9080/api/health

Expected: Health check returns ok.

- [ ] **Step 3: End-to-end login test**

Test the full login flow:
1. Bootstrap admin (if needed)
2. Login with email+password
3. Verify JWT contains context_slug
4. Make API request with X-Context-ID header

- [ ] **Step 4: Final commit**

git add -A
git commit -m "refactor: complete context rename and platform user migration"

---

## Rollback Plan

If any phase fails:
1. Run: git revert HEAD~N..HEAD (where N is number of phases)
2. Run migration rollback: DROP TABLE IF EXISTS users, user_contexts; ALTER TABLE contexts RENAME TO tenants; etc.
3. Restart services

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Migration breaks existing data | Run migrations on staging first, backup before production |
| Breaking API changes | Update API docs, communicate to frontend team |
| Test failures | Run tests after each phase, not just at the end |
| JWT compatibility | Keep old JWT claim name as alias during transition |
| Frontend incompatibility | Update frontend in parallel or provide backward-compatible header |
