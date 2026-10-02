-- 0001_init: roles, account tables, categories, transactions, row-level security.
--
-- Runs as the database owner role: the Neon owner in development (not a superuser; has
-- CREATEROLE and BYPASSRLS), the `postgres` superuser in the Docker test database (ADR-0026,
-- ADR-0027). Nothing here runs as `app_user` or `app_auth`.
--
-- How the API reaches the data (ADR-0019, ADR-0034):
--   - `app_user`: every request on a user's own data, inside WithUserTx
--     (`SET LOCAL ROLE app_user` + `app.user_id`). Row-level security keeps each user to their rows.
--   - `app_auth`: login and operator code, inside WithAuthTx. Account tables only; no access at all
--     to financial tables (categories, transactions).
--   - The owner role (this migration) bypasses RLS and is used only by migrations.

-- +goose Up

-- Every unqualified name below means schema `public` (pg_catalog is always searched first).
set local search_path = public;

---------------------------------------------------------------------------------------------------
-- 1. Extension and schema
--    citext: case-insensitive text, so `users.email` is unique ignoring case.
--    Schema `app`: helper functions, kept apart from the tables.
---------------------------------------------------------------------------------------------------
create extension if not exists citext with schema public;

create schema app;

---------------------------------------------------------------------------------------------------
-- 2. Roles
--    Roles belong to the whole server, not to one database, so they are created only when missing.
--    Two databases on the same server may migrate at the same time (for example the test database
--    and a throw-away one): the loser of that race gets duplicate_object or unique_violation, which
--    means the role now exists, so it is ignored.
--    Both roles: cannot log in, no superuser, cannot create databases or roles, no BYPASSRLS, and
--    NOINHERIT. The API connects as the owner role and switches with `SET LOCAL ROLE`.
---------------------------------------------------------------------------------------------------
-- +goose StatementBegin
do $$
begin
  if not exists (select from pg_catalog.pg_roles where rolname = 'app_user') then
    create role app_user nologin nosuperuser nocreatedb nocreaterole noinherit noreplication nobypassrls;
  end if;
exception
  when duplicate_object or unique_violation then
    raise notice 'role app_user was created at the same time by another migration; keeping it';
end
$$;
-- +goose StatementEnd

-- +goose StatementBegin
do $$
begin
  if not exists (select from pg_catalog.pg_roles where rolname = 'app_auth') then
    create role app_auth nologin nosuperuser nocreatedb nocreaterole noinherit noreplication nobypassrls;
  end if;
exception
  when duplicate_object or unique_violation then
    raise notice 'role app_auth was created at the same time by another migration; keeping it';
end
$$;
-- +goose StatementEnd

-- The owner role may switch to the two roles (SET true) but does not get their rights by default
-- (INHERIT false). Nothing is granted the other way: neither role is a member of any other role.
grant app_user, app_auth to current_user with inherit false, set true;

grant usage on schema app to app_user;

---------------------------------------------------------------------------------------------------
-- 3. Who is the current user?
--    WithUserTx sets `app.user_id` with `set_config(..., true)` (local to the transaction).
--    After such a transaction ends the setting reads back as '' instead of NULL; nullif turns that
--    into NULL ("no user"), so RLS simply matches no rows instead of failing on a bad uuid cast.
---------------------------------------------------------------------------------------------------
-- +goose StatementBegin
create function app.current_user_id() returns uuid
  language sql
  stable
as $$
  select nullif(pg_catalog.current_setting('app.user_id', true), '')::uuid
$$;
-- +goose StatementEnd

revoke execute on function app.current_user_id() from public;
grant execute on function app.current_user_id() to app_user;

---------------------------------------------------------------------------------------------------
-- 4. Account tables (reached only through app_auth; ADR-0034, ADR-0039)
---------------------------------------------------------------------------------------------------

-- One row per account. password_hash stays '' until the invite is accepted (ADR-0025).
create table users (
  id            uuid        primary key default uuidv7(),
  email         citext      not null unique,
  password_hash text        not null default '',
  created_at    timestamptz not null default now(),
  disabled_at   timestamptz
);

