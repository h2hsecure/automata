# Context Rename + Login Simplification (Phase 2) Implementation Plan

> **Superseded:** This plan was executed and completed on 2026-10-13 (see Execution Status below). The canonical post-hoc record for this work is `2026-09-24-context-rename-and-login-simplification.md`; `2026-09-24-context-rename-login-phase2.md` is another duplicate of the same scope. Kept for history.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the remaining pieces of the context-rename / login-simplification design to align the running application with the new schema and authentication model.

**Architecture:** Transition from context-scoped users (`context_users`) to platform-level users (`users`) with role-based memberships (`user_contexts`). This involves database migrations, domain model updates, infrastructure implementation, and API layer adjustments.

**Tech Stack:** Go, PostgreSQL (pgx), Gin, JWT, Bcrypt.

**Spec:** docs/superpowers/specs/2026-09-24-implementation-spec.md

## Execution Status

**Status: Completed** — executed 2026-10-13 on branch `phase2-auth-changes`. The plan was written post-hoc to document work that had already been completed; execution consisted of verifying every step against the codebase, closing the one real gap, and checking off the steps.

- **Code gap closed:** `cicd/global_login_test.go` still seeded `context_users`; it now seeds `users` + `user_contexts` (5 INSERTs + cleanup updated).
- **Verification:** `go build ./...`, `go vet ./cicd/...`, and `make build` all pass. Focused run: all 6 "Global Login" specs pass against a real Postgres testcontainer. Full cicd suite: 279 passed / 23 failed — the 23 failures are pre-existing and unrelated to Phase 2 (tracking/form/banner/placement/campaign specs); the identical failure set was confirmed by re-running the suite with the change stashed.
- **Task 6 Step 3 performed:** `tenantID` → `contextID` rename applied to the 6 cicd test files containing it (169 occurrences); suite results unchanged.

### Deviations from the plan

| Plan step | Actual state |
|---|---|
| Task 1: test at `tests/infrastructure/database/migration_test.go` | Test lives at `internal/infrastructure/database/migration_test.go` (no `tests/` directory in this repo). |
| Task 1: migration drops `idx_context_users_context_email` | `context_users` and its index are kept for backward compatibility — `Login` falls back to `context_users` via `GetByEmailGlobal` when the user is not found in `users`. |
| Task 1: canonical migration file | `cicd/support/testdb.go` runs `migration_users.sql` explicitly (complete, idempotent version); `migration_user_contexts.sql` also exists and is applied via the directory scan (both use `IF NOT EXISTS`, so they are safe to run together). |
| Task 3: create `user_context_postgres.go` | Membership methods (`CreateMembership`, `ListMembers`, etc.) are implemented in `user_postgres.go` instead — single repo file. |
| Task 3: `bootstrap.Bootstrap(pool)` | Actual function is `bootstrap.Run(pool, userRepo, contextRepo)` in `internal/infrastructure/bootstrap/admin.go`, wired into `cmd/automata/main.go`. |
| Task 3: bootstrap test | Covered by `cicd/bootstrap_test.go`. |
| Task 6: `go test ./...` fully green | Full cicd suite has 23 pre-existing failures unrelated to Phase 2 (see above); no regressions from this plan's changes. |
| Commit steps | The work is in branch history (e.g. `22fe826 feat: add users and user_contexts tables migration`); per-task commits were not replayed separately. The remaining delta (test updates, rename) is left uncommitted pending review. |

## Global Constraints

- Use `contextID` instead of `tenantID` (verbatim from spec).
- Password hashing must use bcrypt with cost 10.
- Admin email defaults to `admin@automata.local` if not configured.

## Review Focus

