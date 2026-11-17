# Server rendering of web pages

Status: decided 2026-11-18; the Go side is built (webapi/README
"Pages"), the render service not deployed yet. This document details
[web-go.md](web-go.md) section 4 and is the shared specification of
the render protocol between the Go service and the render service.
Decisions:
[decisions/2026-11-18-web-ssr.md](decisions/2026-11-18-web-ssr.md).
Backend items: `go-web-pages` (Go side, tested with a stand-in
renderer) and `ssr-render-service` (deployment); the React side is the
React application in `web/app/`.

## 1. Requirements

- Every page has a real URL (`/users/<id>/history`,
  `/groups/<id>/chat?at=<msg>`), never a `#` fragment route.
- The first response is complete HTML, for search engines and
  scrapers, above all the public chat pages of groups exposed through
  MCP. React then takes over in the browser: navigation without
  reloads, live views updating in place.
- One render path for every page, signed in or public.
- Access rules (roles, federations, hidden things answering 404, see
  [web-accounts.md](web-accounts.md)) are enforced in one place: the Go
  service.
- Nothing is fetched from a CDN or the network at run time.

## 2. Overview

```
browser --> nginx --+-- /api/...          --> Go service (JSON)
                    +-- migrated pages    --> Go service (HTML)
                    +-- everything else   --> Drogon (until retired)

Go service, for a page:
  1. authenticate, apply the request scope (404 for hidden things)
  2. run the route's loader -> props (plain values)
  3. POST /render {route, url, status, props, viewer}
         over a Unix socket --> render service (Node, React)
                            <-- {head, html}
  4. assemble the document: head, markup, embedded page data,
     asset links -> response
  5. renderer down, slow or failing -> same document, empty markup
     (the browser renders it from the same embedded data)

browser navigation:
  GET /api/page/<path>  -> the same loader's data as JSON
```

The render service has no database access, cookies, CSRF tokens or
secrets, and no network: it turns route and props into markup and
nothing else. Everything it receives was already authorized by Go, so
a fault in it cannot widen what a user may see.

## 3. Routes and loaders (Go)

- The Go service owns the route table: path pattern, route id,
  loader. A route id is a stable lower-case name (`users`, `user`,
  `user_history`, `group`, `group_chat`, `files`, `settings_accounts`,
  ...), shared with the React application's route registry.
- A loader takes the authenticated request (or the anonymous viewer of
  a public page) and returns the page's props as plain values. Strings
  are raw text; React escapes them. Nothing is pre-escaped as the
  Drogon templates were.
- Anything the request scope hides answers exactly like something that
  does not exist: status 404 and the `not_found` route, same markup,
  same timing class. A hidden group and a missing group are
  indistinguishable.
- List state (search, filters, sort, page) is in the query string, so
  `url` alone determines the props.

## 4. Render protocol (Go to render service), version 1

### 4.1 Transport

- HTTP/1.1 over a Unix domain socket. Path from `RENDER_SOCKET`,
  default `/run/tgloggerd-render/render.sock`, in a directory shared
  by the two containers and by nothing else. The render service
  creates the socket (removing a stale one at start), mode 0660, group
  shared with the Go service.
- `POST /render` renders a page; `GET /health` reports readiness.
- Bodies are UTF-8 JSON (`Content-Type: application/json`). A request
  body is at most 8 MiB, a response body at most 8 MiB; larger is an
  error on either side.
- One request per render; keep-alive connections may be reused.

### 4.2 Request: `POST /render`

```json
{
  "v": 1,
  "request_id": "01JD3K7Q9S...",
  "build": "a1b2c3d4",
  "route": "group",
  "url": "/groups/-1001669109501?tab=admins",
  "status": 200,
  "props": { "...": "the loader's result" },
  "viewer": {
    "signed_in": true,
    "account": { "id": 3, "username": "researcher", "name": "Research team", "role": "member" },
    "now": "2026-11-18T09:14:03Z",
    "timezone": "Asia/Jakarta",
    "date_format": "24h",
    "lang": "en",
    "theme": "system",
    "sidebar": "open",
    "rows_per_page": 25,
    "nav": ["dashboard", "users", "groups", "group_messages", "files"],
    "chat": { "auto_load": true, "live": true, "media": "auto", "show_deleted": true, "show_edited": true, "show_service": true }
  }
}
```

- `v`: protocol version. The render service answers 400 to a version
  it does not know.
- `request_id`: from the Go request, for correlated logs.
- `build`: the build id of the client assets the Go service will link
  (section 6). The render service answers 409 if it was built from a
  different commit, because its markup would not hydrate.
