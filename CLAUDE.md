# Expense Tracker — instructions for Claude Code

A private, invite-only expense tracker where each user's data is fully separate: React PWA, Go API that owns
login and all data access, plain Postgres with RLS, Claude Haiku 4.5 for parsing and scanning.
Everything runs locally for now (ADR-0020); no Supabase (ADR-0019).

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
api/                  Go API (cmd/api, internal/...), openapi.yaml, sqlc queries
web/                  React + Vite + TS PWA
db/migrations/        SQL migrations (tables, RLS, seeds); tool chosen in PLAN-0001
docker-compose.yml    Postgres and Mailpit for local development
```

## Non-negotiable rules
- **Privacy (ADR-0014):** every user-data row has `owner_id`; users only ever see their own data.
  Every data query runs inside `WithUserTx` so RLS applies (ADR-0019). Every endpoint needs a
  cross-user test. There are no groups or shared views — do not add any without a new ADR, but
  follow the "Later: groups" guardrails in `docs/05-roadmap.md` so they can be added later.
- **Access (ADR-0015):** signup is invite-only. `/api/admin/*` is operator-only, manages accounts,
  and never returns other users' financial data. Users can never change `is_operator`.
- **The browser never queries tables.** Everything, including login, goes through `/api`.
- **Auth:** owned by the Go API, invite-only, bearer tokens only, no cookies (ADR-0016, ADR-0019). API responses send `Cache-Control: private, no-store`.
- **Secrets:** never read, print or commit `.env` files or keys. Use `.env.example` for shape.
  Nothing secret in `VITE_*` vars.
- **Time limit:** every API request must finish within a 60 s budget, so a later deploy needs no redesign (ADR-0016, ADR-0020).
- **AI:** only called from the API, via `ANTHROPIC_MODEL` env var; respect daily per-user limits.

## Conventions
- Go: chi, pgx, sqlc, slog; wrap errors with context; table-driven tests; `go test ./...` and
  `golangci-lint run` must pass.
- Web: TypeScript strict, TanStack Query for server state, API client generated from `openapi.yaml`.
- Commits: conventional commits (`feat(api): …`, `fix(web): …`), small and focused.
- One plan task per session: plan the task, implement, test, commit, tick it, wrap up.

## Done means
Tests and linters pass, the change is committed, the plan task is ticked, affected spec docs are
updated, decisions are recorded, and the journal entry is written.
