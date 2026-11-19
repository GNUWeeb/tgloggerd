# Decision log: 2026-11-19, Old rich rows are re-read, not re-mapped

Decided on 2026-11-19, for
[../message-types.md](../message-types.md) section 4.9 and the
msg-rich-store item:

- The rich messages stored before their mapping (content type `rich`,
  only TDLib's JSON in `content_extra`) are re-read with `getMessage`
  and stored again through the normal path, which writes their text,
  their curated keys and their media files. They are not re-mapped
  locally from the stored JSON.
- The re-read is part of the repair pass: it runs only when
  `TG_REPAIR_MESSAGES_ON_START` is set (`dry` counts them first, as
  `rich`), shares its paced queue, and keeps no state of its own. A
  rich row qualifies while it is live and its `content_extra` has no
  `blocks` key; the normal path writes that key, so a re-read row is
  never listed again and a pass cut short by a restart resumes where
  it was.
- A row whose message TDLib cannot return any more keeps its JSON, and
  is tried again on every start while the flag is on, like the other
  rows of the repair pass.
- Reason: the rows are few, and a re-read goes through the code every
  new message takes, so the old rows end up exactly like new ones,
  with fresh file references for their media. A local re-map would
  be a second mapping of some thirty block types from the JSON, to be
  kept in step with the first, and would still have to fetch every
  message again to download its media. The earlier decision that
  dropped the general local re-map
  ([2026-11-18-msg-repair-opt-in](2026-11-18-msg-repair-opt-in.md))
  stands; this one only adds the rich rows to the re-read.
