# store-hold-batch

Hold a completed TDLib file for the whole time its store jobs are
being queued, so the first job cannot prune it under the next.

## Why

`handle_file_update` emits each pending use of a completed file one at
a time, and each emit takes a store hold and queues its job. A job on a
file-pool worker can finish and release the last hold before the loop
thread has taken the next job's hold, and TDLib's copy is then deleted
under that job. The window is microseconds, but the result is a message
left unlinked.

## Scope

- Take one hold for the file before the first emit and release it
  after the last (for example a batch hook from TDLib to TgLoggerd
  around the emits), keeping StoreHolds' prune-after-last rule.

## Depends on

Nothing.

## Done when

- A harness scenario where a job finishes between two emits fails
  before the change and passes after; no other scenario changes.
