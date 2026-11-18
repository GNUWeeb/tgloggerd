# Platform forwarding (`/platform-fwd`)

Cross-platform message forwarding is configured under one section, with a page
per **direction**:

| Path | Direction | Daemon | Transport |
|---|---|---|---|
| `/platform-fwd` | index | — | — |
| `/platform-fwd/telegram-discord` | Telegram → Discord | `tgloggerd` | Discord incoming webhook |
| `/platform-fwd/discord-telegram` | Discord → Telegram | `discordd` | Telegram bot (TDLib) |

Direction pages are named `<source>-<destination>`, matching the table naming
convention in [db-naming.md](db-naming.md) — `telegram_discord_webhooks` pairs
with `/platform-fwd/telegram-discord`, and so on. Adding a platform means adding
a direction page and a card on the index, not another top-level nav entry.

This page covers **configuring** routes. How the daemon **renders** a forwarded
message on the far side is separate: the Telegram → Discord author-name format,
which stamps each forward with a reversible sender/message identifier, is
documented in
[telegram-discord-forwarding.md](../../docs/telegram-discord-forwarding.md).

Every endpoint requires an authenticated **admin** — `AuthFilter` then
`AdminFilter` — because these rows hold credentials.

Both directions may run at once on the same pair of chats. When they do, each
daemon declines to re-send what the other delivered, so a message is not echoed
back to where it came from: `discordd` skips webhook-authored Discord messages,
and `tgloggerd` skips Telegram messages sent by a bot in `telegram_bots`.

## The JSON API (Go service)

The Go service (`webapi/`) serves the same configuration as JSON
operations, described in `webapi/openapi.yaml` (tag `forwarding`).
Bodies are JSON, every write carries the session's CSRF token in
`X-CSRF-Token`, and errors are `{"error": "<code>", "message": "..."}`.

    POST   /api/forwarding/telegram-discord        create; 201 {id}
    PATCH  /api/forwarding/telegram-discord/{id}   types, enabled, webhook_url
    DELETE /api/forwarding/telegram-discord/{id}
    POST   /api/forwarding/telegram-discord/test   {id} or {webhook_url}; 204
    POST   /api/forwarding/discord-telegram        create; 201 {id}
    PATCH  /api/forwarding/discord-telegram/{id}   types, enabled, bot_token
    DELETE /api/forwarding/discord-telegram/{id}

The pages' data (`/platform-fwd`, `/platform-fwd/telegram-discord`,
`/platform-fwd/discord-telegram`) lists the configurations with their
counts of what was sent.

- Who: viewers with the `forward` ability of the request scope
  (superadmins and admins today; members once granted it, with
  `forwarding-per-user`). To anyone else the operations and pages do
  not exist (404). A configuration whose Telegram chat the viewer may
  not see, a group hidden from it or a private chat when private
  messages are, is 404 as well, in the lists too.
- Types are lists of the keys above (`["text", "photo"]`); create
  needs them all, PATCH changes only the fields it is given. The
  chat (and the Discord channel) of a configuration cannot change.
- The webhook URL is checked as `parseWebhook` does here, on create,
  on change and on test, also when a saved webhook is tested: anything
  but `https` on one of Discord's hosts with an `/api/webhooks/<id>/<token>`
  path is 400 `bad_webhook`, and nothing is posted. The test message
  answers 204, or 502 `send_failed` with Discord's status.
- Secrets are write-only in both directions: lists show the webhook URL
  with its token cut off (`url_masked`) and a bot by its username, and
  no answer, log line or audit row carries a URL or a token.
- Discord to Telegram refuses a channel id that is not a snowflake
  (`bad_channel`), a token not shaped `<digits>:<secret>`
  (`bad_token`), a duplicate channel and chat (409 `duplicate`) and
  deleting a route that has sent messages (409 `in_use`).
- Every change writes a `web_audit` row: `fwd_t2d_create`,
  `fwd_t2d_update`, `fwd_t2d_delete`, `fwd_d2t_create`,
  `fwd_d2t_update`, `fwd_d2t_delete`, with the id, the chat and what
  changed. Tests change nothing and are not audited.
- Ownership is not stored yet (`web-ownership`): every configuration
  shows as shared, in the Default federation, and the transfer
  operations are planned. The Go service writes through the same
  read-only account and grants as the Drogon pages (see *Grants*).

## Content-type filters

Each route, in either direction, chooses which **content types** it forwards.
Both pages show them as a row of checkboxes in the add/edit form, and the route
table summarises them in its *Forwards* column (`Everything`, a list, or
`Nothing`). Only the ticked types are forwarded.

