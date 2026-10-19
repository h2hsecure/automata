# Ledger — Per-Website SMTP + Queue (2026-10-19)

Plan: `docs/superpowers/plans/2026-10-19-per-website-smtp-queue.md`
Base HEAD: `35a1ae3` (branch `phase2-auth-changes`)

| Task | Name | Implementer | Review | Commit |
|------|------|-------------|--------|--------|
| 1 | Context-scoped settings (backend) | done, 356/356 | PASS | `ecf0d05` |
| 2 | Context-aware SMTP | done, 356/356 | PASS | `19de0bf` |
| 3 | Queue history, isolation, management | done, 363/363 | PASS (2 minor) | `d7e5836` |
| 4 | Email queue API | done, 374/374 | PASS | `30f1e19` |
| 5 | Frontend WebsiteEmailPanel | done, build OK | PASS (minors) | `05513fe` + fix `d0a1d36` |
| 6 | Deploy + live verification | done, 18/18 API checks (re-verified after fixes) | n/a | image `60152f71` (`d28d39e`) |
| — | Final review (full branch) | SHIP-WITH-NOTES, 374/374 | done | review fixes `d28d39e` (sentinel 404/500, jobId validation, handler logging, per-loader gen counters) |