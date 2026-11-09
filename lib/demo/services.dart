import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import '../enrollment/consent_review_controller.dart';
import 'package:dio/dio.dart';
import 'package:health/health.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../app/router.dart';
import '../enrollment/bankid/controller.dart';
import '../enrollment/bankid/gateway.dart';
import '../enrollment/bankid/models.dart';
import '../enrollment/gateway.dart';
import '../enrollment/guardians_controller.dart';
import '../infrastructure/http_client.dart';
import '../questionnaires/controller.dart';
import '../questionnaires/gateway.dart';
import '../questionnaires/models.dart';
import '../session/controller.dart';
import '../session/gateway.dart';
import '../session/models.dart';
import '../session/store.dart';
import '../steps/controller.dart';
import '../steps/health_reader.dart';
import '../steps/movement.dart';
import '../questionnaires/scoliosis.dart';
import '../steps/upload_card.dart';
import '../steps/upload_gateway.dart';
import '../steps/upload_store.dart';
import 'cases.dart';
import 'data.dart';

class MemoryCredentials implements SessionStore, BankIdAttemptStore {
  Map<String, dynamic>? value;
  @override
  Future<Map<String, dynamic>?> read() async => value;
  @override
  Future<void> write(Map<String, dynamic> value) async {
    this.value = Map.of(value);
  }

  @override
  Future<void> clear() async {
    value = null;
  }
}

class DemoSessionGateway implements SessionGateway {
  Participant participant;
  DemoSessionGateway(this.participant);
  @override
  Future<Participant> current(String token) async => participant;
  @override
  Future<void> logout(String token) async {}
}

class DemoForms implements QuestionnaireGateway {
  final DemoStudy study;
  final PreviewCase preview;
  final saved = <String, QuestionnaireResponse>{};
  DemoForms(this.study, this.preview) {
    if (preview.saved) {
      saved[preview.formId!] = QuestionnaireResponse(
        'demo-response',
        preview.formId!,
        preview.answers,
      );
    }
    if (preview.state == 'done' || preview.state.startsWith('steps-')) {
      for (final form in study.forms) {
        saved[form.summary.id] = QuestionnaireResponse(
          'demo-response-${form.summary.id}',
          form.summary.id,
          const {treatmentQuestionId: 'surgery', surgeryDateQuestionId: '2025-04-15'},
        );
      }
    }
  }
  @override
  Future<List<QuestionnaireSummary>> list() async {
    if (preview.state == 'forms-loading') {
      return Completer<List<QuestionnaireSummary>>().future;
    }
    if (preview.state == 'forms-error') {
      throw StateError('Demo discovery error');
    }
    return preview.state == 'forms-empty'
        ? []
        : study.forms.map((f) => f.summary).toList();
  }

  @override
  Future<QuestionnaireDefinition> load(String id) async =>
      study.forms.firstWhere((f) => f.summary.id == id);
  @override
  Future<QuestionnaireResponse?> response(String user, String id) async =>
      saved[id];
  @override
  Future<List<QuestionnaireResponse>> responses(String user) async =>
      saved.values.toList();
  @override
  Future<QuestionnaireResponse> submit(
    String user,
    String id,
    DateTime started,
    Map<String, Object?> answers,
  ) async =>
      saved[id] = QuestionnaireResponse('demo-response', id, Map.of(answers));
}

class DemoQuestionController extends QuestionnaireController {
  final PreviewCase preview;
  DemoQuestionController({
    required this.preview,
    required super.gateway,
    required super.participant,
    required super.questionnaireId,
    super.onSubmitted,
  }) : super(now: () => demoNow);
  @override
  Future<void> load() async {
    if (preview.state == 'question-loading') return;
    if (preview.state == 'question-error') {
      state = const QuestionnaireState(
        loading: false,
        error: 'Kunde inte läsa formuläret. Försök igen.',
      );
      return;
    }
    await super.load();
    if (!mounted || state.definition == null) return;
    state = QuestionnaireState(
      definition: state.definition,
      response: state.response,
      answers: state.response?.answers ?? Map.of(preview.answers),
      loading: false,
      dirty: preview.state == 'question-discard',
      submitting: preview.state == 'question-submitting',
      errors: preview.state == 'question-validation'
          ? {preview.questionId!: 'Kontrollera ditt svar.'}
          : const {},
      error: preview.state == 'question-submit-error'
          ? 'Kunde inte nå servern. Dina svar finns kvar medan formuläret är öppet.'
          : null,
    );
  }
}

