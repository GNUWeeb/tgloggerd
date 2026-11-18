# web-hidden-group-totals

Leave an admin's hidden groups out of the per-user message totals it
sees, as everything else about those groups already is.

## Why

Hiding groups from an admin (web-admin-restrictions) removes them from
every list, search, page and reference, but a user's message total is
one stored number per user (telegram_users.msg_count, counted by the
daemon over all groups). An admin with hidden groups still sees totals
that include the messages in them: in the users list's Messages
column, its sort and filters, the user page and the user search.
Low priority (the project owner, 2026-11-18).

## Scope

- For a scope that hides groups, show each user's total over the groups
  it sees: the stored total minus the user's messages in the hidden
  groups, or a count over the visible ones, whichever the indexes
  serve (telegram_group_messages has (sender_user_id, chat_id)).
- The same total wherever one is shown or used: the users list column,
  its sort and msg_count filters, the user page, the people searches,
  the chat search's user rows.
- Viewers without hidden groups keep reading the stored total, so their
  pages cost nothing more.
- Tests: an admin hiding a group sees each user's total without that
  group's messages, in the list, a sort, a filter and the user page; a
  superadmin sees the stored totals.

## Depends on

- web-admin-restrictions (hidden groups).

## Done when

- No total, sort or filter an admin sees counts messages in a group
  hidden from it.
