# join-chat-federations

Let whoever joins a chat (a superadmin, or an admin granted
`join_chat`) put the joined group into its federations.

## Why

Joining acts on the shared account, and a joined group should land in
the right federation (docs/design/web-accounts.md section 3).

## Scope

The Go service's half is built (web/docs/join-chat.md, "Filing the
joined group into federations"): `federation_ids` on
`POST /api/join-chat/join` files the group on `joined` and holds the
choice after `request_sent` or `approval_required`; the answer carries
`federation_ids` and `filing_pending`; the page data has `pending`; and
`DELETE /api/join-chat/pending/{id}` drops a held filing. What is left
is the React application's half:

- On the confirmation step, the page's `federations` as toggle chips
  (none selected = the group stays in Default), sent as
  `federation_ids`.
- After the join, say where the group went (`federation_ids`), or that
  it goes there once the chat approves the request (`filing_pending`).
- The held filings (`pending`) listed on the page, each with a button
  that drops it.

## Design notes

- Nothing is filed for `already_member`: moving a group the logger has
  is the federation editor's.

## Depends on

Nothing. (The per-admin `join_chat` grant that gates Join chat is
built: web/docs/accounts.md.)

## Done when

- An admin with `join_chat` joins a group and ticks two of its
  federations, and the group appears in both.
