-- 0002_account_tables: users, profiles, sessions, email_tokens and login_attempts, each with its
-- indexes, row-level security, policies and grants.
-- Relies on 0001: the citext extension, the roles app_auth and app_user, and
-- app.current_user_id() (the profiles policies).
--
-- Account tables are reached only through app_auth (ADR-0034, ADR-0039). On each of them:
--   - row-level security is enabled;
--   - policy auth_all: app_auth sees every row (login and operator code need that);
--   - app_auth may select, insert, update and delete. It has nothing on categories or
--     transactions.
-- app_user has nothing on users, sessions, email_tokens or login_attempts; on profiles it reads
-- and renames only its own row.

-- +goose Up

-- Every unqualified name below means schema `public` (pg_catalog is always searched first).
set local search_path = public;

---------------------------------------------------------------------------------------------------
-- users
---------------------------------------------------------------------------------------------------

-- One row per account. password_hash stays '' until the invite is accepted (ADR-0025).
create table users (
  id            uuid        primary key default uuidv7(),
  email         citext      not null unique,
  password_hash text        not null default '',
  created_at    timestamptz not null default now(),
  disabled_at   timestamptz
);

alter table users enable row level security;
create policy auth_all on users for all to app_auth using (true) with check (true);
grant select, insert, update, delete on users to app_auth;

---------------------------------------------------------------------------------------------------
-- profiles
---------------------------------------------------------------------------------------------------

-- One row per user, same id as users. Go creates it with the account (ADR-0036).
-- role: 'user' or 'operator'; set only by the operator command (ADR-0035). app_user cannot change it.
create table profiles (
  id           uuid        primary key references users (id) on delete cascade,
  display_name text        not null default '',
  role         text        not null default 'user' check (role in ('user', 'operator')),
  created_at   timestamptz not null default now()
);

alter table profiles enable row level security;
create policy auth_all on profiles for all to app_auth using (true) with check (true);

-- Profiles: a user reads and renames only their own profile.
create policy own_select on profiles for select to app_user
  using (id = app.current_user_id());
create policy own_update on profiles for update to app_user
  using (id = app.current_user_id())
  with check (id = app.current_user_id());

grant select, insert, update, delete on profiles to app_auth;

-- app_user: own profile, rename only.
grant select on profiles to app_user;
grant update (display_name) on profiles to app_user;

---------------------------------------------------------------------------------------------------
-- sessions
---------------------------------------------------------------------------------------------------

-- Login sessions: only the hash of the opaque bearer token is stored (ADR-0025, ADR-0037).
create table sessions (
  id           uuid        primary key default uuidv7(),
  user_id      uuid        not null references users (id) on delete cascade,
  token_hash   bytea       not null unique,
  created_at   timestamptz not null default now(),
  last_used_at timestamptz not null default now(),
  expires_at   timestamptz not null
);
create index sessions_user_id_idx on sessions (user_id);

alter table sessions enable row level security;
create policy auth_all on sessions for all to app_auth using (true) with check (true);
grant select, insert, update, delete on sessions to app_auth;

---------------------------------------------------------------------------------------------------
-- email_tokens
---------------------------------------------------------------------------------------------------

-- One-time links sent by email (invite / set password now; more purposes later).
create table email_tokens (
  id         uuid        primary key default uuidv7(),
  user_id    uuid        not null references users (id) on delete cascade,
  purpose    text        not null check (purpose in ('set_password')),
  token_hash bytea       not null unique,
  expires_at timestamptz not null,
  used_at    timestamptz,
  created_at timestamptz not null default now()
);
create index email_tokens_user_id_idx on email_tokens (user_id);

alter table email_tokens enable row level security;
create policy auth_all on email_tokens for all to app_auth using (true) with check (true);
grant select, insert, update, delete on email_tokens to app_auth;

---------------------------------------------------------------------------------------------------
-- login_attempts
---------------------------------------------------------------------------------------------------

-- One row per FAILED login, for the rate limit per email and per IP address (ADR-0037).
-- No foreign key: the email may not belong to any account.
create table login_attempts (
  id           uuid        primary key default uuidv7(),
  email        citext      not null,
  ip           inet        not null,
  attempted_at timestamptz not null default now()
);
create index login_attempts_email_idx on login_attempts (email, attempted_at);
create index login_attempts_ip_idx on login_attempts (ip, attempted_at);
-- Each failed login deletes the rows older than a day (ADR-0066).
create index login_attempts_attempted_at_idx on login_attempts (attempted_at);

alter table login_attempts enable row level security;
create policy auth_all on login_attempts for all to app_auth using (true) with check (true);
grant select, insert, update, delete on login_attempts to app_auth;

-- +goose Down

set local search_path = public;

-- Tables, with their policies, indexes and the grants on them. The seed trigger on users was
-- dropped by 0003's Down.
drop table login_attempts;
drop table email_tokens;
drop table sessions;
drop table profiles;
drop table users;
