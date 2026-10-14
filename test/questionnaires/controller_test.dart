import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/questionnaires/controller.dart';
import 'package:research_steps_template/questionnaires/models.dart';
import 'package:research_steps_template/questionnaires/gateway.dart';
import 'fixtures.dart';

void main() {
  QuestionnaireController controller(FakeQuestionnaireGateway gateway) =>
      QuestionnaireController(
        gateway: gateway,
        participant: const ParticipantContext('u', 'female'),
        questionnaireId: 'testfrm00000001',
      );
  test(
    'invalid input is retained and submission failure preserves draft',
    () async {
      final gateway = FakeQuestionnaireGateway();
      final c = controller(gateway);
      addTearDown(c.dispose);
      await c.load();
      c.setAnswer(qid(7), '-');
      expect(await c.submit(), false);
      expect(c.state.answers[qid(7)], '-');
      expect(gateway.submissions, 0);
      c.setAnswer(qid(7), 0);
      gateway.failure = Exception('offline');
      expect(await c.submit(), false);
      expect(c.state.answers[qid(7)], 0);
      expect(c.state.dirty, true);
      gateway.failure = null;
      expect(await c.submit(), true);
      expect(c.state.response, isNotNull);
      expect(c.state.dirty, false);
      c.setAnswer(qid(7), 5);
      expect(c.state.answers[qid(7)], 0);
    },
  );
  test(
    'conditional fields clear when switching branches and server errors map to fields',
    () async {
      final gateway = FakeQuestionnaireGateway();
      final c = controller(gateway);
      addTearDown(c.dispose);
      await c.load();
      c.setAnswer(qid(2), 'branch_a');
      c.setAnswer(qid(3), '2024-01-01');
      c.setAnswer(qid(2), 'branch_c');
      expect(c.state.answers.containsKey(qid(3)), false);
      gateway.failure = AnswerValidationError({qid(7): 'Server validation'});
      await c.submit();
      expect(c.state.errors[qid(7)], 'Server validation');
    },
  );
  test(
    'duplicate taps submit once; disposed sessions ignore late writes',
    () async {
      final gate = Completer<QuestionnaireResponse>();
      final gateway = FakeQuestionnaireGateway()..pendingSubmit = gate.future;
      final c = controller(gateway);
      await c.load();
      c.setAnswer(qid(7), 0);
      final submission = c.submit();
      expect(await c.submit(), false);
      expect(gateway.submissions, 1);
      var completed = false;
      final d = QuestionnaireController(
        gateway: gateway,
        participant: const ParticipantContext('old', 'female'),
        questionnaireId: 'testfrm00000001',
        onSubmitted: () => completed = true,
      );
      await d.load();
      final oldRequest = d.submit();
      d.dispose();
      c.dispose();
      gate.complete(
        const QuestionnaireResponse('saved', 'testfrm00000001', {}),
      );
      expect(await submission, false);
      expect(await oldRequest, false);
      expect(completed, false);
    },
  );
  test('disposed session ignores a late definition load', () async {
    final gate = Completer<QuestionnaireDefinition>();
    final gateway = FakeQuestionnaireGateway()..pendingLoad = gate.future;
    final c = controller(gateway);
    final load = c.load();
    c.dispose();
    gate.complete(fixtureDefinition());
    await load;
  });
  test(
    'existing response opens read-only; explicit empty submission is valid',
    () async {
      final gateway = FakeQuestionnaireGateway();
      final c = controller(gateway);
      await c.load();
      expect(await c.submit(), true);
      c.dispose();
      final d = controller(gateway);
      addTearDown(d.dispose);
      await d.load();
      expect(d.state.response, isNotNull);
      expect(await d.submit(), false);
    },
  );
  test(
    'participant change disposes the previous controller and drops its draft',
    () async {
      final identity = StateProvider(
        (ref) => const ParticipantContext('first', 'female'),
      );
      final gateway = FakeQuestionnaireGateway();
      final container = ProviderContainer(
        overrides: [
          participantContextProvider.overrideWith((ref) => ref.watch(identity)),
          questionnaireGatewayProvider.overrideWithValue(gateway),
        ],
      );
      addTearDown(container.dispose);
      final provider = questionnaireControllerProvider('testfrm00000001');
      final subscription = container.listen(provider, (_, _) {});
      addTearDown(subscription.close);
      await Future<void>.delayed(Duration.zero);
      final old = container.read(provider.notifier);
      old.setAnswer(qid(7), 4);
      container.read(identity.notifier).state = const ParticipantContext(
        'second',
        'male',
      );
      final next = container.read(provider.notifier);
      await Future<void>.delayed(Duration.zero);
      expect(identical(old, next), false);
      expect(next.participant.id, 'second');
      expect(next.state.answers, isEmpty);
      old.setAnswer(qid(7), 8); // A late picker callback must be harmless.
      expect(next.state.answers, isEmpty);
    },
  );
}
