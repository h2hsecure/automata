# Execute-all actions (2026-10-26)

Baseline at HEAD `3afd7e0`:
- CICD (`make test`): 456 specs — 455 pass, 1 known pre-existing failure (`POST /track/batch` 200 vs 204, tracking_server_test.go:138, skipped in cicd-tests-ci)
- Unit: 15 packages ok; known pre-existing: TestContextMiddleware (api, 17/18), proxy test build failure (mock missing GetDefaultContext)

## Actions
- [x] 0. Baseline test run (455/456)
- [x] 1. `specs/roadmap.md`: status date 2026-10-26, real counts (456/455/1), drop cookie_secret known issue, add 6 October features, MCP full server (49 tools) out of "foundation", profile/Security frontend open item
- [x] 2. SUPERSEDED banners: 5× plans/2025-09-10-*.md, 5× prds/2025-09-10-*-prd.md, 2026-09-22-context-rename spec+plan, brief banners on webui-implementation + phase1/phase2 sets
- [x] 3a. Fix 4 false footers: contact-activity L347, tracking-visitors L663, webhook L495, settings-profile L538
- [x] 3b. cicd test: session revocation — RED (token survived revocation: login access token had no iat) → fix iat in service.go Login → GREEN
- [x] 4a. Subagent A: contact import/export wiring + contact tag CRUD — DONE (build+vet clean, focused specs green; csv_exporter flush bug fixed)
- [ ] 4b. Subagent B: tracking.track MCP tool (spec 266) + webhook event wiring + 404 fix + TestDeliver fix — RUNNING
- [x] 4c. Subagent C: jti claim in JWT (spec §5.3) — DONE (service.go + auth_handler.go, 3 new unit tests green)
- [x] 5. BFF: docker-compose `bff` service (9082:8082, AUTOMATA_BFF_API_BACKEND_URL, WEB_DIR) + bff keys in config.example.yaml + docs/architecture.md section — compose config validates
- [x] 6. Integrate: resolve server.go overlap, build + full test + grouped commits + push (git@github.com:hekemen/automata.git main) — done 2026-10-26, commits pushed through f4024d2 (incl. pageview unload-flush fix), deploy-prod.md bumped to f4024d2 / 2026.10.26-f4024d2

## Subagent rules
- TDD, no git commits, return file:line summaries
- A touches: usecase/contact, adapter/api/handler/contact_handler.go, server.go (contact routes), adapter/mcp/contacts tools
- B touches: adapter/mcp/tracking_tools.go, usecase/form, infrastructure/queue webhook service, adapter/api/handler/webhook_handler.go, server.go (webhook routes if needed)
- C touches: infrastructure/auth/service.go, adapter/api middleware comment only
- A & B both edit server.go — minimal additive route changes only