class DemoEnrollment implements EnrollmentGateway {
  final DemoStudy study;
  final PreviewCase preview;
  final requests = <GuardianRequest>[];
  DemoEnrollment(this.study, this.preview) {
    if (['guardians-waiting', 'guardians-approved'].contains(preview.state)) {
      requests.addAll([
        const GuardianRequest(
          '198001010001',
          'https://example.invalid/demo/guardian-one',
          true,
        ),
        GuardianRequest(
          '198001010002',
          'https://example.invalid/demo/guardian-two',
          preview.state == 'guardians-approved',
        ),
      ]);
    }
  }
  @override
  Future<GuardianStatus> guardians() async {
    if (preview.state == 'guardians-error') {
      throw StateError('Demo guardian error');
    }
    return GuardianStatus(
      count: requests.isEmpty ? 0 : requests.length,
      eligible: preview.state == 'guardians-approved',
      requests: List.of(requests),
    );
  }

  @override
  Future<GuardianStatus> createGuardian(
    String personalNumber,
    int count,
  ) async {
    // Demonstration values never leave the process.
    requests.add(
      GuardianRequest(
        personalNumber,
        'https://example.invalid/demo/guardian-${requests.length + 1}',
        false,
      ),
    );
    return GuardianStatus(count: count, requests: List.of(requests));
  }

  @override
  Future<ConsentReceipt> receipt() async {
    if (preview.state == 'receipt-loading') {
      return Completer<ConsentReceipt>().future;
    }
    if (preview.state == 'receipt-error') {
      throw StateError('Demo receipt error');
    }
    return ConsentReceipt(
      study.consent,
      demoNow.subtract(const Duration(days: 1)),
      'active',
    );
  }

  @override
  Future<void> withdraw() async {}
}

class DemoBankIdGateway implements BankIdGateway {
  final ConsentDocument document;
  final Map<String, dynamic> grant;
  DemoBankIdGateway(this.document, this.grant);
  @override
  Future<Uint8List?> qrImage(BankIdAttempt attempt) async => demoQRImage;
  @override
  Future<BankIdAttempt> start(
    BankIdAttempt attempt,
    Map<String, dynamic> input,
  ) async => BankIdAttempt(
    id: 'demo-acceptance',
    secret: 'demo',
    expiresAt: 4102444800,
    status: 'accepted',
    grant: grant,
  );
  @override
  Future<BankIdAttempt> status(BankIdAttempt attempt) async => attempt;
  @override
  Future<BankIdAttempt> returned(BankIdAttempt attempt, String nonce) async =>
      attempt;
  @override
  Future<BankIdAttempt> cancel(BankIdAttempt attempt) async =>
      const BankIdAttempt(
        id: 'demo',
        secret: 'demo',
        expiresAt: 4102444800,
        status: 'cancelled',
      );
}

class DemoBankIdController extends BankIdController {
  final PreviewCase preview;
  DemoBankIdController(
    this.preview,
    ConsentDocument document, {
    required super.gateway,
    required super.onGrant,
  }) : super(
         store: MemoryCredentials(),
         launch: (_) async => true,
         now: () => demoNow,
       ) {
    final kind = preview.state;
    final order = kind.startsWith('bankid-');
    state = AuthenticationState(
      begun: true,
      qrImage: kind == 'bankid-qr' ? demoQRImage : null,
      attempt: order
          ? BankIdAttempt(
              id: 'demo-order',
              secret: 'demo',
              expiresAt: 4102444800,
              status: kind == 'bankid-accepted'
                  ? 'accepted'
                  : kind == 'bankid-cancelled'
                  ? 'cancelled'
                  : 'pending',
              hintCode: kind == 'bankid-cancelled' ? 'userCancel' : 'userSign',
              mode: kind == 'bankid-qr' || kind == 'bankid-expired'
                  ? 'qr'
                  : 'sameDevice',
              qrSecondsRemaining: kind == 'bankid-expired' ? 0 : 90,
            )
          : null,
    );
  }
  @override
  Future<void> restore() async {}
  @override
  Future<void> initialize() async {}
  @override
  Future<void> finish() =>
      preview.state == 'bankid-accepted' ? Future.value() : super.finish();
}

