# Web accounts and roles

The web has several accounts, each with one role
([../../docs/design/web-accounts.md](../../docs/design/web-accounts.md)
section 2). This page describes what is built: the roles, the request
scope they produce in the Go service (`webapi/`), and how the Drogon
pages treat them while they still serve.

## Roles

| Role | Sees | May do | Manages |
|---|---|---|---|
| superadmin | everything | everything | every account, any role |
| admin | every group, private chat and user | all but Join chat and "Look up real name" | members only |
| member | no group, private chat or user yet | nothing beyond seeing | nobody |

- An admin's restrictions (hidden groups, hidden private messages,
  the `join_chat` permission) come with `web-admin-restrictions`;
  until then an admin sees everything and may not join chats (the
  permission is off by default).
- A member sees only the groups granted to it, and users only as they
  appear there. Grants come with `web-member-access`; until then a
  member sees no group, and every group, user, private chat and file
  listing answers it as if empty, every id 404.
- Federations come with `web-federations`. Until then the Default
  federation (id 0 in the API) is the only one: every member is in it
  and every admin administers it, so an admin manages every member.

Web migration 000004 turned today's `admin` accounts into superadmins
and `viewer` accounts into members.

## The request scope

The Go service builds one scope per request
(`Server.requestScope`, package `webapi/internal/scope`) from the
account as its row reads on that request: id, role, and the forced
password change. Every query over the logger's schema and every
permission check takes it. Something outside it answers 404, exactly
like something that does not exist; a forbidden action on something
the request may see answers 403. The menu (`Viewer.nav` of the page
data) is the scope's list of page keys.

A role change, a disable or a sign-out-everywhere applies to the next
request: the role is read from the account, not from the cookie, and
disabling an account or changing its role also bumps its
session_epoch, which ends its sessions on both services.

## The Drogon pages

The Drogon pages know only "admin or not":

- superadmins and admins get today's admin pages;
- members are refused every page (403), and a member's session counts
  as none on the public chat pages, because those pages cannot apply
  grants and would show a member the whole archive;
- `--seed-admin` creates or resets a superadmin.
