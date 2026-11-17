# Decision log: 2026-11-18, Shared stories kept as references only

Decided by the project owner on 2026-11-18, for
[../message-types.md](../message-types.md) section 6, question 3:

- A message that shares or replies to a story stores the story's
  poster chat and story id only. Stories are not fetched or archived.
- Reason: archiving stories means a new table, a fetch per story while
  it is live, and more storage, for content that is not a message of
  the logged chats.
