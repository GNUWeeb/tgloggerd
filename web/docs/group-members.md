# Group members

The Go service shows a group's real member count, as Telegram gives it,
and lists its members on the group page's Members tab. Only the daemon
holds the Telegram session, so both come through its internal API
(docs/internal-api.md: `POST /v1/chats/{chat_id}/refresh` and
`GET /v1/chats/{chat_id}/members`); the shapes of the web's answers are
in webapi/openapi.yaml.

## The member count

The daemon stores Telegram's count in `telegram_groups.member_count`,
with `member_count_at` (when it was stored) and `can_get_members`,
whenever Telegram sends a group's full info or an update. Every group
summary the service sends carries `member_count` and `member_count_at`
(null before a count is stored): the chat page's head, the group page
and its tabs, an exposed group's public page, and the groups list,
which also filters and sorts by `member_count`. Every viewer of a group
gets the count, members and signed-out visitors of exposed groups
included: it is what Telegram shows anyone who opens the group.
`members` beside it is still the people the archive has seen (senders
and admins).

A chat page or a group page whose stored count is missing or older than
10 minutes asks the daemon to refresh it, in the background:

- never in the page's path: the page shows the stored count;
- at most one call in flight per group, and a group just asked about is
  left alone for a minute (the daemon asks Telegram at most once a
  minute per chat anyway);
- the daemon stores the new count, which the next page load shows; a
  failure keeps the stored count and is only logged.

## The member list

`GET /api/groups/{id}/members?offset=&limit=` (offset 0 to 1000000,
limit 1 to 200, default 50) answers a page of the members, in the order
Telegram lists them ("recent"):

    {"available": true, "reason": "", "total": 1234,
     "offset": 0, "limit": 50,
     "members": [{"user": {UserSummary}, "status": "administrator",
                  "joined_at": "2023-11-14T22:13:20Z"}]}

- Who: viewers who see the group and see users in full (superadmins and
  admins). Members, signed-out visitors and anyone the group is hidden
  from get 404; they see the count only.
- Names come from `telegram_users`, in one query per page, where the
  daemon stores the users it lists; a member not stored yet is named by
  its id (`#<id>`, type `unknown`).
- `status` is `creator`, `administrator`, `member` or `restricted`.
- Each page is kept in memory for 2 minutes per group, search, offset
  and limit, so the chat page's background prefetch makes the Members
  tab instant; a `flood_wait` or `error` answer is not kept, so the next
  request asks again.

When there is no list, `available` is false, `members` is empty,
`total` is the stored count (or null), and `reason` says why:

| reason | meaning |
|---|---|
| `not_allowed` | A channel where the logging account is not an administrator, or a group that hides its members. |
| `hidden` | A superadmin hid the channel's list (below); never for a superadmin. |
| `not_member` | The logging account is not in the group. |
| `flood_wait` | Telegram rate-limited the account; `retry_after` is the wait in seconds. |
| `unavailable` | The daemon API is off or did not answer. |
| `error` | Telegram failed otherwise. |

A client shows any other reason as "not available".

## Searching the members

`?q=` searches the members, with `offset` and `limit` as for the list
and the same answer: the same viewers, reasons (a hidden list stays
hidden) and cache. The query is trimmed and a leading `@` dropped; at
most 64 characters (longer, or not UTF-8, is 400); empty, or a lone `@`,
is the plain list. The daemon asks Telegram:

- supergroups and channels: Telegram's member search, which matches
  first and last names and the username;
- basic groups: their stored members whose name or username contains
  the query;
- a query of only digits (a leading `-` allowed) also returns that user
  first when a member.

Each member carries `matched_by`: `id` (the query was its user id) or
`search`; the plain list has no `matched_by`. `total` is the number of
matches (the id match included) when the daemon knows it, else null,
never the group's member count. The id match comes on the first page
(offset 0) in addition to `limit`, and later pages leave it out, so a
page holds up to `limit` + 1 members: the next page starts at `offset`
plus the number of `search` members.

The page `/groups/{id}/members` (route `group_members`) carries the
group's page as its other tabs do, plus the same answer for its
`?offset=` (and `?limit=` and `?q=`; out of range is the first page, a
bad `q` no search); the same viewers get it, the others not_found (a
signed-out visitor is asked to sign in).

## Hiding a channel's list

A superadmin can hide a channel's member list from everyone but the
superadmins: `PATCH /api/groups/{id}` with `{"hide_member_list": true}`
(false shows it again), kept in the web's `web_hidden_member_lists`
(web migration 000013). Only channels, since a group's members see each
other in Telegram: a supergroup or basic group is 400. An admin gets
403, a viewer who does not see the channel 404. Each change is audited
as `member_list_hide` or `member_list_show`. The group page's props
carry `hide_member_list` and `can_hide_member_list` (true for a
superadmin on a channel).
