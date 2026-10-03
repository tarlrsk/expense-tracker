-- 0003_categories: the categories table with its indexes, row-level security, policy, grants and
-- updated_at trigger, and the default categories every new account gets (a function and the
-- trigger on users that calls it).
-- Relies on 0001 (role app_user, app.current_user_id(), app.set_updated_at()) and 0002 (users).
--
-- Financial tables are reached only through app_user; every row has owner_id (ADR-0014,
-- ADR-0039). app_auth has nothing on them.

-- +goose Up

-- Every unqualified name below means schema `public` (pg_catalog is always searched first).
set local search_path = public;

---------------------------------------------------------------------------------------------------
-- categories
---------------------------------------------------------------------------------------------------

-- A user's categories. kind never changes after creation; categories are archived, never deleted.
-- name is stored trimmed and is never empty, so 'Food' and 'Food ' cannot both exist.
-- sort_order is the user's own order (no default: the API always sets it, new categories go to
-- the end). It is not unique and not indexed: a user has a few dozen categories, and ties are
-- broken by id.
-- unique (owner_id, id) is the target of the owner-scoped foreign key from transactions.
create table categories (
  id         uuid        primary key default uuidv7(),
  owner_id   uuid        not null references users (id) on delete cascade,
  name       text        not null check (name = btrim(name) and name <> ''),
  icon       text        not null default '',
  kind       text        not null check (kind in ('expense', 'income')),
  archived   boolean     not null default false,
  sort_order integer     not null check (sort_order >= 0),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint categories_owner_id_id_key unique (owner_id, id)
);
-- Names are unique per user among non-archived categories, ignoring case.
create unique index categories_owner_name_idx on categories (owner_id, lower(name)) where not archived;

alter table categories enable row level security;

-- A user sees and writes only their own rows.
create policy owner_all on categories for all to app_user
  using (owner_id = app.current_user_id())
  with check (owner_id = app.current_user_id());

-- app_user: own categories, no delete (they are archived).
-- updated_at is in the list because GORM adds it to every update by itself; the BEFORE UPDATE
-- trigger overwrites whatever is sent, so this grants nothing real.
grant select, insert on categories to app_user;
grant update (name, icon, archived, sort_order, updated_at) on categories to app_user;

create trigger categories_set_updated_at before update on categories
  for each row execute function app.set_updated_at();

---------------------------------------------------------------------------------------------------
-- Default categories for every new account
---------------------------------------------------------------------------------------------------

-- Default categories for every new account (ADR-0036, ADR-0032).
-- The account is created by app_auth, which has no rights on categories (ADR-0034), so this
-- function runs with the rights of its owner (SECURITY DEFINER). search_path is empty and every
-- name is fully qualified, so nothing can be slipped in through the search path.
-- sort_order follows the listed order. The icons are emoji, chosen in the design plan (ADR-0072).
-- The profile row is created by Go, not here.
-- +goose StatementBegin
create function app.seed_default_categories() returns trigger
  language plpgsql
  security definer
  set search_path = ''
as $$
begin
  insert into public.categories (owner_id, name, icon, kind, sort_order) values
    (new.id, 'Food',                '🍜', 'expense',   1),
    (new.id, 'Groceries',           '🛒', 'expense',   2),
    (new.id, 'Transport',           '🚌', 'expense',   3),
    (new.id, 'Bills & Utilities',   '💡', 'expense',   4),
    (new.id, 'Shopping',            '🛍️', 'expense',   5),
    (new.id, 'Health',              '💊', 'expense',   6),
    (new.id, 'Education',           '🎓', 'expense',   7),
    (new.id, 'Family support',      '👪', 'expense',   8),
    (new.id, 'Donations / Tamboon', '🙏', 'expense',   9),
    (new.id, 'Entertainment',       '🎬', 'expense',  10),
    (new.id, 'Other',               '📦', 'expense',  11),
    (new.id, 'Salary',              '💼', 'income',   12),
    (new.id, 'Other income',        '💰', 'income',   13);
  return null;
end
$$;
-- +goose StatementEnd

revoke execute on function app.seed_default_categories() from public;

create trigger users_seed_default_categories after insert on users
  for each row execute function app.seed_default_categories();

-- +goose Down

set local search_path = public;

-- The seed trigger on users and its function, then the table with its trigger, policy, indexes
-- and the grants on it.
drop trigger users_seed_default_categories on users;
drop function app.seed_default_categories();
drop table categories;
