-- 0005_merchant_rules: the merchant_rules table with its indexes, owner-scoped foreign key,
-- row-level security, policy, grants and updated_at trigger (ADR-0077).
-- Relies on 0001 (role app_user, app.current_user_id(), app.set_updated_at()), 0002 (users) and
-- 0003 (categories and its unique (owner_id, id)).
--
-- Financial tables are reached only through app_user; every row has owner_id (ADR-0014,
-- ADR-0039). app_auth has nothing on them.

-- +goose Up

-- Every unqualified name below means schema `public` (pg_catalog is always searched first).
set local search_path = public;

---------------------------------------------------------------------------------------------------
-- merchant_rules
---------------------------------------------------------------------------------------------------

-- A user's learned merchant -> category rules: one per merchant key.
-- merchant_key is what lookups match on, so it never changes once written (a rule for another key
-- is another row). merchant is the name as the user last wrote it, so a match can propose it.
-- Both are stored trimmed, are never empty and are at most 100 characters, the merchant limit of
-- ADR-0071.
-- A rule whose category is archived later stays; whether it is still used is up to the API.
create table merchant_rules (
  id           uuid        primary key default uuidv7(),
  owner_id     uuid        not null references users (id) on delete cascade,
  merchant_key text        not null check (merchant_key = btrim(merchant_key) and merchant_key <> ''
                                           and char_length(merchant_key) <= 100),
  merchant     text        not null check (merchant = btrim(merchant) and merchant <> ''
                                           and char_length(merchant) <= 100),
  category_id  uuid        not null,
  created_at   timestamptz not null default now(),
  updated_at   timestamptz not null default now(),
  -- One rule per key and user; the same key may exist for another user. It also serves the lookup.
  constraint merchant_rules_owner_key_key unique (owner_id, merchant_key),
  -- Owner-scoped: a rule can only point at a category of the same owner. Foreign-key checks
  -- ignore RLS, so this is what stops a row pointing at another user's category.
  -- Default NO ACTION, not RESTRICT: removing a user deletes their categories and rules in the
  -- same statement, and RESTRICT would fail that.
  constraint merchant_rules_category_fk foreign key (owner_id, category_id) references categories (owner_id, id)
);
-- Serves the foreign key above, which the unique index cannot (its second column is merchant_key):
-- every category deleted, as removing a user does, looks up the rules that point at it.
create index merchant_rules_owner_category_idx on merchant_rules (owner_id, category_id);

alter table merchant_rules enable row level security;

-- A user sees and writes only their own rows.
create policy owner_all on merchant_rules for all to app_user
  using (owner_id = app.current_user_id())
  with check (owner_id = app.current_user_id());

-- app_user: own rules. merchant_key is not updatable (see above).
-- updated_at is in the list because GORM adds it to every update by itself; the BEFORE UPDATE
-- trigger overwrites whatever is sent, so this grants nothing real.
grant select, insert, delete on merchant_rules to app_user;
grant update (merchant, category_id, updated_at) on merchant_rules to app_user;

create trigger merchant_rules_set_updated_at before update on merchant_rules
  for each row execute function app.set_updated_at();

-- +goose Down

set local search_path = public;

-- The table, with its trigger, policy, indexes and the grants on it.
drop table merchant_rules;
