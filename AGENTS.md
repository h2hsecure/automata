Automata is a marketing automation tool.

## Tech stack
- Go backend, PostgreSQL database
- Hexagonal architecture
- zerolog for logging
- Multi-tenant, reverse proxy, AI MCP interface

## Key files
- `docs/architecture.md` — tech stack details
- `specs/roadmap.md` — project roadmap

## When code appears
- Read `go.mod`, `Makefile`, and root config before guessing commands
- Hexagonal architecture: domain logic in `internal/domain`, adapters in `internal/adapter`, infrastructure in `internal/infrastructure` (verify actual layout once code exists)
- Prefer executable sources (scripts, config) over prose when they conflict
