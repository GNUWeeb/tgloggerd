# Decision log: 2026-11-18, Repair only unknown and service rows

Decided by the project owner on 2026-11-18, for
[../message-types.md](../message-types.md) section 4.9 and section 6,
question 4:

- The repair pass re-reads rows stored as `unknown` and service rows
  without `content_extra`. Media rows stored before the change are not
  re-read for their detail.
- Reason: the unknown and service rows are the ones whose meaning is
  missing; media rows already keep their file and caption, and
  re-reading all of them would be far more requests to TDLib.