- **Migration Integrity:** Ensure `context_users` data is correctly migrated to `users` and `user_contexts` without loss of ownership or membership.
- **Auth Middleware:** Verify that `user_id`, `user_email`, and `is_admin` are correctly extracted from JWT and set in the Gin context.
- **Admin Bootstrap:** Confirm that the admin bootstrap only runs once and correctly handles the case where a marker file exists but no admin is in the DB.
- **Role Enforcement:** Ensure `owner`, `admin`, and `member` roles are correctly respected in the `user_contexts` table.
- **API Security:** Verify that `/admin` routes are strictly protected by the updated auth middleware.

---

### Task 1: Database Migration

**Files:**
- Create: `internal/infrastructure/database/migration_user_contexts.sql`
- Test: `tests/infrastructure/database/migration_test.go` (to be created)

**Interfaces:**
- Consumes: Existing PostgreSQL connection pool.
- Produces: `users` and `user_contexts` tables populated with migrated data.

- [x] **Step 1: Write the migration SQL file**
  Implement the SQL script following the spec: create tables, migrate data from `context_users`, drop old index, and add new indices.

- [x] **Step 2: Implement migration test**
  Write a test that runs the migration against a test database and verifies:
  1. `users` table has correct counts.
  2. `user_contexts` has correct roles (`owner` for old owners, `admin` otherwise).
  3. Old `idx_context_users_context_email` is gone.

- [x] **Step 3: Run migration test to verify it passes**
  Run: `go test ./tests/infrastructure/database/...`
  Expected: PASS

- [x] **Step 4: Commit**
  ```bash
  git add internal/infrastructure/database/migration_user_contexts.sql tests/infrastructure/database/migration_test.go
  git commit -m "feat: add user and user_contexts migration"
  ```

### Task 2: Domain Layer Updates

**Files:**
- Modify: `internal/domain/context/user.go`
- Modify: `internal/domain/context/user_repository.go`
- Modify: `internal/domain/auth/auth.go`

**Interfaces:**
- Consumes: Existing domain types.
- Produces: Updated `User`, `UserContext`, `UserRepository` interface, and `AuthService` interface.

- [x] **Step 1: Update `User` and `UserContext` structs in `internal/domain/context/user.go`**
  Add `PasswordHash`, `SSOProvider`, `SSOID`, `IsAdmin`, etc., to `User`. Add `UserContext` struct.

- [x] **Step 2: Update `UserRepository` interface in `internal/domain/context/user_repository.go`**
  Add new methods: `GetByUsername`, `GetByUserIDContext`, `ListByUser`, `CreateMembership`, `UpdateMembership`, `DeleteMembership`, `ListMembers`, `ExistsAnyUser`.

- [x] **Step 3: Update `AuthService` interface in `internal/domain/auth/auth.go`**
  Update `Login` to return `[]ContextInfo` and add `GetCurrentUser`. Update `ContextInfo` to include `Role`.

- [x] **Step 4: Run existing domain tests to ensure no breakage**
  Run: `go test ./internal/domain/...`
  Expected: PASS (Note: some tests might fail if they use the old interface; these will be fixed in later tasks).

- [x] **Step 5: Commit**
  ```bash
  git add internal/domain/context/user.go internal/domain/context/user_repository.go internal/domain/auth/auth.go
  git commit -m "refactor: update domain models and interfaces for new auth schema"
  ```

### Task 3: Infrastructure Implementation (Repository & Bootstrap)

**Files:**
- Modify: `internal/infrastructure/context/repo/user_postgres.go`
- Create: `internal/infrastructure/context/repo/user_context_postgres.go`
- Create: `internal/infrastructure/bootstrap/admin.go`
- Modify: `cmd/automata/main.go`

**Interfaces:**
- Consumes: `UserRepository` and `UserContext` interfaces.
- Produces: Concrete implementations of the new repository methods and the `Bootstrap` function.

- [x] **Step 1: Implement new `UserRepository` methods in `internal/infrastructure/context/repo/user_postgres.go`**
  Implement `GetByUsername`, `ExistsAnyUser`, etc.

- [x] **Step 2: Create `internal/infrastructure/context/repo/user_context_postgres.go`**
  Implement membership methods (`CreateMembership`, `ListMembers`, etc.).