List<HealthDataPoint> demoSamples() => List.generate(14, (i) {
  final day = demoNow.subtract(Duration(days: 14 - i));
  return movementTypes.map(
    (type) => HealthDataPoint(
      NumericHealthValue(switch (type) {
        HealthDataType.STEPS => (3200 + (i * 1379) % 7000).toDouble(),
        HealthDataType.DISTANCE_WALKING_RUNNING => 1800.0 + i * 100,
        HealthDataType.WALKING_STEP_LENGTH => .55 + i * .01,
        HealthDataType.WALKING_SPEED => 1.0 + i * .02,
        HealthDataType.WALKING_ASYMMETRY_PERCENTAGE => .01 + i * .001,
        _ => .25 + i * .003,
      }),
      type,
      switch (type) {
        HealthDataType.STEPS => HealthDataUnit.COUNT,
        HealthDataType.WALKING_SPEED => HealthDataUnit.METERS_PER_SECOND,
        HealthDataType.WALKING_ASYMMETRY_PERCENTAGE ||
        HealthDataType.WALKING_DOUBLE_SUPPORT_PERCENTAGE =>
          HealthDataUnit.PERCENT,
        _ => HealthDataUnit.METER,
      },
      DateTime(day.year, day.month, day.day),
      DateTime(day.year, day.month, day.day, 23),
      PlatformType.IOS,
      'demo-device',
      'demo-source',
      'iPhone',
    ),
  );
}).expand((samples) => samples).toList();

class DemoHealthReader implements HealthReader {
  @override
  Future<HealthRead> read() async => HealthRead(true, demoSamples());
}

class DemoUpload implements UploadGateway {
  @override
  Future<void> upload(
    List<HealthDataPoint> samples,
    CancelToken token,
    void Function(double) progress,
  ) async {
    progress(1);
  }
}

class MemoryUploadStore implements UploadStore {
  DateTime? value;
  @override
  Future<DateTime?> read() async => value;
  @override
  Future<void> write(DateTime value, bool Function() current) async {
    if (current()) this.value = value;
  }
}

class DemoStepsController extends StepsController {
  DemoStepsController(PreviewCase preview, {required super.isCurrent})
    : super(
        reader: DemoHealthReader(),
        gateway: DemoUpload(),
        store: MemoryUploadStore(),
        now: () => demoNow,
      ) {
    final name = preview.state.replaceFirst('steps-', '');
    final phase = preview.state == 'done'
        ? StepsPhase.complete
        : StepsPhase.values.where((p) => p.name == name).firstOrNull ??
              StepsPhase.idle;
    state = StepsState(
      phase: phase,
      progress: phase == StepsPhase.uploading ? .5 : 0,
      samples:
          [
            StepsPhase.ready,
            StepsPhase.uploading,
            StepsPhase.error,
            StepsPhase.complete,
          ].contains(phase)
          ? demoSamples()
          : const [],
      completedAt: phase == StepsPhase.complete ? demoNow : null,
      error: phase == StepsPhase.error
          ? 'Uppladdningen kunde inte bekräftas. Försök igen.'
          : null,
    );
  }
}

