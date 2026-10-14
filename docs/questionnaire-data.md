# Questionnaire data

Use a source document as context when populating PocketBase questionnaire definitions. Read its questions, instructions, answer formats, and valid choices, then resolve any ambiguities with the questionnaire owner before creating records. Do not invent choices, numeric limits, required-answer rules, or conditional behavior that the document or owner has not specified.

## Questionnaire and questions

Create a `questionnaires` record and link its questions through the `questions` relation. Set its `name`, `description`, and `enabled` status according to the intended questionnaire. Set `occurance` to the agreed frequency: `once`, `daily`, `weekly`, or `monthly`.

Create a `questions` record for each question, preserving the source wording and language. Choose the appropriate answer `type` and populate `introduction` and `placeholder` where the source provides instructions or an answer hint. Confirm whether repeated questions represent the same question or separate measurements before combining them.

If the source specifies conditional questions or required answers, clarify how the consuming application will implement those behaviors. Do not encode that logic in `questions.settings`.

## Option sets

Each `singleChoice` or `multiChoice` question must link through `questions.options` to one `questionOptions` record containing its valid options. Give the option set a descriptive `name`. Its `value` is an ordered JSON array of objects with `value` (a stable answer code) and `label` (display text in the source language).

Single choice answers use one code; multiple choice answers use an array of codes. Preserve the source's option wording and order, and keep answer codes stable when display labels change.

## Question settings version 1

`questions.settings` should only contain settings for the question answers, such as an allowed interval for integer answers or date precision. Do not encode additional logic here.

For an integer answer, use `interval` with inclusive `min` and `max` bounds and a `step` of `1`. Obtain the bounds from the source document or questionnaire owner. For a date answer, use `precision` and `format` to distinguish month-only answers (`month` / `YYYY-MM`) from full dates (`day` / `YYYY-MM-DD`). Do not fabricate a day for a month-only answer.

The consuming application must interpret and validate these answer settings. Storing them in PocketBase does not enforce them through the generic answer API.

## Applying the seed

Store the questionnaire seed in a Go migration under `api/migrations` so it can be reproduced in other environments. Create option sets first, then questions, then the questionnaire that references them. Keep existing records intact unless replacement has been explicitly requested, and make record ID conflicts fail rather than silently overwrite existing definitions.

From `api`, run `go run . migrate up --dir=./pb_data` using the server's actual data directory. PocketBase applies the migration transactionally and tracks it once in migration history. New builds also apply unapplied migrations on server startup. Restart an already running server after applying migrations manually.

Verify the created records, question relations, choice options, and answer settings against the source document and any clarifications from the owner.
