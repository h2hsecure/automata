# Contact Activity Timeline — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose a paginated, type-filtered activity timeline per contact with activity count on contact detail, context scoping, and activity creation plumbing from domain events.

**Architecture:** Extend the existing `contact_activities` table with new type constants, add `CreateActivity` and type-filtered `GetActivity` to the repository layer, enrich the usecase layer with title/description formatting and activity count, register the handler route under `/api/context/contacts/:id/activity`, and wire activity creation from contact lifecycle usecases.

**Tech Stack:** Go 1.26.2, Gin v1.12.0, PostgreSQL (pgx v5), zerolog

**Spec:** `docs/superpowers/specs/2026-09-25-contact-activity-design.md`

## Global Constraints

- Module path: `github.com/hekemen/automata`
- Go version: `1.26.2`
- All UUIDs must be valid UUID format strings (use `gen_random_uuid()`)
- JWT HS256 signing
- Config loading: `config.Get("key")` from `internal/infrastructure/config/config.go`
- Database migrations: use `CREATE TABLE IF NOT EXISTS` and `ON CONFLICT` for idempotency
- Hexagonal architecture: domain interfaces in `internal/domain/`, infrastructure in `internal/infrastructure/`, adapters in `internal/adapter/`
- All new files go in existing package directories

---

### Task 1: Extend ActivityType constants and Activity data schemas

**Files:**
- Modify: `internal/domain/contact/activity.go`

**Interfaces:**
- Consumes: `contact.Activity` from `domain/contact`
- Produces: Extended `ActivityType` constants

Steps:
1. **Step 1: Add new ActivityType constants**
   - Add `ActivityBannerImpression`, `ActivityTagAdded`, `ActivityTagRemoved` constants to `activity.go`
   - Update the const block to include all 9 types per spec §4.3
2. **Step 2: Add ActivityResult type**
   - Define `ActivityResult` struct with `Activities []Activity`, `Total int64`, `ActivityCount int64`
   - Define helper `ActivityDetail` struct with `Title`, `Description` fields alongside existing Activity fields

**Verification:**
- `go build ./...` compiles clean

---

### Task 2: Update UserRepository interface and User domain model

**Files:**
- Modify: `internal/domain/context/user.go`
- Modify: `internal/domain/context/user_repository.go`

**Interfaces:**
- Consumes: `context.User` from `domain/context`
- Produces: Extended `User` struct and `UserRepository` interface

Steps:
1. **Step 1: Add profile columns to User struct**
   - Add `DisplayName *string` — nil means unset
   - Add `AvatarURL *string` — nil means no avatar
   - Add `PasswordChangedAt time.Time` — for session revocation
   - Update `Validate()` to not require password if SSO provider is set

2. **Step 2: Add new methods to UserRepository interface**
   - `UpdateProfile(userID string, displayName, avatarURL *string) error`
   - `ChangePassword(userID string, passwordHash string) error`
   - `GetPasswordChangedAt(userID string) (time.Time, error)`
   - `UpdatePasswordChangedAt(userID string) error`

**Verification:**
- `go build ./...` compiles clean

---

### Task 3: Update User PostgreSQL repository

**Files:**
- Modify: `internal/infrastructure/context/repo/user_postgres.go`

**Interfaces:**
- Consumes: `context.UserRepository` interface
- Produces: Extended PostgreSQL implementation

Steps:
1. **Step 1: Update existing queries to include new columns**
   - `Create()`: Add `display_name`, `avatar_url`, `password_changed_at` to INSERT and RETURNING
   - `GetByID()`: Add `display_name`, `avatar_url`, `password_changed_at` to SELECT and Scan
   - `GetByEmail()`: Add new columns
   - `GetByUsername()`: Add new columns
   - `ListAll()`: Add new columns
   - `Update()`: Add `display_name`, `avatar_url` to SET clause

2. **Step 2: Implement new methods**
   - `UpdateProfile(userID string, displayName, avatarURL *string) error`:
     ```sql
     UPDATE users SET
       display_name = COALESCE($2, display_name),
       avatar_url = COALESCE($3, avatar_url),
       updated_at = NOW()
     WHERE id = $1
     ```
   - `ChangePassword(userID string, passwordHash string) error`:
     ```sql
     UPDATE users SET
       password_hash = $2,
       password_changed_at = NOW(),
       updated_at = NOW()
     WHERE id = $1
     ```
   - `GetPasswordChangedAt(userID string) (time.Time, error)`:
     ```sql
     SELECT password_changed_at FROM users WHERE id = $1
     ```
   - `UpdatePasswordChangedAt(userID string) error`:
     ```sql
     UPDATE users SET password_changed_at = NOW(), updated_at = NOW() WHERE id = $1
     ```