class DemoRuntime {
  final PreviewCase preview;
  final DemoStudy study;
  late final ProviderContainer container;
  int blockedRequests = 0;
  DemoRuntime(this.study, this.preview) {
    final participant = Participant(
      id: 'demo-participant',
      gender: preview.gender,
      guardianRequired: preview.guardian,
      consentRequired: preview.state.startsWith('consent'),
    );
    final credentials = MemoryCredentials();
    final sessions = DemoSessionGateway(participant);
    final grant = <String, dynamic>{
      'token': 'demo-session',
      'expiresAt': 4102444800,
      'record': participant.toJson(),
    };
    if (preview.signedIn) credentials.value = grant;
    final forms = DemoForms(study, preview);
    container = ProviderContainer(
      overrides: [
        initialLocationProvider.overrideWithValue(preview.path),
        httpClientProvider.overrideWith((ref) {
          final client = Dio()
            ..interceptors.add(
              InterceptorsWrapper(
                onRequest: (options, handler) {
                  blockedRequests++;
                  handler.reject(
                    DioException(
                      requestOptions: options,
                      error:
                          'Network is disabled in the demo. Supply a sample gateway.',
                    ),
                  );
                },
              ),
            );
          ref.onDispose(() => client.close(force: true));
          return client;
        }),
        sessionStoreProvider.overrideWithValue(credentials),
        sessionGatewayProvider.overrideWithValue(sessions),
        bankIdProvider.overrideWith(
          (ref) => DemoBankIdController(
            preview,
            study.consent,
            gateway: DemoBankIdGateway(study.consent, grant),
            onGrant: (value) =>
                ref.read(sessionProvider.notifier).accept(value),
          ),
        ),
        questionnaireGatewayProvider.overrideWithValue(forms),
        questionnaireControllerProvider.overrideWith((ref, id) {
          final current = ref.watch(participantContextProvider);
          final controller = DemoQuestionController(
            preview: preview,
            gateway: forms,
            participant:
                current ??
                ParticipantContext('demo-participant', preview.gender),
            questionnaireId: id,
            onSubmitted: () => ref.invalidate(questionnaireListProvider),
          );
          if (current != null) unawaited(controller.load());
          return controller;
        }),
        consentReviewGatewayProvider.overrideWithValue(
          DemoConsentReview(study.consent, sessions, preview),
        ),
        enrollmentGatewayProvider.overrideWithValue(
          DemoEnrollment(study, preview),
        ),
        healthReaderProvider.overrideWithValue(DemoHealthReader()),
        openHealthSettingsProvider.overrideWithValue(() async => true),
        healthSourceNameProvider.overrideWithValue('Apple Hälsa'),
        stepsProvider.overrideWith((ref) {
          final session = ref.watch(sessionProvider);
          return DemoStepsController(
            preview,
            isCurrent: () =>
                session != null &&
                session.participant.canParticipate &&
                ref.read(sessionProvider)?.token == session.token,
          );
        }),
        if (preview.state == 'startup-loading')
          startupProvider.overrideWith((ref) => Completer<void>().future),
        if (preview.state == 'startup-error')
          startupProvider.overrideWith(
            (ref) => Future.error(StateError('Demo startup error')),
          ),
      ],
    );
  }
  Future<void> prepare() async {
    if (preview.state.startsWith('startup-')) return;
    await container.read(startupProvider.future);
    container.read(stepsProvider);
    if (preview.signedIn &&
        !preview.state.startsWith('consent') &&
        !preview.guardian &&
        !['forms-loading', 'forms-error'].contains(preview.state)) {
      await container.read(questionnaireListProvider.future);
    }
  }

  void dispose() => container.dispose();
}

class DemoConsentReview implements ConsentReviewGateway {
  final ConsentDocument document;
  final DemoSessionGateway sessions;
  final PreviewCase preview;
  DemoConsentReview(this.document, this.sessions, this.preview);
  @override
  Future<ConsentDocument> current() async {
    if (preview.state == 'consent-loading') {
      return Completer<ConsentDocument>().future;
    }
    if (preview.state == 'consent-error') {
      throw StateError('Demo consent unavailable');
    }
    return document;
  }

  @override
  Future<void> accept(ConsentDocument document, String requestId) async {
    final p = sessions.participant;
    sessions.participant = Participant(
      id: p.id,
      gender: p.gender,
      guardianRequired: p.guardianRequired,
    );
  }
}

// Inert QR fixture generated for the gallery; never accepted by BankID.
final demoQRImage = base64Decode(
  'iVBORw0KGgoAAAANSUhEUgAAARgAAAEYAQMAAAC9QHvPAAAABlBMVEX///8AAABVwtN+AAABrUlEQVR42uyZvZHrIBSFP40ChZRAKZQmlUYplEBIoOG8AWSvbK+DF2wCnJD5lNy55/6JqampqT/SoqpoJQ8oLzrZ2qPvnTkBTLQBYNeSWc8tAdjJFGaVkolY7ySJGsNkpDAMY4OTBxbl9ZzMBwPOsx+T+WAASo7Jw04G+OLB7phH7Q3Ou2MnLyU+X+rzgMxDlZEA1vNLf+6NKd5JW3FKDUlrzFvCRCs/mRaxLQLOAypfsSUTjQK9M8AmRXDS5R0p8YYMy5SIlacAzu/iMdNG8zLTjstICUzpwi3Jal9WxAQ7AKMrGs7vh8pMUl9gMgBrW/9C81ctvbApYgOdM5d32kuZV/NyeedlXh2Wedbe61iQWT5i2C2zKgHYlkDKwP0yMDpTp5RWe3EHelbjCJMBWBNbdZPT8TPX6R7nXpnShevaDwVa8tWJeJnnx2UAoL44tfQ5wfy6C3fG/NwByrx6kKm7nlHETuZ+f3YeaMxbaemXed4Pb7ve+74zmZo/7Gr2+j2GgzPALgA+/NUrw+P+LA+0tlMYq8nc/3t6YCdDOTq+nNl6Zaampqb+R/8GAJd+o6xy7WnrAAAAAElFTkSuQmCC',
);
