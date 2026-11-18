# go-web-pages

Route the migrated page paths through nginx to the Go service, which
now serves the page documents, `GET /api/page/<path>`, the render
client and the browser-rendered fallback (webapi/README "Pages").

## Why

Pages need real URLs whose first response is full HTML, then React
takes over (docs/design/web-go.md section 4). The Go side is built and
tested against a stand-in renderer and the real render service, but
nginx still sends every page path to the Drogon web.

## Scope

- nginx sends each page path whose loader is in the Go service's page
  table (`defaultPages` in webapi/internal/server/pages.go) to the Go
  service, and `/app/assets/` with it; every other page path stays on
  Drogon. Today that is `/login` only.
- The sign-in flow is checked end to end once `/login` moves: the
  Drogon pages send signed-out visitors to `/login?next=`, and the
  React sign-in page goes on to `next` after `POST /api/login`.

## Design notes

- Loaders for the other pages come with the ports that bring their
  data (`go-port-admin`, `web-roles`, `web-user-settings`); each adds
  its loader to the page table, and the path then follows here.
- The password gate comes before the loaders; there is no email
  address gate (decisions/2026-11-18-email-optional.md).
- The build id comes from the bundle's `build.json`, which the build
  writes for the client bundle and the render entry alike;
  `WEBAPI_APP_BUILD` overrides it.
- Needs the render service deployed (`ssr-render-service`) before the
  route moves.

## Depends on

Nothing.

## Done when

- `/login` is served by the Go service through nginx, rendered by the
  deployed render service, and falls back to browser rendering when
  that service is stopped.
