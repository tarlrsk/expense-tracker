# Expense Tracker — instructions for Claude Code

A private, invite-only expense tracker where each user's data is fully separate: React PWA, Go API that owns
login and all data access, plain Postgres with RLS, Claude Haiku 4.5 for parsing and scanning.
The app runs locally for now (ADR-0020); no Supabase (ADR-0019). The development database is on Neon;
tests use Postgres in Docker and never connect to Neon (ADR-0026).

## The spec is the source of truth
The spec repo is at `../expense-tracker-spec` (sibling folder).
- Start at `INDEX.md` (ADR-0022): find the task's topic row and read only the ADRs, doc sections,
  plan and questions file it points to. Skip superseded ADRs and old journal entries unless asked about history.
- Work is organised as **plans** in `plans/`. Only execute plans with status Approved or In progress.
  Continuing a plan uses the `plan-run` skill; new work uses the `plan-new` skill.
- If the code must diverge from the spec, **stop and ask**, then record the change as a new ADR.
- Never rewrite an Accepted ADR — supersede it.

## Ask, don't guess (ADR-0021)
When anything is unclear while planning or implementing — a requirement, behaviour, library,
schema, API shape, security point or UX flow — stop and ask, with options and a recommendation.
Only trivial, easy-to-undo choices (local names, file layout inside a package, message wording)
may be made without asking; list them as `pending review` at wrap-up.
Record every question and answer in `../expense-tracker-spec/questions/` (`NNNN-<plan-slug>.md`
for a plan, `general.md` otherwise; format in `questions/README.md`). Simple go-aheads are not recorded.

## Recording decisions (required)
Any choice about a library, schema, API shape, infrastructure, security, or UX flow that is not
already in an ADR must be recorded with the `decision` skill:
- Decided by the user → `Decided by: user`
- Proposed by you and approved → `Decided by: claude (approved by user)`
- Trivial choices made without asking (see above) → `Decided by: claude (pending review)`, and mention them in the wrap-up.

Finish every session or plan task with the `wrap-up` skill.
At session start a hook runs the spec check (ADR-0023). If it reports problems, fix them first
(e.g. a catch-up journal entry), or ask the user, before starting new work.

## Which model writes code (ADR-0044)
If this session runs on Fable (or whichever model is the most expensive available), do not write
application code yourself: plan the task, ask the questions, then hand a written brief to the
`implementer` subagent (`.claude/agents/implementer.md`, runs on Opus). Afterwards review its diff,
run the tests and linters yourself, commit, and update the spec. Small edits (a few lines, a doc
fix, a review correction) you may make directly. On Opus or a cheaper model, implement directly.

## Working from plain chat
The user talks normally; you pick the right skill. Slash commands (`/decision`, `/plan-new`,
`/plan-run`, `/wrap-up`, `/spec`) are only a manual fallback.
- The user answers a question → record it in `questions/`; if it is a choice, also `decision`.
- The user makes or approves a choice → `decision`, then continue the task.
- The user describes new work → `plan-new` (check existing plans first).
- "continue", "next task", "let's work on …" → `plan-run`.
- The user asks why/what/when about the project → `spec`.
- A task is finished, or the user is stopping → `wrap-up`.
When unsure which applies, ask one short question rather than guessing.

## Repo layout
```
api/                  Go API (cmd/api; internal/handler; internal/registry/<module>/<use case>.go; internal/<module>/{orchestrator,processor,port}/<use case>; internal/<module>/domain; internal/external; internal/db; ADR-0032); no OpenAPI file (ADR-0031)
web/                  React + Vite + TS PWA (TanStack Router, Tailwind + shadcn/ui)
db/migrations/        SQL migrations (tables, RLS, roles, triggers, seeds); no GORM AutoMigrate (ADR-0024); applied with goose (ADR-0027)
docker-compose.yml    Postgres for tests and Mailpit (ADR-0026)
```

## Non-negotiable rules
- **Privacy (ADR-0014):** every user-data row has `owner_id`; users only ever see their own data.
  Every data query runs inside `WithUserTx` so RLS applies (ADR-0019). Only the `account`
  module may use `WithAuthTx` (role `app_auth`, account tables only, ADR-0034). Every endpoint needs a
  cross-user test. There are no groups or shared views — do not add any without a new ADR, but
  follow the "Later: groups" guardrails in `docs/05-roadmap.md` so they can be added later.
- **Access (ADR-0019, ADR-0025):** signup is invite-only. `/api/admin/*` is operator-only, manages accounts,
  and never returns other users' financial data. Users can never change `is_operator`.
- **The browser never queries tables.** Everything, including login, goes through `/api`.
- **Auth:** owned by the Go API, invite-only for now, email + password, opaque bearer tokens only, no cookies (ADR-0016, ADR-0019, ADR-0025). API responses send `Cache-Control: private, no-store`.
- **Secrets:** never read, print or commit `.env` files or keys. Use `.env.example` for shape.
  Nothing secret in `VITE_*` vars.
- **Time limit:** every API request must finish within a 60 s budget, so a later deploy needs no redesign (ADR-0016, ADR-0020).
- **AI:** only called from the API, via `ANTHROPIC_MODEL` env var; respect daily per-user limits.

## Conventions
- Go: gin, GORM, slog (ADR-0024); only `internal/db` holds the root `*gorm.DB`; database adaptors take their connection from the context inside `WithUserTx` / `WithAuthTx` and fail without one; processors and orchestrators import `internal/tx`, never `internal/db`; no outside call (email, AI) inside a transaction; code is grouped by module; inside a module the layers are orchestrator (optional) → processor → port, one folder per use case, each with an interface file and an implementation file (`interface.go` + `processor.go`, `port.go` + `adaptor_pg.go`); a processor never calls another processor (an orchestrator combines them); modules depend only on earlier modules; only adaptor files import GORM; every method takes `context.Context` (ADR-0032); wrap errors with context; table-driven tests; `go test ./...` and
  `golangci-lint run` must pass.
- Web: TypeScript strict, TanStack Query for server state, TanStack Router (ADR-0030), Tailwind + shadcn/ui (ADR-0029) on Base UI (ADR-0054); responsive and phone-first — style the small screen by default, widen with breakpoints, check every screen at phone width (ADR-0050); hand-written typed API client kept in one folder, in step with `docs/04-api.md` (ADR-0031); npm, ESLint + Prettier, Vitest, file-based routes in `web/src/routes/` (ADR-0051); the dev server binds `127.0.0.1` only (ADR-0052); `npm run typecheck`, `lint`, `format:check` and `test` must pass.
- Commits: conventional commits (`feat(api): …`, `fix(web): …`), small and focused.
- One plan task per session: plan the task, implement, test, commit, tick it, wrap up.

## Done means
Tests and linters pass, the change is committed, the plan task is ticked, affected spec docs are
updated, decisions are recorded, and the journal entry is written.
