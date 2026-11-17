# web-join-chat

Let an admin make the logging account join a group or channel from the
web dashboard, given an invite link or a public username.

## Why

Joining a chat is today done by hand in a Telegram client logged in as
the account. The dashboard should let admins control it, so that a chat
can be added to the log without access to that client.

## Scope

- An admin-only page or form on the web: paste an invite link
  (`https://t.me/+...`, `https://t.me/joinchat/...`) or a public
  username (`@name`, `https://t.me/name`), preview the chat (title,
  type, member count, whether the account is already in it), then
  confirm the join.
- New operations on the internal API of tgloggerd, the only holder of
  the TDLib session: resolve the link or username
  (`checkChatInviteLink`, `searchPublicChat`), and join
  (`joinChatByInviteLink`, `joinChat`). Each is a curated operation,
  never a TDLib passthrough.
- Results the admin understands: joined, already a member, join
  request sent (approval needed), link expired or invalid, flood wait
  (with the wait), banned.
- An audit row per join attempt, as for other admin actions.
- `docs/internal-api.md` and `web/docs/` updated with the new
  endpoints.
- Not in scope: leaving chats; joining several at once.

## Design notes

- Builds on the internal API (docs/internal-api.md) and
  `TDLib::request`, both already merged. Joining changes the account's
  membership, which chat-membership-tracking then picks up, and with
  it media downloads for that chat (media-record-without-download).
- Joining is an action with consequences on the account (rate limits,
  a visible join in the chat); require a confirmation step and keep it
  admin-only, behind CSRF like the other admin forms.
- Asked for by the project owner on 2026-11-18.

## Depends on

Nothing.

## Done when

- An admin can join a public group by username and a private one by
  invite link from the dashboard, sees each outcome above reported
  clearly, and the joined chat starts being logged with its media.
