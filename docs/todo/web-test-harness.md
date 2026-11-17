# web-test-harness

A shared test setup for the web: a database per test with dummy data,
run in parallel, from one command.

## Why

Every web feature ships with its tests. The Go tests that need MySQL
share one database and empty its tables, so they cannot run in
parallel, every agent starts its own MySQL by hand, and there is no
dummy data to test pages against.

## Scope

- `docker-compose.test.yml`: a MySQL for tests only (tmpfs storage,
  no published port by default, its own project name), never the
  production one.
- `webapi/internal/testdb`: gives each test its own fresh database
  (created and dropped around it), with the daemon's and the web's
  migrations applied and optional seed fixtures loaded, so `go test`
  across packages and `t.Parallel()` are safe. Skips cleanly when no
  test MySQL is configured.
- Committed dummy data: seed fixtures for users, groups, messages,
  files and web accounts, built for tests and for the stand-in, with
  nothing from production.
- `scripts/test-web.sh`: starts the test MySQL, runs gofmt, go vet
  and go test (with -race) for webapi and the web/app checks
  (check:api, typecheck, tests), and stops what it started.
- The existing Go tests that empty shared tables move to testdb.

## Design notes

- Owner rule (2026-11-18): every feature ships with its tests, with
  dummy data, and the tests run in parallel.
- Migrations are applied once per run into a template database and
  each test's database is copied from it, so a test starts fast.

## Depends on

Nothing.

## Done when

- `scripts/test-web.sh` passes from a clean checkout with only Docker,
  and running the Go tests twice at once does not interfere.
