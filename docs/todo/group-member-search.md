# group-member-search

Search a group's members by name, username or user id on its Members
tab.

## Why

The member list comes from Telegram a page at a time, so filtering
the loaded pages in the browser misses most members of a big group.

## Scope

- The daemon searches a group's members on Telegram: supergroups and
  channels through the member search filter (names and username),
  basic groups by filtering their stored members; a query of digits
  (optionally with a leading -) also returns that user first when a
  member.
- The web service passes the query through (?q= on the group_members
  page and GET /api/groups/{id}/members, 1 to 64 characters, a leading
  @ ignored), caches results like the list, same scope and reasons.
- The React application's search box above the list (debounced, in
  the URL as ?q=), paging within results, "No member matches".

## Depends on

Nothing.

## Done when

- A superadmin or admin finds a member of a big group by part of the
  name, by username and by id.
