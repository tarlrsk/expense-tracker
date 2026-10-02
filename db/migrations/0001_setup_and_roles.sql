-- 0001_setup_and_roles: version check, extension, schema `app`, closed-by-default settings, the
-- three roles and their memberships, the role check, and the helper functions the table files use.
-- Relies on nothing but Postgres 18 or later; every later file relies on this one.
--
-- Runs as the database owner role: the Neon owner in development (not a superuser; has
-- CREATEROLE and BYPASSRLS), the `postgres` superuser in the Docker test database (ADR-0026,
-- ADR-0027). Nothing here runs as `app_login`, `app_user` or `app_auth`.
--
-- How the API reaches the data (ADR-0019, ADR-0034):
--   - `app_login`: the role the API logs in as. It owns nothing and has no rights of its own; it
--     can only switch to the two roles below. A query outside WithUserTx / WithAuthTx therefore
--     sees nothing. Its password is set once with `make db-login-password`, never here.
--   - `app_user`: every request on a user's own data, inside WithUserTx
--     (`SET LOCAL ROLE app_user` + `app.user_id`). Row-level security keeps each user to their rows.
--   - `app_auth`: login and operator code, inside WithAuthTx. Account tables only; no access at all
--     to financial tables (categories, transactions).
--   - The owner role (these migrations) bypasses RLS and is used only by migrations.
--
-- Every table file (0002 onwards) keeps everything about its tables together: the table, its
-- indexes, row-level security, policies, grants and triggers.
--   - Row-level security is enabled on every table (not FORCEd: the owner role bypasses it, and
--     only migrations use it). A role with no policy on a table sees no rows there.
--   - Column lists on UPDATE lock the columns a role must never change (profiles.role,
--     categories.kind, owner_id everywhere). Anything not granted is denied.

-- +goose Up

-- Postgres 18 or later: the next files use uuidv7() for ids. Stop here, with nothing changed,
-- on an older server.
-- +goose StatementBegin
do $$
begin
  if pg_catalog.current_setting('server_version_num')::int < 180000 then
    raise exception 'these migrations need Postgres 18 or later; this server is %',
      pg_catalog.current_setting('server_version');
  end if;
end
$$;
-- +goose StatementEnd

-- Every unqualified name below means schema `public` (pg_catalog is always searched first).
set local search_path = public;

---------------------------------------------------------------------------------------------------
-- 1. Extension and schema
--    citext: case-insensitive text, so `users.email` is unique ignoring case.
--    Schema `app`: helper functions, kept apart from the tables.
---------------------------------------------------------------------------------------------------
create extension if not exists citext with schema public;

create schema app;

