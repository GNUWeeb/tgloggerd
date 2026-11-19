# msg-edit-detail

Show what each earlier version of a message held when it was not text:
a live location's earlier positions, a checklist's earlier tasks.

## Why

The edit list of a live location (stored as one edit version a
minute), or of a checklist, shows one "(no text)" row per version. The
API sends each version's detail (`Edit.detail`, described in
web/docs/message-kinds.md), but the app does not draw it yet.

## Scope

- React (frontend): the edit list in the chat and the message page's
  edit history draw a version's detail compactly: a live location as
  its coordinates (each linked to the map) and time, a checklist as its
  tasks, any other kind with the card it has.
- Left out: a map track of the positions (a later, optional item if
  wanted).

## Design notes

`Edit.detail` is a MessageDetail or null, like the message's. A poll's
versions have null (the poll tables keep only the latest results);
they keep the text. A live location's version has `live.updates`, the
number of versions before it. A version shows no game, invoice or
paid media photos. The live location's edit list can be long: draw it
collapsed after a few versions.

## Depends on

Nothing.

## Done when

- A live location's edit list shows each earlier position with its
  time, in the chat and on the message page.
- React tests cover the list; the app's test fixtures give `Edit` its
  required `detail`.
