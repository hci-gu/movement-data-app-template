# BankID and data model

This backend serves one study. There is no study identifier in configuration, records, tokens, or BankID payloads.

## Collections

| Collection | Purpose |
| --- | --- |
| `users` | One record per BankID personal number. Application field: `personalNumber`. PocketBase also maintains its required auth system fields. |
| `signatures` | Terminal BankID signing attempts and encrypted evidence. `user` is a relation to `users`. An accepted consent signature creates or finds the user. |
| `signingRequests` | One per selected guardian, related to the minor participant and, once accepted, the guardian's BankID signature. Its numeric `XXX-XXX` record ID is used directly in the signing link. |
| `consent_texts` | Published consent versions and the current document. |
| `answers`, `dataUploads` | Records related to the authenticated user. |
| `questionnaires`, `questions`, `questionOptions` | Questionnaire definitions. |

The app shows the current consent before BankID. A successful BankID consent signature creates the user or finds the same record by personal number, saves the signature relation, and grants a session. Returning users sign the current consent in the same single step. Consent and withdrawal are read from signature records; they are not copied onto the user.

The backend checks age from the participant's BankID identity. Adults can proceed to upload. Minors choose one or two guardians; the selected count is fixed when the first link is created. Each link starts a web initiated BankID signing request with the current consent text visible. The backend checks the guardian's identity, age, signature evidence, and consent version before unlocking upload. The guardian request's `signature` relation points to the accepted signature record. Existing links can be used to sign a new consent version.

Signing attempts live in memory until terminal. Each terminal result is saved with provider outcome and encrypted evidence. Failed attempts may have no user relation because BankID did not establish an identity. A backend restart interrupts pending attempts.

PocketBase requires its auth collection's system email and password fields even with password login disabled. The backend creates a random internal password when it creates a user. Neither password authentication nor generic user record access is available to participants.
