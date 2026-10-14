import 'dart:async';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../infrastructure/http_client.dart';
import '../session/controller.dart';
import 'gateway.dart';
import 'models.dart';
import 'rules.dart';

class QuestionnaireState {
  final QuestionnaireDefinition? definition;
  final QuestionnaireResponse? response;
  final Map<String, Object?> answers;
  final Map<String, String> errors;
  final bool loading, submitting, dirty;
  final String? error;
  const QuestionnaireState({
    this.definition,
    this.response,
    this.answers = const {},
    this.errors = const {},
    this.loading = true,
    this.submitting = false,
    this.dirty = false,
    this.error,
  });
}

class QuestionnaireController extends StateNotifier<QuestionnaireState> {
  final QuestionnaireGateway gateway;
  final ParticipantContext participant;
  final String questionnaireId;
  final DateTime started;
  final DateTime Function() now;
  final void Function()? onSubmitted;
  QuestionnaireController({
    required this.gateway,
    required this.participant,
    required this.questionnaireId,
    DateTime Function()? now,
    this.onSubmitted,
  }) : now = now ?? DateTime.now,
       started = (now ?? DateTime.now)(),
       super(const QuestionnaireState());

  Future<void> load() async {
    if (!mounted) return;
    state = const QuestionnaireState();
    try {
      final definition = await gateway.load(questionnaireId);
      if (!mounted) return;
      checkDefinition(definition);
      final saved = await gateway.response(participant.id, questionnaireId);
      if (!mounted) return;
      if (!definition.summary.enabled && saved == null) {
        throw const QuestionnaireConfigurationError(
          'Formuläret är inte längre tillgängligt.',
        );
      }
      state = QuestionnaireState(
        definition: definition,
        response: saved,
        answers: saved?.answers ?? const {},
        loading: false,
      );
    } catch (e) {
      if (mounted) {
        state = QuestionnaireState(
          loading: false,
          error: questionnaireError(e),
        );
      }
    }
  }

  void setAnswer(String id, Object? value) {
    if (!mounted) return;
    if (state.definition == null ||
        state.response != null ||
        state.submitting) {
      return;
    }
    final answers = removeHiddenAnswers(state.definition!, {
      ...state.answers,
      id: value,
    }, participant);
    state = QuestionnaireState(
      definition: state.definition,
      answers: Map.unmodifiable(answers),
      errors: state.errors.isEmpty
          ? const {}
          : validateAnswers(
              state.definition!,
              answers,
              participant,
              now: now(),
            ),
      loading: false,
      dirty: true,
    );
  }

  Future<bool> submit() async {
    if (!mounted) return false;
    final definition = state.definition;
    if (definition == null || state.submitting || state.response != null) {
      return false;
    }
    final answers = removeHiddenAnswers(definition, state.answers, participant);
    final errors = validateAnswers(
      definition,
      answers,
      participant,
      now: now(),
    );
    state = QuestionnaireState(
      definition: definition,
      answers: Map.unmodifiable(answers),
      errors: errors,
      loading: false,
      dirty: state.dirty,
      submitting: errors.isEmpty,
    );
    if (errors.isNotEmpty) return false;
    try {
      final saved = await gateway.submit(
        participant.id,
        questionnaireId,
        started,
        answers,
      );
      if (!mounted) return false;
      state = QuestionnaireState(
        definition: definition,
        response: saved,
        answers: saved.answers,
        loading: false,
      );
      onSubmitted?.call();
      return true;
    } catch (e) {
      if (!mounted) return false;
      state = QuestionnaireState(
        definition: definition,
        answers: Map.unmodifiable(answers),
        loading: false,
        dirty: true,
        errors: e is AnswerValidationError ? e.fields : const {},
        error: e is AnswerValidationError
            ? 'Kontrollera de markerade svaren.'
            : questionnaireError(e),
      );
      return false;
    }
  }
}

final participantContextProvider = Provider<ParticipantContext?>((ref) {
  final participant = ref.watch(
    sessionProvider.select(
      (s) => (
        id: s?.participant.id,
        gender: s?.participant.gender,
        allowed: s?.participant.canParticipate == true,
      ),
    ),
  );
  return !participant.allowed || participant.id == null
      ? null
      : ParticipantContext(participant.id!, participant.gender ?? 'unknown');
});

final questionnaireGatewayProvider = Provider<QuestionnaireGateway>((ref) {
  final token = ref.watch(sessionProvider.select((s) => s?.token));
  return HttpQuestionnaireGateway(
    ref.watch(httpClientProvider),
    sessionToken: token,
  );
});

class QuestionnaireListEntry {
  final QuestionnaireSummary summary;
  final bool completed;
  const QuestionnaireListEntry(this.summary, this.completed);
}

final questionnaireListProvider =
    FutureProvider.autoDispose<List<QuestionnaireListEntry>>((ref) async {
      final participant = ref.watch(participantContextProvider);
      final gateway = ref.watch(questionnaireGatewayProvider);
      if (participant == null) return [];
      final forms = await gateway.list();
      final responses = await gateway.responses(participant.id);
      final completed = responses.map((r) => r.questionnaire).toSet();
      return forms
          .map(
            (form) => QuestionnaireListEntry(form, completed.contains(form.id)),
          )
          .toList();
    });

final questionnaireControllerProvider = StateNotifierProvider.autoDispose
    .family<QuestionnaireController, QuestionnaireState, String>((ref, id) {
      final participant = ref.watch(participantContextProvider);
      final gateway = ref.watch(questionnaireGatewayProvider);
      final controller = QuestionnaireController(
        gateway: gateway,
        participant: participant ?? const ParticipantContext('', 'unknown'),
        questionnaireId: id,
        onSubmitted: () => ref.invalidate(questionnaireListProvider),
      );
      if (participant != null) unawaited(controller.load());
      return controller;
    });
