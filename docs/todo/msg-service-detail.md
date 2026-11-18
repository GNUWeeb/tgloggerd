# msg-service-detail

Recognise every service message class and store its action's fields:
who was added or removed, which message was pinned, the new owner, the
payment or gift, and the rest of message-types.md sections 3.3-3.5.

## Why

Many service messages are stored as `unknown`, and the recognised ones
keep their detail only in an English sentence or not at all.

## Scope

- `describe_service_message` covers every service class, with
  service_type names per decision
  [2026-11-18-msg-service-type-names](../design/decisions/2026-11-18-msg-service-type-names.md)
  and a description in the existing style.
- Curated `content_extra` for each, with message ids converted to
  server ids.
- Passport data received is recorded by kind only; its encrypted
  elements are not stored.

## Design notes

message-types.md sections 3.3-3.5 and 4.2.

## Depends on

Nothing.

## Done when

- Tests cover each service class's service_type, description and
  object; existing service_type values are unchanged.
