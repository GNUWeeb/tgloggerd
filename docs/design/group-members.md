# Group members: counts and lists (daemon side)

Status: built 2026-11-19 on the daemon side; the web (Go) and the React
app build against the same contract (the main session's brief,
"group-members"). This note covers what the daemon stores and answers.

## 1. Goal

Show a group's real member count, fetched from Telegram on demand and
cached in the database (the cache answers when Telegram cannot), and
list a group's members on demand. Channels list their members only
where the logging account is an administrator; a superadmin can hide a
channel's list in the web.

## 2. What is stored

`telegram_groups` gains three columns (migration 000044):

- `member_count INT UNSIGNED NULL`: as Telegram last reported it.
- `member_count_at DATETIME NULL`: when `member_count` was last written.
- `can_get_members TINYINT(1) NULL`: whether the account may list the
  members.

They are written, without touching `updated_at` or the group's history:

- from `updateSupergroup` (its `member_count`, when not 0);
- from `updateSupergroupFullInfo` (`member_count`, `can_get_members`);
- from `updateBasicGroup` (`member_count`) and
  `updateBasicGroupFullInfo` (the size of its member list; the list is
  visible to every member, so `can_get_members` is whether it is
  non-empty);
- by the two endpoints below.

Repeats of the values last written for a chat are not written again, so
a busy group's updates cost nothing. The group admin poller
(`fetch_group_admins`) is unchanged.

`telegram_groups` has a FULLTEXT index on `description`, so 000044
drops it, adds the columns in place and builds it again, as 000034 did
for the message tables (decision
[2026-11-18-msg-message-table-ddl](decisions/2026-11-18-msg-message-table-ddl.md)).
On a synthetic table of the archive's size (58 thousand groups) it took
4 seconds.

## 3. Endpoints

`POST /v1/chats/{chat_id}/refresh` and `GET /v1/chats/{chat_id}/members`
(docs/internal-api.md). The refresh asks Telegram at most once a minute
per chat and otherwise answers the stored row. The member list pages
`getSupergroupMembers` (recent filter) or a basic group's full info,
caches each page for 2 minutes, and leaves the members' users to the
daemon's usual user handling: TDLib sends `updateUser` for every user
before an answer mentions it, and the daemon stores it with its photo.

## 4. Left to the web

Who may see a list (superadmins and admins), the superadmin's toggle
that hides a channel's list (`hidden`), when to refresh a count (missing
or older than 10 minutes, in the background), and naming members not
yet stored.
