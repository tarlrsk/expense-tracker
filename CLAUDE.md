# Expense Tracker — instructions for Claude Code

A private, invite-only expense tracker where each user's data is fully separate: PWA frontend on Vercel, Go API on Cloud Run,
Supabase (Auth, Postgres with RLS, Storage), Claude Haiku 4.5 for parsing and scanning.

## The spec is the source of truth
The spec repo is at `../expense-tracker-spec` (sibling folder).
- Before starting any step, read `docs/` and the relevant `decisions/`.
- Work is organised as **plans** in `plans/`. Only execute plans with status Approved or In progress.
  Continuing a plan uses the `plan-run` skill; new work uses the `plan-new` skill.
- If the code must diverge from the spec, **stop and ask**, then record the change as a new ADR.
- Never rewrite an Accepted ADR — supersede it.

## Recording decisions (required)
Any choice about a library, schema, API shape, infrastructure, security, or UX flow that is not
already in an ADR must be recorded with the `decision` skill:
- Decided by the user → `Decided by: user`
- Proposed by you and approved → `Decided by: claude (approved by user)`
- Small choices you had to make without asking → `Decided by: claude (pending review)`, and mention them in the wrap-up.

Finish every session or plan task with the `wrap-up` skill.

## Working from plain chat
The user talks normally; you pick the right skill. Slash commands (`/decision`, `/plan-new`,
`/plan-run`, `/wrap-up`, `/spec`) are only a manual fallback.
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
supabase/migrations/  SQL migrations (tables, RLS, seeds)
.github/workflows/    api.yml, db.yml (web deploys via Vercel Git integration)
web/vercel.json       Vercel rewrites (/api → Cloud Run, SPA fallback) and headers
```

## Non-negotiable rules
- **Privacy (ADR-0014):** every user-data row has `owner_id`; users only ever see their own data.
  Every data query runs inside `WithUserTx` so RLS applies (ADR-0008). Every endpoint needs a
  cross-user test. There are no groups or shared views — do not add any without a new ADR, but
  follow the "Later: groups" guardrails in `docs/05-roadmap.md` so they can be added later.
- **Access (ADR-0015):** signup is invite-only. `/api/admin/*` is operator-only, manages accounts,
  and never returns other users' financial data. Users can never change `is_operator`.
- **The browser never queries tables.** Supabase JS is for auth only; data goes through `/api`.
- **Auth:** bearer tokens only, no cookies (ADR-0016). API responses send `Cache-Control: private, no-store`.
- **Secrets:** never read, print or commit `.env` files or keys. Use `.env.example` for shape.
  Production secrets live in Secret Manager. Nothing secret in `VITE_*` vars.
- **Time limit:** every API request must finish within a 60 s budget (Vercel allows 120 s to first byte; ADR-0016).
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
