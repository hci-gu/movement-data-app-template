# Questionnaire data

Use a source document as context when populating PocketBase questionnaire definitions. Read its questions, instructions, answer formats, and valid choices, then resolve any ambiguities with the questionnaire owner before creating records. Do not invent choices, numeric limits, required-answer rules, or conditional behavior that the document or owner has not specified.

## Questionnaire and questions

The baseline template creates the questionnaire collections without populating a study questionnaire. Test-only examples live in `api/internal/questionnaires/testdata/records.json`; tests load them into temporary databases. They are not seeded on application startup.

Create a `questionnaires` record and link its questions through the `questions` relation in display order. Set its `name`, `description`, and `enabled` status according to the intended questionnaire. The current app supports `occurance: once`. The schema retains other frequency values, but recurring questionnaires require additional implementation before use.

Create a `questions` record for each question, preserving the source wording and language. Choose the appropriate answer `type` and populate `introduction` and `placeholder` where the source provides instructions or an answer hint. Confirm whether repeated questions represent the same question or separate measurements before combining them.

If the source specifies conditional questions or required answers, clarify the intended behavior. The current app supports the explicit branching and audience fields below and allows every question to be skipped. Required-answer behavior would need corresponding client and server changes. Do not encode that logic in `questions.settings`.

## Option sets

Each `singleChoice` question must link through `questions.options` to one `questionOptions` record containing its valid options. Give the option set a descriptive `name`. Its `value` is an ordered JSON array of objects with `value` (a stable answer code) and `label` (display text in the source language).

Single choice answers use one code. Preserve the source's option wording and order, and keep answer codes stable when display labels change. Multiple choice is not implemented in the current app.

## Widget settings

`questions.settings` contains only widget-related settings, such as numeric bounds or date precision. Do not add `required`, `visibleWhen`, audience restrictions, or completion rules. Empty or missing settings use widget defaults.

| Type | Supported settings | Stored answer |
| --- | --- | --- |
| `text` | `{}`; input hint comes from the question's `placeholder` | String |
| `integer` | Optional top-level `min`, `max`, `unit` | Integer |
| `singleChoice` | `{}`; choices come from the related option set | Option's string `value` |
| `date` | `precision` (`day` or `month`), optional `allowFuture` | `YYYY-MM-DD` or `YYYY-MM` |

Numeric bounds are inclusive and must come from the source document or questionnaire owner. Do not wrap them in an `interval` object or add unsupported `step` settings. Zero is a valid answer and is distinct from unanswered.

Date precision defaults to `day`; `allowFuture` defaults to `true`. Do not invent a day for a month-only answer or store a symbolic date bound such as `max: "today"`. Opening or cancelling a date picker does not create an answer.

Unsupported settings, types, recurrence modes, and use of `valueFromQuestion` produce configuration errors. Slider, multiple choice, date interval, recurring schedules, response editing, and persistent drafts require additional implementation.

## Branching and audience

- `dependency` identifies a parent question. The parent must be visible and answered.
- `dependencyValue` is either a scalar that must equal the parent's stored answer, or an array containing its permitted answers. Compare stored option values, not display labels.
- `audience` is `all`, `female`, or `male`; an empty value means `all`. Participant responses expose the registered gender category derived from verified BankID identity. Unknown gender does not match a restricted audience.
- The frontend removes answers from hidden branches when an earlier answer changes. The backend rejects supplied answers to hidden or ineligible questions.
- Missing dependency targets, invalid dependency values, and cycles are configuration errors.

The implementation lives in `lib/questionnaires/` and `api/internal/questionnaires/`. Keep any future study-specific rules in readable source code with corresponding backend enforcement, outside widget settings.

## Responses and access

Questionnaires are reachable independently of health-data availability or upload completion. Session, consent, and guardian requirements still apply and are enforced by the backend.

All visible questions may be skipped. Explicit submission marks completion; it does not imply every question has an answer. Supplied answers must pass type, numeric, date, and option validation. Hidden and unknown question keys are rejected. Empty answers are omitted, and an explicitly submitted empty answer object is valid.

Drafts remain in memory, prompt before being discarded, and are cleared on logout or participant changes. Completed responses are read-only for participants.

An `answers` record contains `user`, `questionnaire`, `started`, and an `answers` object keyed by question ID. The server checks the authenticated participant and generates a hidden `submissionKey` of `<user>:<questionnaire>:once`. Transactional lookup/save and a unique index make retries return the original response. The client also retrieves an existing response after an uncertain submission result. Participant update/delete requests are denied.

These checks run in the application's PocketBase hooks, including direct collection requests. Run the Go backend from this repository so its validators are registered; collection settings alone do not enforce the contract.

Keep definitions and option values stable once responses exist. Publish a new questionnaire when the instrument changes. The schema migration preserves legacy duplicate responses, assigning the earliest response the once-only submission key.

## Applying the seed

Store the questionnaire seed in a Go migration under `api/migrations` so it can be reproduced in other environments. Create option sets first, then questions, then the questionnaire that references them. Keep existing records intact unless replacement has been explicitly requested, and make record ID conflicts fail rather than silently overwrite existing definitions.

From `api`, run `go run . migrate up --dir=./pb_data` using the server's actual data directory. PocketBase applies the migration transactionally and tracks it once in migration history. New builds also apply unapplied migrations on server startup. Restart an already running server after applying migrations manually.

Verify the created records, question relations, choice options, and answer settings against the source document and any clarifications from the owner.

Removing a seed file prevents future installations from applying it; it does not remove records from an existing database. Any cleanup of an existing study database is a separate data operation.

## Verification

Run `flutter analyze`, `flutter test`, and `go test ./...` from `api/`. Flutter and Go questionnaire tests share the synthetic fixture, cover branching and audience rules, and verify submission validation and retry behavior without creating study responses in the running database.
