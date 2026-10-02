// Package schema has no code of its own. Its tests check the database schema made by the SQL
// migrations in db/migrations, at the SQL level, on the Docker test database (ADR-0033): roles,
// grants, row-level security, owner-scoped foreign keys, the default-category trigger and the
// cascade when a user is removed.
//
// The tests connect as the test superuser and switch roles inside a transaction exactly as
// WithUserTx and WithAuthTx do (SET LOCAL ROLE, set_config('app.user_id', ..., true)). Every
// transaction is rolled back, so the shared test database stays clean.
package schema
