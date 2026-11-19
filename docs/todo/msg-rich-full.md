# msg-rich-full

Store a rich message in full when TDLib delivers it without all of
its blocks.

## Why

`richMessage.is_full` false means the object lacks part of the
message, and only `getFullRichMessage` returns the rest. msg-rich-store
stores what arrives, so such a message keeps a partial text, summary
and set of block files.

## Scope

- Daemon (TDLib session): when a stored rich message is not full, ask
  `getFullRichMessage`, re-read the message and store it again through
  the normal path with the full blocks, paced and once per message per
  run, without writing an edit version.

## Depends on

Nothing.

## Done when

- A rich message delivered as not full ends up stored with all of its
  blocks' text and files, with no edit version.
