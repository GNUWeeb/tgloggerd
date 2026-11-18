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
| admin | every group, private chat and user | all but Join chat and "Look up real name" | the members of its federations |
| member | no group, private chat or user yet | nothing beyond seeing | nobody |

- An admin's restrictions (hidden groups, hidden private messages,
  the `join_chat` permission) come with `web-admin-restrictions`;
  until then an admin sees everything and may not join chats (the
  permission is off by default).
- A member sees only the groups granted to it, and users only as they
  appear there. Grants come with `web-member-access`; until then a
  member sees no group, and every group, user, private chat and file
  listing answers it as if empty, every id 404.
- Federations limit what an admin manages, never what it sees (see
  Federations below): an admin manages the members filed in a
  federation it administers.

Web migration 000004 turned today's `admin` accounts into superadmins
and `viewer` accounts into members. Web migration 000006 made every
admin an admin of the Default federation and filed every member in
it, so an admin still managed every member until federations were set
up.

## Federations

A federation is a named set of groups with admins and members
([../../docs/design/web-accounts.md](../../docs/design/web-accounts.md)
section 3), stored in the web's own schema: `web_federations`, its
groups (`web_federation_groups`), admins (`web_federation_admins`) and
members (`web_federation_members`). A group and a member may be in
several.

- The Default federation is the row with id 0. Its groups are
  derived, never stored: every group in no other federation,
  including groups joined later, and a group taken out of its last
  federation is back in Default. Admins and members are assigned to
  it like to any other.
