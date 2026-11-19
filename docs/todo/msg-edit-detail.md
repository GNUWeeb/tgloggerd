# msg-edit-detail

Show what each earlier version of a message held when it was not text:
a live location's earlier positions, a checklist's earlier tasks.

## Why

The `Edit` schema carries only text, content type and media, not
detail, so the edit list of a live location (stored as one edit
version a minute), or of a checklist, shows one "(no text)" row per
version. The positions are stored but cannot be seen.

## Scope

- Go (webapi): `Edit.detail`, the version's MessageDetail built from
  the edit row's `content_extra` (the edit tables have it since daemon
  migration 000034); OpenAPI and web/docs/message-kinds.md follow.
- React (frontend): the edit list in the chat and the message page's
  edit history draw a version's detail compactly: a live location as
  its coordinates (each linked to the map) and time, a checklist as its
  tasks, any other kind with the card it has.
- Left out: a map track of the positions (a later, optional item if
  wanted).

## Design notes

A poll's detail comes from the poll tables, which keep only the latest
results, so a poll's earlier versions have no detail of their own;
they keep the text. The live location's edit list can be long: draw it
collapsed after a few versions.

## Depends on

Nothing.

## Done when

- A live location's edit list shows each earlier position with its
  time, in the chat and on the message page.
- Go tests build a version's detail from `content_extra`; React tests
  cover the list.
