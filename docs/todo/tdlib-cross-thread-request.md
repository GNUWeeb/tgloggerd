# tdlib-cross-thread-request

`TDLib::request()`: send a TDLib function from any thread except the
loop thread and wait for its answer, with a timeout.

## Why

Only the loop thread may call `send_query`; other threads can only
fire and forget (`deleteLocalFile`). The fetch service and the
internal API run on their own threads and need TDLib's answers
(`getRemoteFile`, `downloadFile`, `getMessage`, ...).

## Scope

- The public method on `TDLib` and its implementation in `TDLib::Impl`,
  as storage-cache.md section 4 describes: a mutex-guarded map of
  promises keyed by `aux_query_id_`, `client_manager_->send` from the
  calling thread, and `process_response` fulfilling promises for ids
  it does not find in `handlers_`.
- Errors as values: a TDLib `error` object comes back like any other
  answer; a request made while not authorized or while closing fails
  at once; pending promises fail when the loop stops.
- Not in scope: any caller (file-fetch-on-demand and the API use it).

## Design notes

- Never call it from the loop thread (it would wait for itself), and
  not from a `serial_` or `files_` job that the loop might block
  posting into (ThreadPool::post blocks when the queue is full).
  Document both in the header and assert the first.
- The existing aux ids of `deleteLocalFile` share the counter but have
  no promise; their answers are still dropped.
- No wake-up is needed: TDLib's reply makes `receive()` return.

## Depends on

Nothing.

## Done when

- tgloggerd builds without new warnings.
- A temporary self-test (not committed) that calls `request(getOption
  "version")` from another thread after login gets the answer; noted
  in the commit message as verified by the coordinator at cutover if
  it cannot run in a worktree.
