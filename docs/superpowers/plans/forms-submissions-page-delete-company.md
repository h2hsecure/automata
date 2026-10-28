# Plan: Forms submissions page, deletion, company default field

## Context

Approved in-chat design (bounded brainstorm, no spec file — this plan IS the spec):
1. Move form submissions from the modal in `FormsView.vue` to a dedicated page (route `forms/:id/submissions`, following the `contacts/:id` detail-page pattern).
2. Allow deleting a submission: full backend addition (interface → postgres → usecase → handler → route) + UI delete with confirm dialog.
3. Add a `company` default field when creating a form (backend already maps `data["company"]` → contact `Company`).

## Global Constraints

- Branch: `feat/forms-submissions-page` (worktree at `/home/hekemen/.local/share/opencode/worktree/d75391/forms-submissions-page`).
- Pre-existing baseline failures (NOT caused by this plan; do not fix): `internal/adapter/api` TestContextMiddleware (expects "context not found", middleware says "no context available") and `internal/adapter/proxy` build failure (mockContextRepo missing `GetDefaultContext`).
- Go responses use PascalCase (no json tags); route context resolved via `c.GetString("context_id")`; 401 when empty.
- DELETE submission route: `api.DELETE("/context/forms/:id/submissions/:sid", ...)` in `internal/adapter/api/server.go` admin block (line ~231), NOT the tracking adapter.
- Submissions are context-scoped by fetching then comparing `s.ContextID` (mirror `GetSubmission` usecase); foreign/missing → "submission not found" (404), not 403.
- Frontend: Vue 3 + TS, vitest; reuse `useConfirm`/`ConfirmDialog`, `usePagination`, `DataTable`.
- MCP delete tool: NOT included (not requested; ruling recorded in ledger).

## Task 1: Backend DELETE submission

Files:
- `internal/domain/form/repository.go` — add `DeleteSubmission(id string) error` to `Repository`.
- `internal/usecase/form/delete_submission.go` (new) + `delete_submission_test.go` (new).
- `internal/infrastructure/form/repo/postgres.go` — postgres `DeleteSubmission` (mirror `Delete`: `DELETE FROM form_submissions WHERE id = $1`, `RowsAffected()==0` → `sql.ErrNoRows` wrap).
- `internal/adapter/api/handler/form_handler.go` — `DeleteSubmission(c)` (mirror `Delete` at line 190: 401 empty context, `uc.DeleteSubmission` error → 404 "submission not found", success → 204).
- `internal/adapter/api/handler/form_handler_test.go` — extend `fakeFormRepo` (add `DeleteSubmission` tracking method; make `GetSubmission` return a configurable submission), add `TestDeleteSubmission_Handler` (401 / 404 / 204).
- `internal/adapter/api/server.go` — register route after line 231.

Steps:
1. RED: write `delete_submission_test.go` (usecase tests: empty id → error; get-submission error → "submission not found"; context mismatch → "submission not found"; success calls repo delete; delete error wrapped). Needs fake repo with `DeleteSubmission` — compile failure is the RED.
2. Run `go test ./internal/usecase/form/...` → expect build failure (undefined `DeleteSubmission`).
3. Implement: interface method, postgres impl, usecase `DeleteSubmission(repo form.Repository, contextID, id string) error`, handler method, route, fake repo method.
4. GREEN: `go test ./internal/usecase/form/... ./internal/adapter/api/handler/...` → ok.
5. Commit: `feat(forms): add DELETE submission endpoint (repo, usecase, handler, route)`.

## Task 2: Frontend submissions page

Files:
- `webui/src/views/FormSubmissionsView.vue` (new) — page version of the modal: route param `id`, load form via `getForm(id)` (check client.ts exports `getForm` — verify first), DataTable with same flattened rows + files + created columns, per-row Delete button (`useConfirm` destructive), pagination footer, Back button (`router.push('/forms')`), `ConfirmDialog` mounted.
- `webui/src/api/client.ts` — `deleteSubmission(formId, submissionId): Promise<void>` → `client.delete(...)`.
- `webui/src/router/index.ts` — route `forms/:id/submissions`, name `FormSubmissions`.
- `webui/src/views/FormsView.vue` — remove submissions modal + its state (`subModalOpen`, `subForm`, `subRows`, `subsLoading`, pagination, `subColumns`, `flattenSubmission`, `openSubmissions`, `loadSubmissions`, `downloadFile`, watch); Inbox button → `router.push({ name: 'FormSubmissions', params: { id: row.ID } })`; drop now-unused imports.
- `webui/src/views/FormsView.spec.ts` — delete the `FormsView submissions modal` describe block (10 tests total; 5 move to the new spec).
- `webui/src/views/FormSubmissionsView.spec.ts` (new) — tests: renders rows with field columns, pagination, empty state, delete flow (confirm dialog → `deleteSubmission` called → list reloaded), back button navigates.

Steps:
1. RED: write `FormSubmissionsView.spec.ts` → fails (component missing).
2. Run `npm test` in webui → expect new spec failure only.
3. Implement view, router entry, client fn, FormsView rewiring, spec cleanup.
4. GREEN: `npm test` in webui → all pass (was 39; expect ~44).
5. Commit: `feat(webui): submissions page with delete; remove submissions modal`.

## Task 3: Default company field on form create

Files:
- `webui/src/views/FormsView.vue` — `openCreate()` defaults gain `{ name: 'company', type: 'text', required: false, label: 'Company', accept: '' }`.
- `webui/src/views/FormsView.spec.ts` — test: opening create modal shows email, name, company fields.

Steps:
1. RED: add the test → fails (only 2 default fields).
2. Implement the default.
3. GREEN: `npm test` in webui → all pass.
4. Commit: `feat(webui): add company field to default form fields`.

## Review Focus

- Context scoping of delete (foreign-context submission must be 404, never deleted).
- Frontend state hygiene after modal removal (no dead imports/refs, no broken spec coverage).
- Route param vs `authStore.currentContextId` consistency in the new page.