**Verification:**
- `go build ./...` compiles clean

---

### Task 4: Contact repository — CreateActivity, type-filtered GetActivity

**Files:**
- Modify: `internal/domain/contact/repository.go`
- Modify: `internal/infrastructure/contact/repo/postgres.go`
- Modify: `internal/usecase/contact/tags.go` (Repository interface)

**Interfaces:**
- Consumes: `contact.Repository` interface
- Produces: Extended `CreateActivity`, type-filtered `GetActivity(contactID, contextID, offset, limit, activityType) ([]Activity, int64, error)`

Steps:
1. **Step 1: Update Repository interface**
   - Add `CreateActivity(a *Activity) error`
   - Update `GetActivity(contactID string, offset, limit int) ([]Activity, error)` → `GetActivity(contactID, contextID string, offset, limit int, activityType *string) ([]Activity, int64, error)`

2. **Step 2: Update tags.go Repository interface** to match

3. **Step 3: Implement CreateActivity in postgres.go**
   ```sql
   INSERT INTO contact_activities (id, contact_id, context_id, type, data, source_id, created_at)
   VALUES ($1, $2, $3, $4, $5, $6, NOW())
   ```

4. **Step 4: Implement type-filtered GetActivity in postgres.go**
   - Build WHERE clause: `contact_id = $1 AND context_id = $2`
   - Add `type = $3` if activityType is non-nil
   - Execute COUNT query separately for total
   - Execute paginated SELECT for activities list
   - Return `(activities, total, error)`

**Verification:**
- `go build ./...` compiles clean

---

### Task 5: Contact migration — new types, cookie mappings, GIN index

**Files:**
- Modify: `internal/infrastructure/contact/migration.sql`

**Interfaces:**
- Consumes: existing migration
- Produces: Extended migration with new tables and indexes

Steps:
1. **Step 1: Add GIN index on data column**
   ```sql
   CREATE INDEX IF NOT EXISTS idx_activities_data ON contact_activities USING GIN (data);
   ```

2. **Step 2: Add contact_cookie_mappings table**
   ```sql
   CREATE TABLE IF NOT EXISTS contact_cookie_mappings (
       contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
       context_id  UUID NOT NULL,
       cookie      TEXT NOT NULL,
       source      VARCHAR(32) NOT NULL,
       created_at  TIMESTAMPTZ DEFAULT NOW(),
       PRIMARY KEY (context_id, cookie)
   );
   CREATE INDEX IF NOT EXISTS idx_cookie_contact ON contact_cookie_mappings(contact_id);
   ```

**Verification:**
- SQL is syntactically valid (tested with `psql --no-password -f migration.sql`)

---

### Task 6: Contact usecase layer — GetActivity with filtering, CreateActivity plumbing, activity count

**Files:**
- Modify: `internal/usecase/contact/get_activity.go`
- Modify: `internal/usecase/contact/create_contact.go`
- Modify: `internal/usecase/contact/update_contact.go`
- Modify: `internal/usecase/contact/tags.go`
- Modify: `internal/usecase/contact/merge_contacts.go`

**Interfaces:**
- Consumes: `contact.Repository`
- Produces: Enriched activity responses, activity creation calls

Steps:
1. **Step 1: Update GetActivity usecase**
   - Accept `activityType *string` parameter
   - Call `repo.GetActivity(contactID, contextID, offset, limit, activityType)`
   - Return `(result []contact.ActivityDetail, total int64, err error)`
   - Add `formatActivityTitle(a Activity) string` helper: type-switch producing human-readable titles
   - Add `formatActivityDescription(a Activity) string` helper

2. **Step 2: Add CreateActivity call to CreateContact**
   - After `repo.Create(contact)`, call `repo.CreateActivity()` with:
     ```go
     activity := &contact.Activity{
         ID:      uuid.New().String(),
         ContactID: id,
         ContextID: contextID,
         Type:    contact.ActivityContactCreated,
         Data: map[string]interface{}{
             "source":    input.Source,
             "created_at": time.Now().Format(time.RFC3339),
         },
     }
     repo.CreateActivity(activity)
     ```