- `route`, `url`, `status`: as decided by Go. `status` is 200 or 404
  (other errors are Go's own error pages); the render service never
  changes it.
- `props`: the loader's result, the same value `/api/page` returns.
- `viewer`: what rendering needs to match the browser exactly. `now`
  is the server's clock, used for relative times ("5m ago") on both
  sides so the first client render matches. `nav` lists the pages the
  menu shows; hidden pages are absent, never disabled. For a signed-out
  visitor `signed_in` is false, `account` and `nav` are absent and the
  site defaults apply. Fields are added, never repurposed; unknown
  fields are ignored.

### 4.3 Response

200, `application/json`:

```json
{
  "v": 1,
  "build": "a1b2c3d4",
  "head": "<title>GNU/Weeb - tgloggerd</title><meta name=\"description\" content=\"...\">",
  "html": "<div class=\"app\">...</div>",
  "ms": 14
}
```

- `head`: elements for `<head>` only: `title`, `meta` (description,
  robots, Open Graph), `link rel=canonical`. No scripts, no styles.
- `html`: the markup React rendered for the application root.
- `ms`: render time, for logs.

Errors: 400 (bad request or unknown `v`), 409 (build mismatch), 422
(unknown `route`), 500 (a component threw), each with
`{"v": 1, "error": "<code>", "detail": "<text>"}`. Every error, a
timeout or a refused connection makes Go fall back (section 5).

### 4.4 `GET /health`

`200 {"v": 1, "ok": true, "build": "a1b2c3d4", "uptime_s": 3600}`.
Used by the compose health check and by the Go service's status page.

### 4.5 Timeouts

- Go waits at most `RENDER_TIMEOUT` (default 500 ms) for connect and
  response together.
- After a refused connection or a timeout, Go skips the render service
  for `RENDER_BACKOFF` (default 2 s) and serves the fallback directly,
  so a dead renderer costs no latency.
- The render service runs `renderToString` in a pool of
  `RENDER_WORKERS` processes (default 2); streaming can come later
  without changing the protocol.

## 5. The document (assembled by Go)

Go assembles every page response, rendered or not, from one fixed
template, so the success path and the fallback differ only in the
markup:

```html
<!doctype html>
<html lang="{viewer.lang}" data-theme="{viewer.theme unless system}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="csrf-token" content="{session CSRF token, signed-in only}">
{render response head, or a title-only head in the fallback}
<link rel="stylesheet" href="/app/assets/index-<hash>.css">
<link rel="modulepreload" href="/app/assets/...">
</head>
<body>
<div id="root" data-ssr="1">{render response html, empty in the fallback}</div>
<script type="application/json" id="__tgd">{page data}</script>
<script type="module" src="/app/assets/entry-client-<hash>.js"></script>
</body>
</html>
```

- Page data: `{"v": 1, "route", "url", "status", "props", "viewer"}`,
  the same values sent to the render service, minus nothing and plus
  nothing.
- Serialisation is Go's `encoding/json` with HTML escaping on (the
  default): `<`, `>` and `&` become `\u003c`, `\u003e`, `\u0026`, and
  U+2028 and U+2029 are escaped, so no string can close the script
  element or break JavaScript parsing. The browser reads it with
  `JSON.parse(el.textContent)`, never `eval` or `innerHTML`.
- `data-ssr="0"` marks the fallback: the client calls `createRoot`
  instead of `hydrateRoot`.
- The CSRF token is added by Go only; the render service never sees it.
- A Content-Security-Policy without inline scripts holds: the page data
  script is `type="application/json"`, not executable.
- Caching: signed-in responses are `Cache-Control: private, no-store`.
  A public page requested without a session cookie may be
  `public, max-age=60`; with one, it is private.

## 6. Assets and builds

- `vite build` produces the client bundle in `web/app/dist/client/`
  (content-hashed files under `/app/assets/`) and the server bundle in
  `web/app/dist/server/`, from the same commit, with the same build id
  (the short commit hash, or a hash of the inputs).
- The Go service reads `dist/client/.vite/manifest.json` at start for
  the entry script, styles and preloads. The manifest carries no
  build id, so the Go service is given it (`WEBAPI_APP_BUILD`, the
  `TGD_BUILD` of the build). It serves
  `/app/assets/` with `Cache-Control: public, max-age=31536000,
  immutable` (or nginx does).
- Fonts and every other asset come from npm packages inside the bundle;
  dependencies are pinned in `web/app/package-lock.json` and installed
  with `npm ci` at build time.

## 7. The browser side

- The entry reads the page data, picks the component for `route` from
  the route registry, and hydrates (or renders, in the fallback).
- Navigation to a path calls `GET /api/page/<path>` (with the query
  string) through the generated client. The answer is the same page
  data object. 404 answers `{"route": "not_found", "status": 404}` so
  the not-found page renders in place. A path the Go service does not
  serve as a React page yet answers `{"reload": true}` and the browser
  does a full navigation, so links into Drogon pages keep working
  during the migration without the client knowing which paths moved.
- The document title and head on navigation come from the same route
  module function the render service used.
- Live views (chat, dashboard) use the API's cursors at first; a
  Server-Sent Events endpoint can follow under `/api/`.

## 8. The render service

- Code: `web/app/server/` (a `node:http` server on the socket, no
  framework) plus the React application's server entry. The route
  registry is shared with the client build.
- Deployment (`ssr-render-service`): Node pinned, its own compose
  service `render`, the socket directory as its only shared volume, no
  network (`network_mode: none`), read-only file system, non-root user,
  no secrets in its environment, restarted on failure. Logs per render:
  request id, route, status, bytes, ms.
- A stand-in renderer for Go tests answers the protocol with fixed
  markup (`go-web-pages`).

## 9. Alternatives considered

- **Node in front of the Go service**: Node would receive every request
  and call Go for data; the access rules would sit on both sides of a
  hop, or Node would need database access. Rejected.
- **A JavaScript engine embedded in Go** (goja, or V8 through cgo):
  goja runs React's server renderer far slower than Node and lags its
  language support; V8 brings cgo and a heavy build into a
  memory-safe service. Rejected; the protocol would let one replace
  Node later.
- **Client-only rendering**: no HTML for crawlers. Rejected.
- **Go templates hydrated by React**: every page written twice, in two
  languages, matching byte for byte. Rejected.
