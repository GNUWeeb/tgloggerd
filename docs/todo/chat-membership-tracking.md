# chat-membership-tracking

Know, per chat, whether the account is a member, from the supergroup
and basic group status updates TDLib already sends.

## Why

The logger drops `supergroup.status` and does not handle
`updateBasicGroup`, so it cannot tell a joined group from a channel it
only heard of through a forward or a reply. Not downloading media from
non-member chats (media-record-without-download) needs that answer.

## Scope

- Cache the member flag from `updateSupergroup` and a new
  `updateBasicGroup` handler, keyed by chat id, with the rules of
  storage-cache.md section 5.
- `bool is_member_chat(int64_t chat_id)` on the loop thread.
- A debug log line when a chat's flag changes.
- Not in scope: acting on the flag (media-record-without-download);
  persisting it in the database.

## Design notes

- Supergroup and channel chat ids are `-100<supergroup_id>`
  (`-1000000000000 - id`), basic group chat ids are `-<id>`; check how
  TDLib.cpp already maps them (`chat_to_group_`, `group_state_`) and
  reuse that rather than computing ids twice.
- Unknown counts as member, so that a chat whose status has not
  arrived yet keeps today's behaviour.

## Depends on

Nothing.

## Done when

- tgloggerd builds.
- At cutover, the debug log shows member and non-member chats, and a
  spot check of a few against the Telegram client agrees.
