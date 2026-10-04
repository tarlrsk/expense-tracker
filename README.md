# Satang — expense tracker

A private, invite-only expense tracker: a React PWA, a Go API that owns login and all data
access, and Postgres with row-level security so each user only ever sees their own data.
It runs locally for now. The spec (decisions, plans, docs) lives in the sibling repo
`../expense-tracker-spec`; start at its `INDEX.md`.

## What you need
- Go (current stable), Node 24 (`web/.nvmrc`), Docker, `make`.
- A Neon project with Postgres 18 for the development database. Tests never use it: they run
  on Postgres in Docker.

## First-time setup
1. Install the tools and dependencies:
   ```sh
   make tools        # golangci-lint, pinned
   make web-install  # web dependencies
   ```
2. Copy `.env.example` to `.env`. Put the Neon **owner** connection string for the **direct**
   host (not the `-pooler` one) in `MIGRATION_DATABASE_URL`. `.env` is never committed.
3. Create the tables:
   ```sh
   make migrate
   ```
4. Give the API its own database login. This prints a `DATABASE_URL=…` line; paste it into
   `.env`. The line holds a password, so treat the output as secret.
   ```sh
   make db-login-password
   ```
5. Create the first operator (the account that can invite others). Mailpit must be running to
   catch the email:
   ```sh
   docker compose up -d mailpit
   make operator EMAIL=you@example.com
   ```
   Open Mailpit at <http://127.0.0.1:8025>, follow the link in the email and set a password.

## Run
```sh
make dev
```
This starts Mailpit, the API (`127.0.0.1:8080`) and the web app. Open
<http://127.0.0.1:5173>. Ctrl-C stops the API and the web app; `docker compose down` stops
Mailpit. Nothing listens outside this machine.

Signup is invite-only: an operator invites people under Settings → Admin, and the invitation
email shows up in Mailpit.

## Test and lint
```sh
make test   # starts the Docker test database, then runs the API and web tests
make lint   # golangci-lint, ESLint, Prettier check, type-check
```

## Other commands
`make help` lists everything, including `make migrate-status`, `make migrate-down CONFIRM=yes`,
and `make run` / `make web-dev` to start the API or the web app on their own.

## Layout
```
api/                Go API
web/                React + Vite + TypeScript PWA
db/migrations/      SQL migrations, applied by make migrate
docker-compose.yml  Postgres for tests, Mailpit for email
```
