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
chat title and the flood wait when known:

    joined @gnuweeb "GNU/Weeb"
    flood_wait https://t.me/+AbCdEfGhIjKl "Some Group" retry_after=300
    no_answer @gnuweeb

When the daemon gave no outcome, the first word is `api_off` (not
configured), `no_answer` (refused connection or timeout) or `http_<status>`,
followed by the ref as the admin typed it. `detail` is cut to 255 characters.
Previews are not audited: they change nothing.

## Configuration

`WEB_TGLOGGERD_API` (`http://host:port`, in the compose deployment
`http://10.0.88.5:8091`) points the web at the daemon, which must run with
`TG_API_LISTEN` set. The bearer key is derived from `WEB_APP_KEY`, which the
two already share. Without `WEB_TGLOGGERD_API` the page says the daemon API is
not configured and every join fails with 502.