- A superadmin administers every federation without being listed.
  An admin administers those it is listed in, Default included only
  when listed; a new admin administers none until a superadmin says
  so (`admin_of`, or a federation's `admin_ids`).
- An admin manages the members filed in at least one federation it
  administers, and changes their federations only among its own; a
  member's other federations stay as they are. A member in no
  federation is managed by superadmins alone.
- Federations change no visibility: an admin sees every group whatever
  its federations (restrictions come with `web-admin-restrictions`),
  and a member's groups come from its grants (`web-member-access`).
- The scope reads an admin's federations from the account on every
  request, with its role, so a change applies to its next request.

The federations API (webapi/openapi.yaml, tag accounts) is the
superadmins'; to anyone else it answers 404:

    POST   /api/federations       {name, group_ids?, admin_ids?}; 201 {id}
    PATCH  /api/federations/{id}  the same fields, each optional; 200 {id}
    DELETE /api/federations/{id}  204

- `group_ids` are logger groups (400 `bad_group` otherwise) and
  replace the federation's groups; `admin_ids` are accounts with the
  admin role (400 `bad_admin`) and replace its admins. A name is 1 to
  64 characters and unique without regard to case (409 `taken`).
- Default (id 0) takes `admin_ids` only; groups or a new name answer
  400 `bad_request`, and it cannot be deleted.
- Deleting a federation puts its groups back in Default unless
  another federation holds them, and files a member it was the only
  federation of in Default.
- New federations get the highest id so far plus one (the ids are not
  AUTO_INCREMENT, see web migration 000006).
- Audited as `federation_create`, `federation_update` (with what
  changed: name, groups added and removed, admins before and after)
  and `federation_delete`, with the acting superadmin.
- The groups of a federation are listed by the pickers' chat search,
  `GET /api/chats/search?federation=<id>`; for Default those are the
  groups in no other federation.

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

## The accounts API

Served by the Go service and described in webapi/openapi.yaml (tag
accounts); every write carries the session's CSRF token.

    GET   /api/accounts[?role=]                 the accounts the viewer manages
    GET   /api/accounts/{id}                    one of them
    POST  /api/accounts                         create; 201 {id, temporary_password}
    PATCH /api/accounts/{id}                    name, email, role, admin_of, grants
    POST  /api/accounts/{id}/disable            and /enable
    POST  /api/accounts/{id}/password-reset     200 {temporary_password}
    POST  /api/accounts/{id}/sign-out-everywhere

- Who manages whom is the request scope's: a superadmin every
  account and any role, an admin the members of its federations. An
  account the viewer does not manage answers 404, as a missing one
  does; to a member the whole administration is 404. A role the
  viewer may not give, or a federation it does not administer,
  answers 403 `forbidden`; a federation that does not exist, 400
  `bad_federation`.
- A member's `grants` are the federations it is filed in, each
  `{"fed": <id>, "group_ids": []}`; groups cannot be granted inside
  them yet (400 `not_supported`, `web-member-access`). An admin must
  file a new member in a federation it administers: without `grants`,
  400 `grants_required`. A superadmin's member without `grants` is
  filed in Default; `"grants": []` files it in none.
- A PATCH with `grants` replaces the member's federations among those
  the viewer administers (all of them for a superadmin); the others
  stay. A grant of a federation the member is already in passes even
  when the viewer does not administer it, so the page can send the
  member's whole list back.
- `admin_of` sets the federations an admin administers; only a
  superadmin sends it. A role change drops the federations of the old
  role: a member made admin leaves its federations, and an account
  made member is filed in Default unless the same PATCH gives
  `grants`.
- The last active superadmin cannot be disabled or demoted (409
  `last_superadmin`). The check locks the active superadmins' rows,
  so two superadmins disabling each other at once leave one.
- Accounts are disabled, never deleted. Disabling, a role change, a
  password reset and sign-out-everywhere bump the account's
  session_epoch, ending its sessions on both services.
- Every change is written to `web_audit` with the acting account
  (`web_user_id`) and the account changed (`target_user_id`):
  `account_create`, `account_update` (naming the fields, the role
  change as `role old->new`, and federation changes as
  `admin_of 1->1,2` or `federations 0->none`), `account_disable`,
  `account_enable`,
  `account_password_reset`, `account_signout_everywhere`. The
  account's row in the API carries its newest 20.
- The address given at creation is recorded only; it is verified,
  and used, once `web-email` exists.

## The accounts and federations pages

The Go service loads the data of the accounts page
(`/settings/accounts`, AccountsProps) and of the federations page
(`/settings/federations`, FederationsProps), for superadmins and
admins; to a member they do not exist. The accounts are those of
`GET /api/accounts`; the federations part, and which parts of an
account are `editable` yet, come from the federation helpers
(`webapi/internal/server/federationprops.go`), the same ones the
federations operations use.

## Temporary passwords

Until set-password links exist (`web-email`), a new account gets its
first password like this
([../../docs/design/decisions/2026-11-18-temporary-passwords.md](../../docs/design/decisions/2026-11-18-temporary-passwords.md)):
the creating superadmin or admin gets a random 20-character temporary
password in the create answer, shown once, and hands it to the
account's owner. Only its Argon2id hash is stored; it is never logged
or audited. The account is flagged `must_change_password` and must
choose its own password at its first sign-in. `password-reset` does
the same for an existing account. When links arrive, creation and
reset send one instead and this path goes.

## Changing a password, and the forced change

`POST /api/account/password` (webapi/openapi.yaml) changes the
account's own password as the Drogon settings page does: the current
one must match, the new one must be at least 8 bytes, with no upper
limit below the body cap, and differ from the current one; it is
hashed with Argon2id. Every other session of the account ends on both
services, and this one gets a new cookie and CSRF token.

An account flagged `must_change_password` (one that signed in with a
temporary password) passes a gate first: until it has chosen its own
password, every API operation behind a session answers 403
`password_change_required`, except the session, logout and the
password change, and page data answers the `password_change` route.
The gate is one check in the request scope (`Scope.Gate`), applied by
`RequireSession`; the email gate will be the next one there.

## The Drogon pages

The Drogon pages know only "admin or not":

- superadmins and admins get today's admin pages;
- members are refused every page (403), and a member's session counts
  as none on the public chat pages, because those pages cannot apply
  grants and would show a member the whole archive;
- `--seed-admin` creates or resets a superadmin.
