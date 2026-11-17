# tgloggerd web — documentation

Developer docs for the tgloggerd web UI (Drogon + MySQL).

The web backend is moving to the Go service in `webapi/`
([../../docs/design/web-go.md](../../docs/design/web-go.md)). Its JSON
API under `/api/` and the file URLs `/files/<token>` are
described in [../../webapi/openapi.yaml](../../webapi/openapi.yaml)
(OpenAPI 3.1), the API contract: the React application's client is
generated from it, and a test keeps it in line with the routes the
service serves. Operations marked `x-tgd-status: planned` are designed
but not served yet. The pages below explain behaviour; a port moves or
updates its page in the same series, and the operation's shape is the
one in the description.

- [accounts.md](accounts.md) — the roles superadmin, admin and member, the
  request scope they give in the Go service, the accounts API, temporary
  passwords, and how the Drogon pages treat the roles.
- [contact-names.md](contact-names.md) — a contact's own name versus the
  name the logging account saved them under: what each column means, what
  the pages show, and the admin action "Look up real name" with its routes.
- [db-naming.md](db-naming.md) — database table naming convention: the
  per-platform prefixes (`telegram_`, `discord_`), what is and isn't prefixed,
  and the rule to follow when adding tables for a new messaging platform.
- [join-chat.md](join-chat.md) — the admin `/join-chat` page: preview and
  join a group or channel by invite link or username through the daemon's
  internal API, the outcomes it reports, and the `chat_join` audit rows.
- [mcp.md](mcp.md) — the `/mcp` Model Context Protocol endpoint: how exposure
  is gated to an admin-curated allowlist, bearer-token auth, the AND/OR/NOT
  filter grammar, the Telegram tools, and the two query-plan traps that make
  full-text search and recent-message listing fast.
- [media.md](media.md) — how `/files/<token>` serves files: tokens,
  `alias_of`, byte ranges, fetching files the store does not hold through
  the daemon, and the access reports sent to the daemon for eviction.
- [platform-forwarding.md](platform-forwarding.md) — the `/platform-fwd`
  section: how the per-direction pages are laid out and named, the Discord →
  Telegram endpoints in full, how bot tokens are kept write-only, the extra
  `GRANT`s needed, and why a used route is disabled rather than deleted.
- [search-api.md](search-api.md) — the advanced-search JSON API
  (`/v1/search/...`): the `[{c,o,v,n}]` condition grammar, operators, the users
  field registry (including history filters), response shape, limits and worked
  examples.