-- Closed by default (explicit, so it does not depend on the host's defaults):
--   - nobody but the owner may create objects in schema public;
--   - nobody but the owner may create temporary tables in this database (Postgres gives everyone
--     TEMP on a new database). The API's roles never need one. The database's name is only known
--     when this runs, hence the dynamic statement;
--   - functions this role creates from now on in this database are not executable by everyone;
--     each one is granted to the role that needs it. This comes after the citext extension on
--     purpose: app_user and app_auth must keep using citext's functions (= on emails).
--     A later migration that adds an extension must grant execute on its functions as needed.
revoke create on schema public from public;

-- +goose StatementBegin
do $$
begin
  execute pg_catalog.format('revoke temporary on database %I from public', pg_catalog.current_database());
end
$$;
-- +goose StatementEnd

alter default privileges revoke execute on functions from public;

---------------------------------------------------------------------------------------------------
-- 2. Roles
--    Roles belong to the whole server, not to one database, so they are created only when missing.
--    Two databases on the same server may migrate at the same time (for example the test database
--    and a throw-away one): the loser of that race gets duplicate_object or unique_violation, which
--    means the role now exists, so it is ignored.
--    app_user and app_auth: cannot log in, no superuser, cannot create databases or roles, no
--    BYPASSRLS, and NOINHERIT.
--    app_login: the same, except that it can log in. It is created without a password, so nobody
--    can log in as it until `make db-login-password` sets one. The API logs in as app_login and
--    switches with `SET LOCAL ROLE` inside each transaction.
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

-- +goose StatementBegin
do $$
begin
  if not exists (select from pg_catalog.pg_roles where rolname = 'app_login') then
    create role app_login login nosuperuser nocreatedb nocreaterole noinherit noreplication nobypassrls;
  end if;
exception
  when duplicate_object or unique_violation then
    raise notice 'role app_login was created at the same time by another migration; keeping it';
end
$$;
-- +goose StatementEnd

-- app_login may switch to the two roles (SET true) but does not get their rights by default
-- (INHERIT false), so outside a transaction it can read nothing. It is granted only when missing,
-- and a membership granted at the same moment by another migration is kept (same race as above).
-- The owner role is not granted the two roles: only migrations use it, and they never switch
-- (the next step removes what Postgres may grant it by itself).
-- +goose StatementBegin
do $$
declare
  r text;
begin
  foreach r in array array['app_user', 'app_auth'] loop
    if not exists (select from pg_catalog.pg_auth_members
                   where roleid = r::regrole and member = 'app_login'::regrole) then
      begin
        execute pg_catalog.format('grant %I to app_login with inherit false, set true', r);
      exception
        when unique_violation then
          raise notice 'app_login was granted % at the same time by another migration; keeping it', r;
      end;
    end if;
  end loop;
end
$$;
-- +goose StatementEnd

-- The role running this migration must not be able to act as the three roles either. When a role
-- that is not a superuser creates a role, Postgres makes it a member with ADMIN (so it can manage
-- and drop the role), and, if the server or that role sets `createrole_self_grant` (for example
-- to 'set, inherit'), adds a second membership with SET and/or INHERIT. Remove SET and INHERIT
-- from every membership this role holds in the three roles; the ADMIN membership stays. Nothing
-- happens when there is none (a superuser runs this, createrole_self_grant is empty as by default,
-- or the roles already existed).
-- A membership granted by another role that this role may not revoke is left for the check below,
-- which then stops the migration.
-- +goose StatementBegin
do $$
declare
  m record;
begin
  for m in
    select r.rolname as role, g.rolname as grantor, a.admin_option
    from pg_catalog.pg_auth_members a
    join pg_catalog.pg_roles r on r.oid = a.roleid
    join pg_catalog.pg_roles g on g.oid = a.grantor
    where a.member = (select oid from pg_catalog.pg_roles where rolname = current_user)
      and r.rolname in ('app_user', 'app_auth', 'app_login')
      and (a.set_option or a.inherit_option)
  loop
    begin
      if m.grantor = current_user and not m.admin_option then
        -- The membership createrole_self_grant made: SET and/or INHERIT only. Remove all of it.
        execute pg_catalog.format('revoke %I from %I granted by %I', m.role, current_user, m.grantor);
      else
        execute pg_catalog.format('revoke inherit option for %I from %I granted by %I', m.role, current_user, m.grantor);
        execute pg_catalog.format('revoke set option for %I from %I granted by %I', m.role, current_user, m.grantor);
      end if;
    exception
      when insufficient_privilege then
        raise notice 'cannot remove the membership of % in % granted by %; the role check will stop the migration',
          current_user, m.role, m.grantor;
    end;
  end loop;
end
$$;
-- +goose StatementEnd

-- A role that already existed on the server is not trusted blindly: an app_user with BYPASSRLS,
-- for example, would silently void every policy in the later files. Stop the migration if:
--   - app_user or app_auth can log in, has any special power, inherits, or is a member of another
--     role; or has a member other than app_login (and the role running this migration);
--   - app_login cannot log in, has any special power or inherits, or is a member of anything but
--     app_user and app_auth, each with SET and without INHERIT or ADMIN; or has any member other
--     than the role running this migration;
--   - any of the three may create temporary tables in this database;
--   - the role running this migration can SET ROLE to, or inherits, any of the three (see above;
--     its ADMIN membership is allowed).
-- The error names the role and what is wrong, including the names of unexpected members.
-- +goose StatementBegin
do $$
declare
  r record;
  bad text;
  me oid := (select oid from pg_catalog.pg_roles where rolname = current_user);
  login oid := (select oid from pg_catalog.pg_roles where rolname = 'app_login');
  members text;
begin
  for r in
    select oid, rolname, rolcanlogin, rolsuper, rolbypassrls, rolcreaterole, rolcreatedb,
           rolreplication, rolinherit
    from pg_catalog.pg_roles
    where rolname in ('app_user', 'app_auth', 'app_login')
  loop
    if r.rolname = 'app_login' then
      bad := pg_catalog.concat_ws(', ',
        case when not r.rolcanlogin then 'NOLOGIN' end,
        case when r.rolsuper        then 'SUPERUSER' end,
        case when r.rolbypassrls    then 'BYPASSRLS' end,
        case when r.rolcreaterole   then 'CREATEROLE' end,
        case when r.rolcreatedb     then 'CREATEDB' end,
        case when r.rolreplication  then 'REPLICATION' end,
        case when r.rolinherit      then 'INHERIT' end,
        case when exists (select from pg_catalog.pg_auth_members m
                          where m.member = r.oid
                            and (m.roleid not in ('app_user'::regrole, 'app_auth'::regrole)
                                 or m.inherit_option or not m.set_option or m.admin_option))
             then 'a membership other than SET on app_user and app_auth' end,
        case when (select count(distinct m.roleid) from pg_catalog.pg_auth_members m
                   where m.member = r.oid) <> 2
             then 'not a member of both app_user and app_auth' end);
      select string_agg(distinct pg_catalog.quote_ident(u.rolname), ', ') into members
      from pg_catalog.pg_auth_members m join pg_catalog.pg_roles u on u.oid = m.member
      where m.roleid = r.oid and m.member <> me;
      if members is not null then
        bad := pg_catalog.concat_ws(', ', nullif(bad, ''), 'members (' || members || ')');
      end if;
    else
      bad := pg_catalog.concat_ws(', ',
        case when r.rolcanlogin    then 'LOGIN' end,
        case when r.rolsuper       then 'SUPERUSER' end,
        case when r.rolbypassrls   then 'BYPASSRLS' end,
        case when r.rolcreaterole  then 'CREATEROLE' end,
        case when r.rolcreatedb    then 'CREATEDB' end,
        case when r.rolreplication then 'REPLICATION' end,
        case when r.rolinherit     then 'INHERIT' end,
        case when exists (select from pg_catalog.pg_auth_members m where m.member = r.oid)
             then 'member of another role' end);
      select string_agg(distinct pg_catalog.quote_ident(u.rolname), ', ') into members
      from pg_catalog.pg_auth_members m join pg_catalog.pg_roles u on u.oid = m.member
      where m.roleid = r.oid and m.member not in (login, me);
      if members is not null then
        bad := pg_catalog.concat_ws(', ', nullif(bad, ''), 'members other than app_login (' || members || ')');
      end if;
    end if;
    if pg_catalog.has_database_privilege(r.oid, pg_catalog.current_database(), 'TEMPORARY') then
      bad := pg_catalog.concat_ws(', ', nullif(bad, ''), 'TEMP on this database');
    end if;
    if bad <> '' then
      raise exception 'role % already exists with %; fix or drop the role, then migrate again',
        r.rolname, bad;
    end if;
    if exists (select from pg_catalog.pg_auth_members m
               where m.roleid = r.oid and m.member = me and (m.set_option or m.inherit_option)) then
      raise exception 'role % (running this migration) can still SET ROLE to or inherit %; '
        'revoke that membership, then migrate again', current_user, r.rolname;
    end if;
  end loop;
end
$$;
-- +goose StatementEnd

-- app_login gets nothing else: no grant on any table, no USAGE on schema app, no EXECUTE on any
-- app function. Everything it can do, it does as app_user or app_auth.
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
-- 4. updated_at
---------------------------------------------------------------------------------------------------

-- updated_at follows every update of categories and transactions (their triggers are in 0003
-- and 0004).
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

revoke execute on function app.set_updated_at() from public;

-- +goose Down

set local search_path = public;

-- Functions, schema and extension. The later files' Down sections have already dropped every
-- table, trigger and policy that used them.
drop function app.set_updated_at();
drop function app.current_user_id();
drop schema app;
drop extension citext;

-- Undo the default for functions and the TEMP revoke made in section 1. "revoke create on schema
-- public" is kept: it is the default since Postgres 15 and opening the schema again would only
-- weaken it.
alter default privileges grant execute on functions to public;

-- +goose StatementBegin
do $$
begin
  execute pg_catalog.format('grant temporary on database %I to public', pg_catalog.current_database());
end
$$;
-- +goose StatementEnd

-- Every privilege app_user and app_auth had in this database was on an object dropped above or
-- by the later files' Down sections, so none is left here. The roles themselves belong to the
-- whole server: drop each one only if no other database still uses it, and only if this owner may
-- (it has ADMIN on roles it created); otherwise keep it.
-- app_login holds no privilege anywhere, so nothing would stop dropping it even while another
-- database on the server still needs it. It is dropped only when app_user and app_auth are both
-- gone, that is when no database uses this set of roles any more.
-- +goose StatementBegin
do $$
declare
  r text;
begin
  foreach r in array array['app_user', 'app_auth', 'app_login'] loop
    if r = 'app_login' and exists (select from pg_catalog.pg_roles
                                   where rolname in ('app_user', 'app_auth')) then
      raise notice 'role app_login is kept: app_user or app_auth is still used in another database';
      continue;
    end if;
    begin
      execute pg_catalog.format('drop role if exists %I', r);
    exception
      when dependent_objects_still_exist then
        raise notice 'role % is still used in another database; keeping it', r;
      when insufficient_privilege then
        raise notice 'role % cannot be dropped by this role (no ADMIN on it); keeping it', r;
    end;
  end loop;
end
$$;
-- +goose StatementEnd
