-- 0004_transactions: the transactions table with its indexes, owner-scoped foreign key,
-- row-level security, policy, grants and updated_at trigger.
-- Relies on 0001 (role app_user, app.current_user_id(), app.set_updated_at()), 0002 (users) and
-- 0003 (categories and its unique (owner_id, id)).
--
-- Financial tables are reached only through app_user; every row has owner_id (ADR-0014,
-- ADR-0039). app_auth has nothing on them.

-- +goose Up

-- Every unqualified name below means schema `public` (pg_catalog is always searched first).
set local search_path = public;

---------------------------------------------------------------------------------------------------
-- transactions
---------------------------------------------------------------------------------------------------

-- Income and expense entries. amount is always positive; income or expense comes from the
-- category's kind. The client normally sends id (ADR-0040).
-- currency is an ISO 4217 alphabetic code. THB only for now (ADR-0041); adding a currency is a
-- one-line migration that widens the check.
create table transactions (
  id          uuid          primary key default uuidv7(),
  owner_id    uuid          not null references users (id) on delete cascade,
  amount      numeric(12,2) not null check (amount > 0),
  currency    text          not null default 'THB' check (currency in ('THB')),
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
-- One category, newest first, without a sort; it also serves the foreign key above.
create index transactions_owner_category_idx on transactions (owner_id, category_id, occurred_on desc, id desc);

alter table transactions enable row level security;

-- A user sees and writes only their own rows.
create policy owner_all on transactions for all to app_user
  using (owner_id = app.current_user_id())
  with check (owner_id = app.current_user_id());

-- app_user: own transactions.
-- updated_at is in the list because GORM adds it to every update by itself; the BEFORE UPDATE
-- trigger overwrites whatever is sent, so this grants nothing real.
grant select, insert, delete on transactions to app_user;
grant update (amount, currency, occurred_on, merchant, category_id, note, updated_at) on transactions to app_user;

create trigger transactions_set_updated_at before update on transactions
  for each row execute function app.set_updated_at();

-- +goose Down

set local search_path = public;

-- The table, with its trigger, policy, indexes and the grants on it.
drop table transactions;
