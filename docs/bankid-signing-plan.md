# BankID and data model

This backend serves one study. There is no study identifier in configuration, records, tokens, or BankID payloads.

## Collections

| Collection | Purpose |
| --- | --- |
| `users` | One record per BankID personal number. Application field: `personalNumber`. PocketBase also maintains its required auth system fields. |
| `signatures` | Terminal BankID signing attempts and encrypted evidence. `user` is a relation to `users`. An accepted consent signature creates or finds the user. |
| `consent_texts` | Published consent versions and the current document. |
| `answers`, `dataUploads` | Records related to the authenticated user. |
| `questionnaires`, `questions`, `questionOptions` | Questionnaire definitions. |

The app shows the current consent before BankID. A successful BankID consent signature creates the user or finds the same record by personal number, saves the signature relation, and grants a session. Returning users sign the current consent in the same single step. Consent and withdrawal are read from signature records; they are not copied onto the user.

Signing attempts live in memory until terminal. Each terminal result is saved with provider outcome and encrypted evidence. Failed attempts may have no user relation because BankID did not establish an identity. A backend restart interrupts pending attempts.

PocketBase requires its auth collection's system email and password fields even with password login disabled. The backend creates a random internal password when it creates a user. Neither password authentication nor generic user record access is available to participants.