| Type (`fwd_<key>`) | Telegram → Discord | Discord → Telegram |
|---|---|---|
| `fwd_sticker`  | a sticker | a sticker (`sticker_items`) |
| `fwd_text`     | a text message, or the caption of a media message | the message text |
| `fwd_photo`    | a photo | an attachment whose MIME type is `image/*` |
| `fwd_video`    | a video | an attachment whose MIME type is `video/*` |
| `fwd_document` | a document (file) | every other attachment, including one with no MIME type |
| `fwd_other`    | service (system) messages and every type not listed: voice, audio, animation (GIF), … | — (no such class) |

A message is filtered **part by part**, not as a whole:

- A photo with a caption and *Photo* off, *Text message* on → the caption alone
  is forwarded. With *Text message* off and *Photo* on → the photo alone.
- A Discord message with text, an image and a PDF becomes one Telegram message
  holding only the lines whose type is ticked (Discord media crosses as one link
  line per item, and stickers as a link line too).
- When no part of a message is enabled, nothing is sent for it.

Telegram → Discord files what a **service message** says ("X joined the group")
under *Others*, not *Text message*: it is part of that message, not a caption.

The flags are the `fwd_*` columns of `telegram_discord_webhooks` and
`discord_telegram_routes` (migration `000031`), all defaulting to `1`, so a
route forwards everything until told otherwise. The daemons pick a change up on
their next reload of the table, like any other edit (see *Propagation*).

Changing a filter affects **new** messages only. An edit or delete of a message
forwarded earlier still reaches its forwarded copy, and a message that was
filtered out is not sent retroactively when its type is ticked later.

### Parameters

Both save endpoints — `POST /platform-fwd/telegram-discord/save` and
`POST /platform-fwd/discord-telegram/save` — accept one parameter per type,
named as in the table above (`fwd_other` on Telegram → Discord only):

| Value | Meaning |
|---|---|
| `1` | Forward this type |
| `0` (or anything else non-empty) | Do not forward it |
| omitted | **Create:** forward it. **Update:** leave the stored flag unchanged |

The pages always send every type. The "omitted" rule exists so a script written
before the filters existed keeps working without switching anything off.

Saving with an `id` that no longer exists is refused with a 404
(`That integration no longer exists. Please reload.` or `That route no longer
exists. Please reload.`) rather than silently doing nothing.

The rest of this document covers **Discord → Telegram**, which is the newer and
more involved of the two.

## Secrets: how the bot token is handled

`telegram_bots.token` is a Telegram bot token. Whoever holds it can read and
send as that bot, in every chat the bot is in. It is therefore treated as
write-only:

- **It is never SELECTed for display.** `dao::routes::listBots()` and
  `dao::routes::list()` do not name the `token` column at all.
- **It never appears in a response, page, or DOM attribute.** A bot is
  identified by `bot_label`, built from its Telegram **user id** and `@username`
  — the id half of a token (`<bot_user_id>:<secret>`) is not secret, and we
  already store it separately once `discordd` logs the bot in.
- **Editing a route never round-trips the token.** The form's token field is
  always blank; leaving it blank keeps the currently selected bot. Supplying a
  token means "use this bot instead", and interns it.
- **Tokens are interned, not per-route.** `telegram_bots.token` is `UNIQUE`, so
  re-adding the same bot reuses its row and several routes share one credential.

This deliberately departs from the Telegram → Discord page, which returns `webhook_url` in
full and prints it into the page. A webhook URL is scoped to one channel and is
further constrained there by a host allowlist; a bot token is a whole account,
so the same treatment would not be acceptable.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| GET    | `/platform-fwd/discord-telegram`        | SSR admin page |
| POST   | `/platform-fwd/discord-telegram/save`   | Create or update a route |
| POST   | `/platform-fwd/discord-telegram/delete` | Delete a route |
| GET    | `/platform-fwd/discord-telegram/chats`  | select2 Telegram chat search |

Request bodies are **`application/x-www-form-urlencoded`**, not JSON — matching
the Telegram → Discord page. Every mutating POST carries `csrf`, whose value the page
exposes as `data-csrf` on the page root.

### `POST /platform-fwd/discord-telegram/save`

