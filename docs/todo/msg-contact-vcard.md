# msg-contact-vcard

Show what a shared contact's vCard holds beyond the name and number:
other numbers, e-mail addresses, the organisation (optional).

## Why

A contact's `vcard` is sent to viewers who may see users' phones, but
the card shows only the name and one number.

## Scope

- React (frontend): parse the vCard (a small parser, no library) for
  its numbers, e-mail addresses and organisation, show them under the
  contact, and offer the vCard as a `.vcf` download.
- Go: nothing; the phone rule for `vcard` stays as it is.

## Depends on

Nothing.

## Done when

- A contact whose vCard has two numbers and an e-mail address shows
  both numbers and the address, and downloads as a `.vcf`, for a
  viewer who may see phones; others see neither.
- A React test covers the parser and the phone rule.
