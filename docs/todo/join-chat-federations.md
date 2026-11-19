# join-chat-federations

Let whoever joins a chat (a superadmin, or an admin granted
`join_chat`) put the joined group into its federations.

## Why

Joining acts on the shared account, and a joined group should land in
the right federation (docs/design/web-accounts.md section 3).

## Scope

- On the confirmation step, the admin's federations as toggle chips
  (none selected = the group stays in Default); after a successful
  join, the group is added to the chosen federations, audited.
- `web/docs/join-chat.md` updated.

## Design notes

- The group's chat id is known from the preview; add it only after the
  join succeeds (or the request is sent, if approval is needed: then
  add it once the account becomes a member).

## Depends on

Nothing. (The per-admin `join_chat` grant that gates Join chat is
built: web/docs/accounts.md.)

## Done when

- An admin with `join_chat` joins a group and ticks two of its
  federations, and the group appears in both.
