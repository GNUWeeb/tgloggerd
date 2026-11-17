# ssr-render-service

Deploy the Node render service that turns a page's route and data
into HTML for the Go service.

## Why

Server rendering of the React pages needs Node
(docs/design/decisions/2026-11-18-web-ssr.md). The compose services
(`render`, `render-build`, the `render-socket` volume) are in
docker-compose.yml and tested end to end on a copy of the tree
(scripts/test-render-service.sh); the running deployment does not
have them yet.

## Scope

- Build and start them on the deployment, in the order
  web/app/README "Deployment" gives: `render-build`, then `render`,
  then the `webapi` service recreated (it mounts the socket volume and
  reads the new bundle at start).
- Leave `WEBAPI_APP_BUILD` empty or unset in the deployment's
  settings, so the Go service takes the build id from the bundle's
  `build.json`.

## Design notes

- The Go service's fallback covers the render service being down or
  not built yet: pages are then rendered in the browser.

## Depends on

Nothing.

## Done when

- The deployed stack renders a page through the service
  (`data-ssr="1"`); stopping the service makes pages fall back to
  browser rendering (`data-ssr="0"`) without errors.
