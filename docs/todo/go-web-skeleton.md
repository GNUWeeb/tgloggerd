# go-web-skeleton

Start the Go web service: module, build, container and routing beside
the Drogon web.

## Why

The web backend moves to Go (docs/design/web-go.md); everything else
in the migration builds on this service.

## Scope

- `webapi/` with module path `tgloggerd/webapi` (no repository host in
  it), `go.mod`, the few dependencies vendored under `webapi/vendor/`
  (standard library HTTP, `database/sql` with a MySQL driver,
  `golang.org/x/crypto`); builds with `-mod=vendor` and no network.
- Configuration from the same environment as the Drogon web (`WEB_*`,
  `WEB_DB_RO_*`, `WEB_DB_APP_*`), the two database pools, structured
  logging, a health endpoint, graceful shutdown on SIGTERM.
- The Go toolchain, pinned, in the build image; a `run.sh` target; a
  `webapi` compose service (`tgloggerd-webapi`) on the compose network.
- The nginx example in `web/README.web` routes `/api/` to it.
- `webapi/README` with build, run and layout.
- Not in scope: authentication and any endpoint beyond health.

## Design notes

- docs/design/web-go.md sections 2 and 3.
- Go's standard router (method and path patterns) is enough; add a
  third-party router only with a reason.

## Depends on

Nothing.

## Done when

- `go build -mod=vendor` succeeds in the image without network; the
  container starts, answers its health endpoint through nginx under
  `/api/`, and stops cleanly; nothing under `webapi/` names a
  repository host except vendored third-party import paths.
