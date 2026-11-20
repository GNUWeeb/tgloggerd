# Joining a chat (`/join-chat`)

An admin can make the logging account join a group or channel from the
dashboard, by invite link or public username, instead of doing it by hand in
a Telegram client logged in as the account. Once joined, the daemon's
chat-membership tracking picks the chat up, and it is logged from then on,
media included.

Only the daemon holds the TDLib session, so the web asks it over the internal
API ([../../docs/internal-api.md](../../docs/internal-api.md),
`POST /v1/chats/resolve` and `POST /v1/chats/join`). The daemon parses the
input itself and accepts only invite links and usernames; the web passes the
admin's text through, trimmed and capped at 256 bytes.

## Endpoints

Every route is `AuthFilter` + `AdminFilter`: joining acts on the Telegram
account, visibly to the chat's members and against Telegram's rate limits.

| Method | Path | Does |
|---|---|---|
| GET | `/join-chat` | The page: the form, and the last 20 join attempts from `web_audit`. |
| POST | `/join-chat/preview` | Resolves `ref`; read-only on the account. |
| POST | `/join-chat/join` | Joins `ref`, and writes one `web_audit` row. |

Both POSTs take a form body (`application/x-www-form-urlencoded`) with `csrf`
(the session's CSRF token, `data-csrf` on the page) and `ref`. The preview is
a POST behind CSRF as well, because each one costs a rate-limited Telegram
lookup that a cross-site page must not be able to trigger.

`ref` is an invite link (`https://t.me/+...`, `https://t.me/joinchat/...`) or
a public username (`@name`, `name`, `https://t.me/name`, `t.me/name`); the
exact grammar is in the internal API document.

### POST /join-chat/preview

    200 {"ok": true, "outcome": "ok", "message": "", "ref": "@gnuweeb",
         "kind": "username", "chat": {...}}

- `outcome`, `ref` (normalised), `kind` and `chat` are the daemon's
  (`chat` has `chat_id`, `title`, `type`, `member_count`, `is_member`,
  `banned`, `needs_approval`, `paid`, `username`).
- `message` is the web's sentence for the outcome; empty for `ok`.
- A preview that found nothing (`not_found`, `link_invalid`, ...) is still
  `ok: true`, with `chat` null and the reason in `message`.

### POST /join-chat/join

    200 {"ok": true, "outcome": "joined",
         "message": "Joined. The chat is now logged, media included.",
         "ref": "@gnuweeb", "chat": {...}}

`outcome` is one of the daemon's: `joined`, `already_member`, `request_sent`,
`approval_required`, `declined`, `not_a_group`, `not_found`, `link_invalid`,
`flood_wait` (with `retry_after`, seconds), `banned`, `too_many_chats`,
`error` (Telegram's message in `message`). The page shows `message`.

### Failures

When the daemon gives no outcome the answer is `{"ok": false, "error":
"<sentence>"}`:

| Status | When |
|---|---|
| 400 | Empty or over-long `ref`, or the daemon refused it as neither an invite link nor a username. |
| 403 | Bad or missing CSRF token, or not an admin. |
| 502 | The daemon API is off (`WEB_TGLOGGERD_API` unset), unreachable, refused the web's key (`WEB_APP_KEY` differs), TDLib is not ready (daemon 503), or Telegram did not answer in time (daemon 504). A join that timed out may still have happened; the message says to preview again. |

## Audit

Each `POST /join-chat/join` that passes CSRF writes one `web_audit` row,
whatever came of it, with `action = 'chat_join'`, the admin's `web_user_id`,
the client IP, and a `detail` of the outcome and the normalised ref, plus the
chat title and the flood wait when known; the Go service also ends it with
the chat's id when the daemon gave one:

    joined @gnuweeb "GNU/Weeb" chat=-1001000000001
    flood_wait https://t.me/+AbCdEfGhIjKl "Some Group" retry_after=300
    no_answer @gnuweeb

When the daemon gave no outcome, the first word is `api_off` (not
configured), `no_answer` (refused connection or timeout) or `http_<status>`,
followed by the ref as the admin typed it. `detail` is cut to 255 characters.
Previews are not audited: they change nothing.

## The JSON API (Go service)

The Go service serves the same two actions as JSON operations
(`webapi/openapi.yaml`, tag `join-chat`), with the session's CSRF
token in `X-CSRF-Token`:

    POST /api/join-chat/preview   {"ref": "..."}
    POST /api/join-chat/join      {"ref": "...", "federation_ids": [0]}

Both answer `{outcome, message, chat, ref}` (and `retry_after` with
`flood_wait`) on a daemon outcome, `chat_id` 0 where the daemon has
null (the account cannot see the chat before joining). Failures are
400 `bad_request` (empty, over 256 bytes, or refused by the daemon)
and 502 `daemon`
with the sentences above. They are for superadmins and for admins
granted `join_chat` by a superadmin (`restrict.join_chat`,
web/docs/accounts.md); to anyone else they do
not exist (404). `federation_ids` must be existing federations the
viewer administers (404 otherwise, before anything is joined); the
page offers those. Each join writes the same `chat_join` row as above, so
the page's list (`/join-chat`'s data, the last 20 attempts) reads the
rows of both services. A superadmin's list holds every account's
attempts; an admin's only its own, leaving out the attempts at chats
hidden from it (`restrict.hide_groups`), which the `chat=` id tells,
and giving no title where a row does not name its chat.

## Filing the joined group into federations

The join's `federation_ids` (Default, 0, counts as none) say where the
group goes (docs/design/web-accounts.md section 3), and the join's answer
carries `federation_ids` (where it went, or waits to go) and
`filing_pending`:

- `joined`: the group is added to each chosen federation at once, as the
  federation editor adds one (the members' Default grants of it go), with
  one `federation_update` audit row per federation
  (`federation 9; groups +1 -0; joined -1001234567890`).
- `request_sent`, `approval_required`: the account is not a member yet, so
  the choice is held (`web_join_filings`, web migration 000017; audited as
  `chat_join_hold <ref> federations 9,10 chat=<id>`) and `filing_pending`
  is true. A held filing is settled once the account is a member:
  - a background sweep, once a minute, reads the held filings whose chat id
    is known (one query, and nothing more when there are none) and files
    those whose group's message counter (`telegram_groups.msg_count`) has
    grown past its value at the request: the account only receives a
    group's messages as a member;
  - a later preview that finds the account in the chat (`is_member`), or a
    join answering `joined` or `already_member`, settles the filings held
    for that ref or chat id. This is the only way for a private group
    behind an invite link, whose id the account cannot see before joining.

  Settling files the group as above (`approved` in place of `joined` in
  the audit row), checking again that the account that asked still
  administers the federation; if not, the filing is dropped unfiled
  (`chat_join_unheld ... no longer the account's`).
- `already_member` and every other outcome: nothing is filed. Moving a
  group the logger already has between federations is the federation
  editor's.

The page data's `pending` lists the held filings (an admin's own, all of
them for a superadmin); `DELETE /api/join-chat/pending/{id}` drops one
(204; 404 when it is not held or not the viewer's; audited as
`chat_join_unheld <ref> federation <id> cancelled`). A filing also goes
when its federation or its account is deleted.

## Configuration

`WEB_TGLOGGERD_API` (`http://host:port`, in the compose deployment
`http://10.0.88.5:8091`) points the web at the daemon, which must run with
`TG_API_LISTEN` set. The bearer key is derived from `WEB_APP_KEY`, which the
two already share. Without `WEB_TGLOGGERD_API` the page says the daemon API is
not configured and every join fails with 502.
