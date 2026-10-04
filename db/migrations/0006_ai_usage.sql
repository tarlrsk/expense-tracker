-- 0006_ai_usage: the ai_usage table with its row-level security, policy and grants (ADR-0077).
-- Relies on 0001 (role app_user, app.current_user_id()) and 0002 (users).
--
-- Financial tables are reached only through app_user; every row has owner_id (ADR-0014,
-- ADR-0039). app_auth has nothing on them.

-- +goose Up

-- Every unqualified name below means schema `public` (pg_catalog is always searched first).
set local search_path = public;

---------------------------------------------------------------------------------------------------
-- ai_usage
---------------------------------------------------------------------------------------------------

-- How many AI calls a user made on one day, for the daily per-user limits. day is a calendar date
-- the API supplies in the app time zone (ADR-0042). Rows are kept: one small row per user and day,
-- a history for a later cost view.
-- Only parse_count for now; scan_count is added with scans (PLAN-0005), nothing ahead of need
-- (ADR-0059). No timestamps: the row is the day.
create table ai_usage (
  owner_id    uuid    not null references users (id) on delete cascade,
  day         date    not null,
  parse_count integer not null default 0 check (parse_count >= 0),
  primary key (owner_id, day)
);

alter table ai_usage enable row level security;

-- A user sees and writes only their own rows.
create policy owner_all on ai_usage for all to app_user
  using (owner_id = app.current_user_id())
  with check (owner_id = app.current_user_id());

-- app_user: own rows, counted up in place. No delete: rows are kept (ADR-0077).
grant select, insert on ai_usage to app_user;
grant update (parse_count) on ai_usage to app_user;

-- +goose Down

set local search_path = public;

-- The table, with its policy, index and the grants on it.
drop table ai_usage;
