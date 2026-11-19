# group-members

Show a group's real member count and its member list: the count
fetched from Telegram on demand and cached in the database, the list
fetched on demand in a Members tab of the group page.

## Why

The chat and the group page show only how many members the archive
has seen (senders), not how many the group has, and there is no way
to see who is in a group.

## Scope

- The daemon stores each group's member count (and whether its
  members may be listed) whenever Telegram sends it, refreshes it on
  request, and lists members page by page (supergroups, basic groups,
  and channels where the logging account is an admin).
- The Go service shows the cached count on the chat head, the group
  page and lists, refreshes it in the background when it is older
  than ten minutes (the cache answers when Telegram fails), and
  serves the member list to superadmins and admins, its first page
  cached briefly so a prefetch from the chat makes the tab instant.
- A superadmin can hide a channel's member list.
- The React application shows the count, a Members tab with the list
  (or the count alone, with the reason, where no list is available),
  the superadmin's toggle, and prefetches the first page when a chat
  opens.

## Design notes

The contract between the three parts: daemon migration 000044
(telegram_groups member_count, member_count_at, can_get_members),
POST /v1/chats/{chat_id}/refresh and GET /v1/chats/{chat_id}/members
on the daemon (docs/internal-api.md), web migration 000013
(web_hidden_member_lists), GET /api/groups/{id}/members and the page
route /groups/{id}/members (group_members), GroupProps
hide_member_list and can_hide_member_list.

Members (the account role) and anonymous viewers see the count only;
a member list names users a member may not otherwise see.

## Depends on

Nothing.

## Done when

- The chat head and group page show "N members" beside the members
  seen; the count survives Telegram failing.
- The Members tab lists a group's members for an admin, says why when
  it cannot, and is absent for members and anonymous viewers.
- A superadmin's hide toggle hides a channel's list from admins.