3. **Step 3: Add CreateActivity call to UpdateContact**
   - After `repo.Update(existing)`, compute `changed_fields` by comparing old vs new values
   - Create `contact_updated` activity with changed fields

4. **Step 4: Add CreateActivity call to ApplyTags (tags.go)**
   - Before: get old tag names, after: get new tag names
   - Create `tag_added` activity for each new tag
   - Create `tag_removed` activity for each removed tag

5. **Step 5: Add CreateActivity call to MergeContacts**
   - After `repo.Merge(keepID, mergeIntoID)`, create `contact_merged` activity

**Verification:**
- `go build ./...` compiles clean

---

### Task 7: Contact handler — context scoping, type filter, activity count on Get

**Files:**
- Modify: `internal/adapter/api/handler/contact_handler.go`

**Interfaces:**
- Consumes: `contact.Repository`, `auth.AuthService` (for is_admin)
- Produces: Extended JSON responses with activity_count, type-filtered activity timelines

Steps:
1. **Step 1: Extend Get handler with activity_count**
   - After fetching contact, add subquery for activity count:
     ```sql
     SELECT (SELECT COUNT(*) FROM contact_activities ca WHERE ca.contact_id = c.id) AS activity_count
     ```
   - Return `gin.H{"contact": contact, "activity_count": count}`

2. **Step 2: Update GetActivity handler**
   - Add `is_admin` check from context (set by AuthMiddleware): if `is_admin == true`, skip context check
   - Parse `type` query parameter as `activityType *string`
   - Parse `limit` (default 50, max 200), `offset` (default 0)
   - Verify contact context matches context_id unless admin
   - Call usecase with type filter
   - Return `{"activities": activities, "total": total}`

3. **Step 3: Add context verification**
   - In `GetActivity`, fetch the contact first to verify context_id matches (unless admin)
   - Return 403 if non-admin user accesses a contact from a different context

**Verification:**
- `go build ./...` compiles clean

---

### Task 8: Register activity routes in server.go

**Files:**
- Modify: `internal/adapter/api/server.go`

**Interfaces:**
- Consumes: `contact.Repository`, `auth.AuthService`
- Produces: Registered route under `/api/context/contacts/:id/activity`

Steps:
1. **Step 1: Register the activity timeline endpoint**
   - Add after existing context routes:
     ```go
     contactsGroup := api.Group("/context/contacts")
     {
         contactHandler := handler.NewContactHandler(h.contactRepo)
         contactsGroup.GET("/:id/activity", middleware.AuthMiddleware(authService), contactHandler.GetActivity)
     }
     ```
   - Note: The existing `/api/context/...` routes already exist under ContextResolver middleware; add the activity route to the appropriate group

**Verification:**
- `go build ./...` compiles clean

---

### Task 9: Testing — unit tests for activity, integration tests for full flow

**Files:**
- Create: `cicd/activity_repo_test.go`
- Create: `cicd/activity_http_test.go`
- Create: `cicd/activity_handler_test.go`

**Interfaces:**
- Consumes: `support.TestDB`, `support` fixtures
- Produces: Test coverage for activity CRUD, type filtering, context scoping

Steps:
1. **Step 1: Unit tests for CreateActivity and GetActivity**
   - Test `CreateActivity` inserts a record and verifies data column
   - Test `GetActivity` returns activities ordered by `created_at DESC`
   - Test `GetActivity` with type filter returns only matching activities
   - Test `GetActivity` total count matches filtered results

2. **Step 2: Integration test — full flow**
   - Create contact → create activities of different types → retrieve timeline → verify pagination and filtering

3. **Step 3: HTTP integration test**
   - Register the route in a test server, send requests with and without auth, verify 200/403/404 responses
   - Test type filter query parameter parsing

**Verification:**
- `go build ./...` compiles clean
- Integration tests pass with `go test ./cicd/...`

---

## Phase 5: Frontend Integration (out of scope for this spec)

1. BFF/Vue UI: Fetch activities via `GET /api/context/contacts/:id/activity?limit=50&type=form_submission`
2. Display: Timeline component with type-based filtering tabs
3. Contact detail page: Show `activity_count` badge
