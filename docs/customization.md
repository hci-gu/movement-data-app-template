# Adapting this app

This is a concrete app to edit, not a workflow engine. Read the application source
material in `context/`, then change the relevant widgets, functions and backend
rules directly. Add interfaces only for useful testing/platform boundaries.

The journey is **Introduction → BankID / consent → Home checklist → Done**.
Home lists enabled questionnaires and step upload in either task order. Guardian
approval is a conditional prerequisite on home. Information, guardian links and
step charts use optional sheets. The account menu includes consent and withdrawal.

| Change | Start here | Related work |
| --- | --- | --- |
| Introduction, explanation, contacts | `lib/study/` | Publish approved consent; review native/store metadata. |
| Navigation and startup | `lib/app/router.dart`, `app.dart` | Preserve return links and access guards. |
| Checklist tasks/completion | `lib/app/home_state.dart`, `home_screen.dart` | Compose feature state directly; operations stay in their feature. |
| Backend/return URL | `lib/app/environment.dart` | `API_BASE_URL` / `BANKID_RETURN_URL` build overrides; native schemes and associated domains must match. |
| Collection window, summaries, charts | `lib/steps/` | Read-only steps since January 1 five years before the current year; chart shows last 14 available days. |
| Replace steps with another measurement | Collection feature | Also change native permissions, approved consent, upload validation/storage/export and tests. |
| New questionnaire input | `lib/questionnaires/models.dart`, `widgets/`, `rules.dart` | Match backend validation in `api/internal/questionnaires/`; update data guide/fixtures. |
| Paging, focus and progress | `lib/questionnaires/questionnaire_screen.dart` | Visibility stays in rules; answers stay in the controller. |
| Ordinary branching/audience | PocketBase `dependency`, `dependencyValue`, `audience` | See [questionnaire data](questionnaire-data.md). Settings describe widget presentation only. |
| Study-specific eligibility | Named functions beside questionnaire rules | Enforce matching backend conditions; never trust client identity facts. |
| Replace BankID | `lib/enrollment/`, `lib/session/` | Also change backend authentication/evidence, native links and deployment. |
| Theme/formatting | `lib/ui/` | Keep feature-specific copy/formatting with its feature. |

## Ownership and invariants

`main.dart` initializes links and preferences. `app/` composes features and owns
navigation/lifecycle. `session/` owns typed sessions, server verification, expiry
and secure persistence. It never imports steps or questionnaires to reset them.
Providers observe stable session identity, dispose obsolete controllers, and
ignore late async results. Routine `/api/me` refreshes preserve questionnaire
drafts. Gateways pin requests to the token that started them.

Shared Dio transport owns authentication headers and access-denial handling.
Public consent and attempt-secret BankID calls never inherit participant tokens.
An old request's 401/403 cannot revoke or restrict a newer session. Server
verification precedes restored access. Return links trigger verification and never
grant access alone. See [BankID architecture](bankid-signing-plan.md).

Consent and guardian authorization remain server-enforced. Guardian approval
refreshes verified session facts; confirmed withdrawal invalidates the session.
Session changes dispose feature state, cancel pending chunks and prevent late
reads/uploads from marking completion. Participant detail sheets dismiss.
Age-based decisions use the verified BankID personal number on the backend;
do not add a participant age-group toggle or birth-date question for routing.
See [age and consent routing](workflow.md#age-and-consent-routing) when adapting
thresholds or selecting age-specific consent documents.

Questionnaire parsing belongs to gateway/models. Cross-question relationship
validation belongs to controller loading. Visibility recursively follows parent
visibility; branch changes remove hidden answers. The screen preserves page
position by question ID. Supplied answers are validated on explicit submission;
skips remain unanswered. Saved responses are read-only. The backend independently
checks definitions, answers, visibility and identity-derived audience. Registered
gender is derived in `api/internal/study/identity.go` and passed into rules.
Do not add `required`, `visibleWhen`, expressions or a condition registry to settings.

A valid nonempty answer advances to the next visible question after 500 ms
without another edit. Dates count as answered only on picker confirmation.
Manual interaction/navigation and app backgrounding cancel pending advancement;
the last page always requires explicit submission. Short question content is
centered vertically, while long content remains scrollable. Keep these behaviors
in the questionnaire screen and widgets, outside definition settings.

Step daily totals still sum rounded values by each sample's local start day.
Uploads preserve native `HealthDataPoint.toJson()` samples in gzip JSON
`{userId, chunkIndex, data}`, ordered using the existing roughly-ten-chunk partition.
Retry starts at chunk zero. Changing aggregation/deduplication is separate work.
Completion is persisted only after **all** chunks succeed. One `dataUploads` row
proves one chunk, not a complete dataset. Denied permission, no data or failed
chunks never complete the task. Health reading requires an explicit action.

Finish requires verified access, successful questionnaire discovery, saved form
responses and successful step upload. Loading, discovery errors and unsupported
recurring forms keep Finish disabled. There is no overall completion flag.
Restored sessions land on home, including completed participants. Done is entered
explicitly and protected against direct navigation.

## Local storage compatibility

| Storage | Key | Handling |
| --- | --- | --- |
| Secure session | `${API_BASE_URL}:authSessionV2` | Existing `token`, Unix `expiresAt`, `record` format retained; verify `/api/me` before access. Only needed participant facts are persisted. |
| Secure BankID attempt | `${API_BASE_URL}:bankidAttempt` | Existing attempt/credential format retained and verified with server. |
| Step completion | `${API_BASE_URL}:steps:${participantId}:completedAt` | ISO timestamp; retained across logout/relaunch for this backend/participant only. |
| Legacy preferences | `userId`, `hasUploadedData`, `lastUploadAt` | Removed during step-status restoration. No backend identity, so they cannot safely prove completion. Existing installations with only these flags must upload again. |

Credentials retain Flutter Secure Storage's existing iOS
`first_unlock_this_device` accessibility. Drafts live in memory while the form is
open; leaving a dirty form prompts. No offline queue or cross-device upload
completion recovery is introduced.

## Native health and deployment

The local `health/` path dependency keeps the existing native integration and raw
sample serialization under repository control. This restructuring does not
upgrade it. Dart code is in `health/lib/`, native code in `health/ios/` and
`health/android/`. App entitlements and descriptions are in `ios/Runner/`; Android
permissions and links are in `android/app/src/main/AndroidManifest.xml`. Consult
[the plugin README](../health/README.md) and test real devices before changing
platforms or versions. The reader retains the plugin's default Android backend;
changing health providers is separate work.

Before participant use, replace generic study copy, example contacts, native
names/identifiers and permission descriptions. Publish approved consent and review
keyrings, URLs, links and deployment using [the workflow](workflow.md). Keep
developer reminders in documentation and preserve source material in `context/`.

Run `flutter analyze`, `flutter test`, and `cd api && go test ./...`. Build Android
release and iOS simulator apps. Follow [manual review](manual-review.md) for
device-dependent checks.