| Param | Required | Meaning |
|-------|----------|---------|
| `id` | no | Route id; empty creates a new route |
| `discord_channel_id` | yes | Discord channel snowflake, digits only |
| `chat_id` | yes | Telegram chat id (negative for a group/channel) |
| `telegram_bot_id` | conditionally | Existing `telegram_bots.id` |
| `bot_token` | conditionally | New bot token; wins over `telegram_bot_id` |
| `enabled` | no | `1` to enable |
| `fwd_sticker`, `fwd_text`, `fwd_photo`, `fwd_video`, `fwd_document` | no | Content types to forward — see *Content-type filters* |
| `csrf` | yes | Session CSRF token |

One of `telegram_bot_id` or `bot_token` must be supplied.

### `POST /platform-fwd/discord-telegram/delete`

`id` and `csrf`. Refused with an explanation when the route has already
forwarded messages — see *Deleting a used route* below.

### `GET /platform-fwd/discord-telegram/chats`

`q` (search term) and optional `limit` (1–50, default 20). Returns select2's
shape, reusing `dao::discord::searchChats` so both pages resolve chats
identically:

```json
{ "results": [ { "id": -1001347566306, "text": "…", "type": "group", "title": "…" } ] }
```

## Responses

Success is `{"ok":true}`. Failure is `{"ok":false,"error":"…"}` with a 4xx/5xx
status. The client shows `error` verbatim, so the strings are written to be read
by an operator.

Unlike the Telegram → Discord page, the handlers here wrap their database calls in
`try`/`catch` and return the error envelope on failure. Without that, a missing
`GRANT` (see below) surfaces as an HTML 500 into a caller that is parsing JSON.

### Validation, and why each check exists

| Error | Cause |
|---|---|
| `Enter the Discord channel ID …` | Non-numeric or empty channel id. Pasted by hand, so a typo must be named rather than silently becoming `0`. |
| `That chat is not in the log yet …` | The chat is not in `telegram_groups`/`telegram_users`, so the logger has never seen it. |
| `That Discord channel already forwards to that Telegram chat.` | Would violate `uq_dtr_channel_chat`. Checked first so the duplicate reads as a sentence. |
| `That does not look like a Telegram bot token …` | Token is not `<digits>:<secret>`. Shape only — whether it *works* is discovered when `discordd` logs it in. |
| `Choose an existing bot, or paste a bot token to add one.` | Neither `telegram_bot_id` nor `bot_token` given. |
| `That route no longer exists. Please reload.` | Saving with an `id` that was deleted meanwhile (404). |
| `This route has forwarded N message(s) …` | See below. |
| `Your session expired. Please reload.` | CSRF check failed (403). |

## Deleting a used route

`discord_telegram_sent_messages.route_id` references
`discord_telegram_routes(id)` under the default **RESTRICT**, so deleting a
route that has ever forwarded a message fails at the database. Rather than
letting a foreign-key error escape, the delete endpoint counts the referencing rows
first and refuses with a count and the alternative: **disable** the route.

That is the right default. Those rows are what map a Discord message to the
Telegram message it became; cascading the delete would destroy the history that
makes replies and edits resolve.

## Grants

The web app reaches the logger's schema through the read-only `web_ro` user, so
this page needs DML on exactly two more tables:

```sql
GRANT INSERT, UPDATE, DELETE ON `tgloggerd`.discord_telegram_routes TO 'web_ro'@'%';
GRANT INSERT, UPDATE, DELETE ON `tgloggerd`.telegram_bots TO 'web_ro'@'%';
FLUSH PRIVILEGES;
```

`docker/mysql/init/10-web-users.sh` issues these, but that hook only runs on a
**fresh** MySQL data directory. On an existing deployment they must be applied
once as root; until then the page lists routes but every save and delete fails.

## Propagation

`discordd` re-reads the route table every `DISCORDD_ROUTE_REFRESH_SECS`
(default 30, floor 5), so changes take effect without a restart. A route is live
only when both `discord_telegram_routes.enabled` and `telegram_bots.enabled` are
set, which is why the page distinguishes *Disabled* from *Bot disabled*.

One ordering caveat: a **newly added bot** has `bot_user_id = 0` until `discordd`
logs it in, and the Telegram → Discord forwarder's loop guard matches on that id.
So in the first seconds after adding a route through this page, a message it
forwards may still be echoed back into Discord once. It settles on the next
refresh.

## Why no Discord channel picker

The Telegram side has a select2 search because the logger has a populated list
of chats. There is no equivalent for Discord: `discordd` only writes
`discord_channels` for channels that **already** have a route, so a picker would
be empty exactly when it is needed. The channel is therefore entered as an id
(Discord → Developer Mode → Copy Channel ID), and shown as an id in the table.
