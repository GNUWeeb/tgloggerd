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
- [email.md](email.md) — account email in the Go service, optional per
  account and per service: verified addresses, set-password and
  verification links, the forgotten-password form, the security notices,
  and the SMTP relay's settings.
- [contact-names.md](contact-names.md) — a contact's own name versus the
  name the logging account saved them under: what each column means, what
  the pages show, and the admin action "Look up real name" with its routes.
- [db-naming.md](db-naming.md) — database table naming convention: the
  per-platform prefixes (`telegram_`, `discord_`), what is and isn't prefixed,
  and the rule to follow when adding tables for a new messaging platform.
- [group-tabs.md](group-tabs.md) — the group page's Files and Polls tabs:
  who sees them, what they list and in which order, paging by message id,
  the kinds and their counts, and how a page avoids a query per row.
- [join-chat.md](join-chat.md) — the admin `/join-chat` page: preview and
  join a group or channel by invite link or username through the daemon's
  internal API, the outcomes it reports, and the `chat_join` audit rows.
- [message-kinds.md](message-kinds.md) — how the Go service's chat shows
  each kind of message (locations, polls, calls, service messages, ...):
  the detail per kind, the people and chats named and what each viewer may
  see of them, service texts rebuilt for the viewer, forum topics, pins and
  reactions.
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
- [settings.md](settings.md) — an account's own settings in the Go service:
  the preferences (timezone, date format, language, theme, list and chat
  view defaults) with their site defaults, the language lookup of the texts,
  pinned groups, saved searches, and the sessions page with sign-out.
