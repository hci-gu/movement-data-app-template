import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../session/controller.dart';
import '../steps/controller.dart';
import '../questionnaires/controller.dart';

class HomeState {
  final bool eligible, stepsCompleted;
  final AsyncValue<List<QuestionnaireListEntry>> questionnaires;
  const HomeState({
    required this.eligible,
    required this.questionnaires,
    required this.stepsCompleted,
  });
  bool get canFinish =>
      eligible &&
      stepsCompleted &&
      !questionnaires.isLoading &&
      !questionnaires.hasError &&
      questionnaires.hasValue &&
      questionnaires.requireValue.every(
        (entry) => entry.completed && entry.summary.occurrence == 'once',
      );
}

final homeStateProvider = Provider<HomeState>((ref) {
  final eligible = ref.watch(
    sessionProvider.select((s) => s?.participant.canParticipate == true),
  );
  return HomeState(
    eligible: eligible,
    questionnaires: eligible
        ? ref.watch(questionnaireListProvider)
        : const AsyncLoading(),
    stepsCompleted: eligible && ref.watch(stepsProvider).completed,
  );
});
