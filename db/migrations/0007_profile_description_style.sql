-- 0007_profile_description_style: the description style of a profile and the column grant for it
-- (ADR-0081, PLAN-0003 T7). Relies on 0001 (role app_user) and 0002 (profiles).
--
-- The column belongs with profiles in 0002, but 0002 is already applied, so it is added here
-- (ADR-0064). profiles keeps its row-level security and policies from 0002: app_user already
-- reads and updates only its own row there; this file adds the right to update this one column.

-- +goose Up

-- Every unqualified name below means schema `public` (pg_catalog is always searched first).
set local search_path = public;

---------------------------------------------------------------------------------------------------
-- profiles.description_style
---------------------------------------------------------------------------------------------------

-- How freely the AI may write the description of an entry it reads (ADR-0081): as_typed (the
-- user's own words, the default), light_tidy (capitals and obvious typos fixed) or clean_name
-- (rewritten as a clean merchant name). Existing rows get the default.
alter table profiles
  add column description_style text not null default 'as_typed'
    constraint profiles_description_style_check
    check (description_style in ('as_typed', 'light_tidy', 'clean_name'));

-- app_user: own profile, now also the description style (ADR-0058).
grant update (description_style) on profiles to app_user;

-- +goose Down

set local search_path = public;

-- The column, with its check constraint and the column grant on it.
alter table profiles drop column description_style;