- [x] **Step 3: Implement `Bootstrap` in `internal/infrastructure/bootstrap/admin.go`**
  Implement the logic to create the first admin user and log credentials to stdout.

- [x] **Step 4: Integrate `Bootstrap` in `cmd/automata/main.go`**
  Call `bootstrap.Bootstrap(pool)` after DB connection.

- [x] **Step 5: Run bootstrap test**
  Write and run a test for `Bootstrap` to ensure it creates the admin and the marker file.

- [x] **Step 6: Commit**
  ```bash
  git add internal/infrastructure/context/repo/ user_postgres.go user_context_postgres.go internal/infrastructure/bootstrap/admin.go cmd/automata/main.go
  git commit -m "feat: implement user repository and admin bootstrap"
  ```

### Task 4: Infrastructure Implementation (Auth Service)

**Files:**
- Modify: `internal/infrastructure/auth/service.go`

**Interfaces:**
- Consumes: `UserRepository`, `UserContext` repository, and `AuthService` interface.
- Produces: Updated `Login` implementation.

- [x] **Step 1: Update `Login` in `internal/infrastructure/auth/service.go`**
  Change logic to query `users` table, verify bcrypt, and return memberships from `user_contexts`.

- [x] **Step 2: Run auth tests**
  Run: `go test ./internal/infrastructure/auth/...`
  Expected: PASS (after fixing dependencies).

- [x] **Step 3: Commit**
  ```bash
  git add internal/infrastructure/auth/service.go
  git commit -m "feat: update auth service for new user schema"
  ```

### Task 5: API Layer & Middleware

**Files:**
- Modify: `internal/adapter/api/server.go`
- Create: `internal/adapter/api/handler/user_handler.go`
- Create: `internal/adapter/api/handler/context_user_handler.go`
- Modify: `internal/adapter/api/handler/auth_handler.go`
- Modify: `internal/adapter/api/middleware/auth.go`

**Interfaces:**
- Consumes: `UserRepository`, `ContextRepository`, and `AuthService`.
- Produces: New API endpoints and updated middleware.

- [x] **Step 1: Implement `UserHandler` and `ContextUserHandler`**
  Create the new handlers for user and membership management.

- [x] **Step 2: Update `AuthMiddleware` in `internal/adapter/api/middleware/auth.go`**
  Update to extract `user_id`, `user_email`, and `is_admin` from JWT.

- [x] **Step 3: Register new routes in `internal/adapter/api/server.go`**
  Add `/admin/users`, `/admin/contexts/:id/users`, and `/auth/me`.

- [x] **Step 4: Update `AuthHandler` in `internal/adapter/api/handler/auth_handler.go`**
  Update `Login` response and implement `GetMe`.

- [x] **Step 5: Run API tests**
  Run: `go test ./internal/adapter/api/...`
  Expected: PASS.

- [x] **Step 6: Commit**
  ```bash
  git add internal/adapter/api/ handler/ server.go middleware/auth.go
  git commit -m "feat: implement user management APIs and update auth middleware"
  ```

### Task 6: Final Verification & Cleanup

**Files:**
- Modify: `config.example.yaml`
- Modify: `cicd/global_login_test.go`
- Modify: Various test files to fix `tenantID` -> `contextID`.

- [x] **Step 1: Update `config.example.yaml`**
  Add/verify `auth.admin_email`.

- [x] **Step 2: Update `cicd/global_login_test.go`**
  Update to use the new `users` table and login flow.

- [x] **Step 3: Global search and replace `tenantID` with `contextID` in tests**
  Ensure all tests are consistent with the new naming convention.

- [x] **Step 4: Run full test suite**
  Run: `go test ./...`
  Expected: PASS.

- [x] **Step 5: Commit**
  ```bash
  git add .
  git commit -m "chore: final cleanup and test updates"
  ```
