import '../questionnaires/models.dart';
import 'data.dart';

class PreviewCase {
  final String id, title, group, path, state, gender;
  final String? formId, questionId;
  final bool signedIn, guardian, saved;
  final Map<String, Object?> answers;
  final List<String> actions;
  const PreviewCase(
    this.id,
    this.title,
    this.group,
    this.path, {
    this.state = '',
    this.gender = 'female',
    this.formId,
    this.questionId,
    this.signedIn = true,
    this.guardian = false,
    this.saved = false,
    this.answers = const {},
    this.actions = const [],
  });
  Map<String, Object?> toJson() => {
    'id': id,
    'title': title,
    'group': group,
    'path': path,
    'state': state,
    'formId': formId,
    'questionId': questionId,
    'saved': saved,
  };
}

List<PreviewCase> previewCases(DemoStudy study) {
  final cases = <PreviewCase>[
    const PreviewCase(
      'startup-loading',
      'Starting the app',
      'Startup',
      '/loading',
      state: 'startup-loading',
      signedIn: false,
    ),
    const PreviewCase(
      'startup-error',
      'Startup unavailable',
      'Startup',
      '/loading',
      state: 'startup-error',
      signedIn: false,
    ),
    const PreviewCase(
      'introduction',
      'Welcome',
      'Joining the study',
      '/introduction',
      signedIn: false,
    ),
    const PreviewCase(
      'information',
      'Study information',
      'Joining the study',
      '/introduction',
      signedIn: false,
      actions: ['show-information'],
    ),
    for (final state in [
      'consent',
      'consent-loading',
      'consent-error',
      'bankid-pending',
      'bankid-qr',
      'bankid-expired',
      'bankid-cancelled',
      'bankid-accepted',
    ])
      PreviewCase(
        state,
        {
          'consent': 'Review consent',
          'consent-loading': 'Loading consent',
          'consent-error': 'Consent unavailable',
          'bankid-pending': 'Continue in BankID',
          'bankid-qr': 'BankID QR signing',
          'bankid-expired': 'Expired QR code',
          'bankid-cancelled': 'Cancelled signing',
          'bankid-accepted': 'Consent signed',
        }[state]!,
        'Joining the study',
        '/introduction/login',
        state: state,
        signedIn: false,
      ),
    const PreviewCase('home', 'Checklist', 'Checklist', '/home'),
    for (final state in ['forms-loading', 'forms-error', 'forms-empty'])
      PreviewCase(
        state,
        {
          'forms-loading': 'Loading questionnaires',
          'forms-error': 'Questionnaires unavailable',
          'forms-empty': 'No enabled questionnaires',
        }[state]!,
        'Checklist',
        '/home',
        state: state,
      ),
    const PreviewCase(
      'guardian-required',
      'Guardian approval required',
      'Guardians',
      '/home',
      guardian: true,
    ),
    const PreviewCase(
      'guardians-new',
      'Create guardian links',
      'Guardians',
      '/home',
      guardian: true,
      actions: ['show-guardians'],
    ),
    const PreviewCase(
      'guardians-waiting',
      'Waiting for guardians',
      'Guardians',
      '/home',
      guardian: true,
      state: 'guardians-waiting',
      actions: ['show-guardians'],
    ),
    const PreviewCase(
      'guardians-approved',
      'Guardians approved',
      'Guardians',
      '/home',
      state: 'guardians-approved',
      actions: ['guardians-approved'],
    ),
    const PreviewCase(
      'guardians-error',
      'Guardian status unavailable',
      'Guardians',
      '/home',
      guardian: true,
      state: 'guardians-error',
      actions: ['show-guardians'],
    ),
    for (final state in [
      'idle',
      'reading',
      'denied',
      'empty',
      'ready',
      'uploading',
      'error',
      'complete',
    ])
      PreviewCase(
        'steps-$state',
        {
          'idle': 'Read movement history',
          'reading': 'Reading movement history',
          'denied': 'Health access denied',
          'empty': 'No health samples',
          'ready': 'Movement data ready',
          'uploading': 'Upload progress',
          'error': 'Upload failed',
          'complete': 'Upload complete',
        }[state]!,
        'Movement data',
        '/home',
        state: 'steps-$state',
      ),
    const PreviewCase(
      'step-details',
      'Movement details and chart',
      'Movement data',
      '/home',
      state: 'steps-ready',
      actions: ['show-step-details'],
    ),
    const PreviewCase(
      'account-menu',
      'Study and account menu',
      'Account',
      '/home',
      actions: ['participant-menu'],
    ),
    const PreviewCase(
      'receipt',
      'Signed consent receipt',
      'Account',
      '/consent-receipt',
    ),
    const PreviewCase(
      'receipt-loading',
      'Loading receipt',
      'Account',
      '/consent-receipt',
      state: 'receipt-loading',
    ),
    const PreviewCase(
      'receipt-error',
      'Receipt unavailable',
      'Account',
      '/consent-receipt',
      state: 'receipt-error',
    ),
    const PreviewCase(
      'withdraw',
      'Confirm withdrawal',
      'Account',
      '/consent-receipt',
      actions: ['withdraw-consent'],
    ),
    const PreviewCase(
      'done',
      'All tasks complete',
      'Completion',
      '/done',
      state: 'done',
    ),
  ];
  final pickerTypes = <bool>{};
  for (final form in study.forms) {
    final route = '/questionnaires/${form.summary.id}';
    if (form.questions.isEmpty) {
      cases.add(
        PreviewCase(
          'form-${form.summary.id}-empty',
          form.summary.name,
          'Questionnaires',
          route,
          formId: form.summary.id,
        ),
      );
    }
    for (final q in form.questions) {
      final context = questionContext(form, q);
      final path = '$route?question=${Uri.encodeQueryComponent(q.id)}';
      for (final saved in [false, true]) {
        cases.add(
          PreviewCase(
            'question-${form.summary.id}-${q.id}${saved ? '-saved' : ''}',
            '${form.summary.name} · ${q.text}${saved ? ' (saved)' : ''}',
            saved ? 'Saved answers' : 'Questionnaires',
            path,
            formId: form.summary.id,
            questionId: q.id,
            gender: context.gender,
            answers: {...context.answers, if (saved) q.id: sampleAnswer(q)},
            saved: saved,
          ),
        );
      }
      if (q.type == QuestionType.date &&
          pickerTypes.add(q.dateSettings.monthOnly)) {
        cases.add(
          PreviewCase(
            'picker-${q.dateSettings.monthOnly ? 'month' : 'day'}',
            q.dateSettings.monthOnly ? 'Month picker' : 'Date picker',
            'Questionnaires',
            path,
            formId: form.summary.id,
            questionId: q.id,
            gender: context.gender,
            answers: {...context.answers, q.id: sampleAnswer(q)},
            actions: ['date-${q.id}'],
          ),
        );
      }
    }
    if (form.questions.isNotEmpty) {
      final q = form.questions.first;
      final context = questionContext(form, q);
      for (final state in [
        'question-loading',
        'question-error',
        'question-validation',
        'question-submit-error',
        'question-submitting',
        'question-discard',
      ]) {
        cases.add(
          PreviewCase(
            '$state-${form.summary.id}',
            {
              'question-loading': 'Loading questionnaire',
              'question-error': 'Questionnaire unavailable',
              'question-validation': 'Answer validation',
              'question-submit-error': 'Submission failed',
              'question-submitting': 'Submitting answers',
              'question-discard': 'Confirm leaving unsaved answers',
            }[state]!,
            'Questionnaire states',
            '$route?question=${Uri.encodeQueryComponent(q.id)}',
            state: state,
            formId: form.summary.id,
            questionId: q.id,
            gender: context.gender,
            answers: {...context.answers, q.id: sampleAnswer(q)},
            actions: state == 'question-discard'
                ? ['questionnaire-close']
                : const [],
          ),
        );
      }
    }
  }
  if (cases.map((c) => c.id).toSet().length != cases.length) {
    throw StateError('Duplicate preview IDs');
  }
  return cases;
}
