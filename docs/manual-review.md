# Manual review of the restructured app

Run against the backend containing your questionnaire definitions:

```sh
flutter run --dart-define=API_BASE_URL=http://YOUR_LAN_IP:8090
```

The default remains `http://192.168.10.144:8090`. BankID return settings, native
links and server configuration must agree; see [BankID testing](bankid-testing.md).

- [ ] Fresh launch shows introduction, one Continue action and optional study information.
- [ ] Generate/install the study icon using [the icon guide](app-icons.md), run `prepare-app-icons.py --check`, inspect the small exports and installed Home Screen icon, and verify the App Store icon in the release archive before upload.
- [ ] Review consent, enable signing, cancel/retry and complete same-device BankID return. Confirmed signing opens home automatically. Also try QR signing.
- [ ] Enrollment has no self-reported age-group toggle or birth-date question for routing. BankID test identities below, at and above the study's age threshold receive the backend-derived consent/guardian requirements. Any age-specific document is selected using verified identity and matches the text actually signed.
- [ ] Relaunch during BankID: enrollment resumes and verifies status; a link alone grants nothing.
- [ ] Home lists each questionnaire directly beside step upload. No forms is valid after successful discovery; discovery errors show Retry and disable Finish.
- [ ] For a guardian participant, tasks and questionnaire deep links are blocked. Generate/sign guardian links, refresh or resume, and confirm tasks unlock only after verification.
- [ ] Answer a question: after 0.5 seconds without another edit, a valid nonempty answer advances to the next visible question. Choice taps and confirmed dates advance; opening/cancelling the picker, cleared answers and invalid numbers do not. Typing another character restarts the delay. The last answer never submits automatically.
- [ ] Swipe questions vertically and use Previous/Next to skip or revisit questions. Manual navigation, touching another control, leaving the form and backgrounding the app cancel pending automatic navigation. Saved responses never advance by themselves.
- [ ] Short question content is vertically centered; long content scrolls within its page. Tap empty space at both ends of date/choice rows to check the full-width hit area. Check large text, keyboard, VoiceOver/TalkBack and reduced motion.
- [ ] Fill a conditional answer, change its parent and confirm hidden answers disappear and progress updates. Gender-specific questions use BankID identity facts.
- [ ] Only the final page offers Submit. Invalid supplied answers return to their page. Network failure preserves draft/page for retry. Leaving a dirty form prompts; Previous does not.
- [ ] Confirmed submission returns home. Completed rows open saved responses read-only.
- [ ] On a physical device, allow/deny health permission, check empty data, preview details, explicitly upload, interrupt and retry. Only full success completes the task. Home itself never requests health access.
- [ ] Complete questionnaire then steps; reverse the order with another test participant. Finish remains disabled until all tasks are confirmed.
- [ ] Finish opens Done. Review returns home without a redirect loop; relaunch also opens home with scoped step completion restored.
- [ ] Open information, receipt and sign-out from home and Done. Withdrawal signs out only after confirmed success.
- [ ] Sign out during a slow read/request/upload and use another test participant. No old draft, result or completion marker may appear.
- [ ] Generate `app_report.html` using [the report guide](app-report.md) after the last UI change. Review every image and long-content continuation, including all questionnaire branches, saved answers, sheets/dialogs and guardian browser pages. Test search, image enlargement and offline opening; include the report in the final delivery.

Use test identities/data. Actual BankID handoff, guardian signing and native health
checks require device interaction; automated fake gateways do not substitute for
them. The generated app report records automated verification results; record
physical-device results separately and keep remaining checks explicit.