-- One row per user, same id as users. Go creates it with the account (ADR-0036).
-- role: 'user' or 'operator'; set only by the operator command (ADR-0035). app_user cannot change it.
create table profiles (
  id           uuid        primary key references users (id) on delete cascade,
  display_name text        not null default '',
  role         text        not null default 'user' check (role in ('user', 'operator')),
  created_at   timestamptz not null default now()
);

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

---------------------------------------------------------------------------------------------------
-- 5. Financial tables (reached only through app_user; every row has owner_id; ADR-0014, ADR-0039)
---------------------------------------------------------------------------------------------------

-- A user's categories. kind never changes after creation; categories are archived, never deleted.
-- unique (owner_id, id) is the target of the owner-scoped foreign key from transactions.
create table categories (
  id         uuid        primary key default uuidv7(),
  owner_id   uuid        not null references users (id) on delete cascade,
  name       text        not null check (btrim(name) <> ''),
  icon       text        not null default '',
  kind       text        not null check (kind in ('expense', 'income')),
  archived   boolean     not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint categories_owner_id_id_key unique (owner_id, id)
);
-- Names are unique per user among non-archived categories, ignoring case.
create unique index categories_owner_name_idx on categories (owner_id, lower(name)) where not archived;

-- Income and expense entries. amount is always positive; income or expense comes from the
-- category's kind. The client normally sends id (ADR-0040). Only THB for now, enforced by the API
-- (ADR-0041).
create table transactions (
  id          uuid          primary key default uuidv7(),
  owner_id    uuid          not null references users (id) on delete cascade,
  amount      numeric(12,2) not null check (amount > 0),
  currency    char(3)       not null default 'THB',
  occurred_on date          not null,
  merchant    text          not null default '',
  category_id uuid          not null,
  note        text          not null default '',
  source      text          not null default 'manual' check (source in ('manual', 'text', 'scan', 'csv')),
  raw_input   text          not null default '',
  created_at  timestamptz   not null default now(),
  updated_at  timestamptz   not null default now(),
  -- Owner-scoped: a transaction can only point at a category of the same owner. Foreign-key checks
  -- ignore RLS, so this is what stops a row pointing at another user's category.
  -- Default NO ACTION, not RESTRICT: removing a user deletes their categories and transactions in
  -- the same statement, and RESTRICT would fail that.
  constraint transactions_category_fk foreign key (owner_id, category_id) references categories (owner_id, id)
);
create index transactions_owner_date_idx on transactions (owner_id, occurred_on desc, id desc);
create index transactions_owner_category_idx on transactions (owner_id, category_id);

---------------------------------------------------------------------------------------------------
-- 6. Triggers
---------------------------------------------------------------------------------------------------

-- updated_at follows every update of categories and transactions.
-- +goose StatementBegin
create function app.set_updated_at() returns trigger
  language plpgsql
as $$
begin
  new.updated_at := now();
  return new;
end
$$;
-- +goose StatementEnd

create trigger categories_set_updated_at before update on categories
  for each row execute function app.set_updated_at();
create trigger transactions_set_updated_at before update on transactions
  for each row execute function app.set_updated_at();

-- Default categories for every new account (ADR-0036, ADR-0032).
-- The account is created by app_auth, which has no rights on categories (ADR-0034), so this
-- function runs with the rights of its owner (SECURITY DEFINER). search_path is empty and every
-- name is fully qualified, so nothing can be slipped in through the search path.
-- Icons are chosen later (PLAN-0002 T9). The profile row is created by Go, not here.
-- +goose StatementBegin
create function app.seed_default_categories() returns trigger
  language plpgsql
  security definer
  set search_path = ''
