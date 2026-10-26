# Study app design and copy

Read this guide before adapting the template. Apply it to real app widgets,
guardian browser pages and the final report. The template should demonstrate
these defaults; study adaptations may change colors and artwork while preserving
the layout and interaction principles.

## Navigation and hierarchy

- Put the study name or page title in `AppScaffold`'s navigation bar. Give the
  bar an opaque background and a subtle bottom separator, visible even at the top
  of the page (`automaticBackgroundVisibility: false`). Keep back/close controls
  and the account menu in the bar. Check safe areas on a physical iPhone.
- Use the body for information and tasks. Avoid repeating the navigation title
  or adding an inspirational headline above a checklist. A heading should identify
  the content that follows it.
- Center standalone study artwork horizontally inside an explicit size constraint.
  In the introduction, retain the `Center` and 112 × 112 artwork slot when replacing
  the icon with `Image.asset(..., fit: BoxFit.contain)`. Leave 24 logical pixels
  before the next section. Body paragraphs can remain left-aligned.
- Keep ordinary information, consent, receipts and checklists top-aligned and
  scrollable. A document must remain readable through its final paragraph.

## Status screens and actions

Center short BankID, signing-result and completion content vertically in the
available body below the navigation bar and inside safe areas. Center the whole
group of status, explanation, QR code and actions. Do not center only its heading
or position it using a fixed top spacer. `AppScrollBody(centerContent: true)`
provides this layout and lets taller content scroll. Guardian browser pages use
the same principle with a viewport minimum height and automatic margins.

For standalone status/error messages such as an unavailable receipt, also use
`TextAlign.center` so the message is horizontally aligned with its action.
Vertical centering alone does not center the text. Paragraphs inside cards and
long documents can remain left-aligned.

Use one clear primary action and only the secondary actions needed for that
state. During BankID signing, Cancel request is sufficient to cancel; do not add
a second Start over action. An expired QR code can offer renewal. A cancelled or
failed request offers Review consent and try again. Successful signing still
opens the checklist automatically; its briefly visible status follows the same
layout. Keep long consent review separate from the centered status treatment.

Use a consistent spacing rhythm: 8–12 logical pixels between related labels,
16 between explanatory text and its button, and 24 between sections. In
particular, guardian approval text must have 16 pixels before its filled button.
Buttons must accommodate wrapped text and retain accessible tap targets. Check
small phones and enlarged text; all actions must remain reachable by scrolling.

Display contact email addresses as recognizable, tappable `mailto:` links. Use
the link color and underline, keep a comfortable tap target, and open the mail
app with the displayed recipient address.

## Factual participant copy

Write plain, specific text explaining the study, the requested action or the
current result. Use the supplied study purpose and approved material. Do not
invent benefits, reassurance, availability, eligibility rules or promotional copy.

| Avoid | Use instead |
| --- | --- |
| “Din vardag. Vår gemensamma kunskap.” | A factual study purpose from the source material. |
| “One task at a time” above an already titled checklist | A short instruction about the actual tasks/order, when useful. |
| “Studien är inte öppen ännu. Du kan läsa…” added because setup is unfinished | Complete the setup and record unresolved requirements in delivery documentation. |
| TODOs, example contacts, “coming soon”, draft consent or inactive placeholder actions | Final source-backed content and working actions. |

Preserve approved consent and questionnaire wording. Do not rewrite it for tone
or invent missing choices. Keep newly drafted material for owner review outside
participant-facing screens until resolved. Genuine runtime errors should explain
what happened and offer an appropriate retry or contact action. A study closure
is appropriate only when explicitly required by the study owner and supported by
real application state; never infer it from incomplete development or setup.

The delivered study app must have **zero unfinished content placeholders**. Input
format hints such as YYYYMMDDXXXX are useful UI, and isolated report fixtures use
synthetic participant data. Neither is a substitute for final study copy,
contacts, branding or approved consent. See the [workflow](workflow.md#delivery-readiness)
for missing-input handling and the limits of automated checks.

## Visual acceptance

Review a freshly generated report after the last UI change:

- Report headings sit outside screenshot frames with a visible gap. Only the
  screenshots have rounded corners/borders/shadows. No route or case-ID footer
  appears around them; continuation images follow the same rule.
- The introduction artwork is centered; headings are factual; page titles read
  as navigation bars rather than floating body text.
- BankID pending, QR, expired, cancelled and accepted states are centered as a
  complete group. No Start over action appears. Done and short guardian browser
  statuses follow the same pattern.
- Guardian approval has breathing room above its action. Check comparable cards
  and sheets for text touching buttons.
- At 320 logical pixels wide and at 2× text size, content does not clip and the
  final action/document paragraph remains reachable. Review the ordinary 390 ×
  844 captures too. Check navigation, sheets, keyboard and safe areas on-device.
- Every participant-facing string is final and accurate. Review study content,
  contacts, assets, permission descriptions and enrollment readiness explicitly;
  passing a build or a placeholder scan does not establish this.