as $$
begin
  insert into public.categories (owner_id, name, icon, kind) values
    (new.id, 'Food',                '', 'expense'),
    (new.id, 'Groceries',           '', 'expense'),
    (new.id, 'Transport',           '', 'expense'),
    (new.id, 'Bills & Utilities',   '', 'expense'),
    (new.id, 'Shopping',            '', 'expense'),
    (new.id, 'Health',              '', 'expense'),
    (new.id, 'Education',           '', 'expense'),
    (new.id, 'Family support',      '', 'expense'),
    (new.id, 'Donations / Tamboon', '', 'expense'),
    (new.id, 'Entertainment',       '', 'expense'),
    (new.id, 'Other',               '', 'expense'),
    (new.id, 'Salary',              '', 'income'),
    (new.id, 'Other income',        '', 'income');
  return null;
end
$$;
-- +goose StatementEnd

revoke execute on function app.seed_default_categories() from public;

create trigger users_seed_default_categories after insert on users
  for each row execute function app.seed_default_categories();

---------------------------------------------------------------------------------------------------
-- 7. Row-level security
--    Enabled on every table (not FORCEd: the owner role bypasses it, and only migrations use it).
--    A role with no policy on a table sees no rows there.
---------------------------------------------------------------------------------------------------
alter table users          enable row level security;
alter table profiles       enable row level security;
alter table sessions       enable row level security;
alter table email_tokens   enable row level security;
alter table login_attempts enable row level security;
alter table categories     enable row level security;
alter table transactions   enable row level security;

-- Account tables: app_auth sees every row (login and operator code need that).
create policy auth_all on users          for all to app_auth using (true) with check (true);
create policy auth_all on profiles       for all to app_auth using (true) with check (true);
create policy auth_all on sessions       for all to app_auth using (true) with check (true);
create policy auth_all on email_tokens   for all to app_auth using (true) with check (true);
create policy auth_all on login_attempts for all to app_auth using (true) with check (true);

-- Profiles: a user reads and renames only their own profile.
create policy own_select on profiles for select to app_user
  using (id = app.current_user_id());
create policy own_update on profiles for update to app_user
  using (id = app.current_user_id())
  with check (id = app.current_user_id());

-- Financial tables: a user sees and writes only their own rows.
create policy owner_all on categories for all to app_user
  using (owner_id = app.current_user_id())
  with check (owner_id = app.current_user_id());
create policy owner_all on transactions for all to app_user
  using (owner_id = app.current_user_id())
  with check (owner_id = app.current_user_id());

---------------------------------------------------------------------------------------------------
-- 8. Grants
--    Column lists on UPDATE lock the columns a role must never change (profiles.role,
--    categories.kind, owner_id everywhere). Anything not granted here is denied.
---------------------------------------------------------------------------------------------------

-- app_auth: the five account tables. Nothing on categories or transactions.
grant select, insert, update, delete on users, profiles, sessions, email_tokens, login_attempts to app_auth;

-- app_user: own profile (rename only), own categories (no delete: they are archived), own
-- transactions. Nothing on users, sessions, email_tokens or login_attempts.
grant select on profiles to app_user;
grant update (display_name) on profiles to app_user;

grant select, insert on categories to app_user;
grant update (name, icon, archived) on categories to app_user;

grant select, insert, delete on transactions to app_user;
grant update (amount, currency, occurred_on, merchant, category_id, note) on transactions to app_user;

-- +goose Down

set local search_path = public;

-- Tables, with their triggers, policies, indexes and the grants on them.
drop table transactions;
drop table categories;
drop table login_attempts;
drop table email_tokens;
drop table sessions;
drop table profiles;
drop table users;

-- Functions, schema and extension.
drop function app.seed_default_categories();
drop function app.set_updated_at();
drop function app.current_user_id();
drop schema app;
drop extension citext;

-- Every privilege app_user and app_auth had in this database was on an object dropped above, so
-- none is left here. The roles themselves belong to the whole server: drop each one only if no
-- other database still uses it; otherwise keep it (and the owner's membership in it).
-- +goose StatementBegin
do $$
declare
  r text;
begin
  foreach r in array array['app_user', 'app_auth'] loop
    begin
      execute pg_catalog.format('drop role if exists %I', r);
    exception
      when dependent_objects_still_exist then
        raise notice 'role % is still used in another database; keeping it', r;
    end;
  end loop;
end
$$;
-- +goose StatementEnd
