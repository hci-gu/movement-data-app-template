import 'package:flutter/cupertino.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/questionnaires/controller.dart';
import 'package:research_steps_template/questionnaires/models.dart';
import 'package:research_steps_template/questionnaires/questionnaire_screen.dart';
import 'package:research_steps_template/questionnaires/widgets/question_field.dart';
import 'package:research_steps_template/questionnaires/widgets/question_card.dart';
import 'fixtures.dart';

void main() {
  testWidgets(
    'date picker cancel does not set a value; confirm stores month only',
    (tester) async {
      Object? value;
      final question = fixtureDefinition().questions.first;
      await tester.pumpWidget(
        CupertinoApp(
          home: CupertinoPageScaffold(
            child: QuestionField(
              question: question,
              value: value,
              onChanged: (next) => value = next,
            ),
          ),
        ),
      );
      await tester.tap(find.text('ÅÅÅÅ-MM'));
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<CupertinoDatePicker>(find.byType(CupertinoDatePicker))
            .mode,
        CupertinoDatePickerMode.monthYear,
      );
      await tester.tap(find.text('Avbryt'));
      await tester.pumpAndSettle();
      expect(value, isNull);
      await tester.tap(find.text('ÅÅÅÅ-MM'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Välj'));
      await tester.pumpAndSettle();
      expect(value, matches(RegExp(r'^\d{4}-\d{2}$')));
    },
  );
  testWidgets(
    'form changes branches without moving answers and submits read-only',
    (tester) async {
      tester.view.physicalSize = const Size(430, 932);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final gateway = FakeQuestionnaireGateway();
      final container = ProviderContainer(
        overrides: [
          participantContextProvider.overrideWithValue(
            const ParticipantContext('u', 'female'),
          ),
          questionnaireGatewayProvider.overrideWithValue(gateway),
        ],
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const CupertinoApp(
            home: QuestionnaireScreen(questionnaireId: 'testfrm00000001'),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text('Option A'));
      await tester.tap(find.text('Option A'));
      await tester.pumpAndSettle();
      final provider = questionnaireControllerProvider('testfrm00000001');
      container.read(provider.notifier).setAnswer(qid(3), '2024-04-01');
      container.read(provider.notifier).setAnswer(qid(7), 3);
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text('Option C'));
      await tester.tap(find.text('Option C'));
      await tester.pumpAndSettle();
      expect(container.read(provider).answers.containsKey(qid(3)), false);
      expect(container.read(provider).answers[qid(7)], 3);
      await tester.ensureVisible(find.text('Skicka in svar'));
      await tester.tap(find.text('Skicka in svar'));
      await tester.pumpAndSettle();
      expect(gateway.saved, isNotNull);
      expect(gateway.saved!.answers[qid(7)], 3);
      expect(find.text('Skicka in svar'), findsNothing);
      expect(
        find.text('Dina svar är inskickade. Tack för ditt deltagande.'),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets('back prompts before discarding a draft', (tester) async {
    final gateway = FakeQuestionnaireGateway();
    final container = ProviderContainer(
      overrides: [
        participantContextProvider.overrideWithValue(
          const ParticipantContext('u', 'male'),
        ),
        questionnaireGatewayProvider.overrideWithValue(gateway),
      ],
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const CupertinoApp(
          home: QuestionnaireScreen(questionnaireId: 'testfrm00000001'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    container
        .read(questionnaireControllerProvider('testfrm00000001').notifier)
        .setAnswer(qid(7), 0);
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(CupertinoIcons.back));
    await tester.pumpAndSettle();
    expect(find.text('Lämna formuläret?'), findsOneWidget);
    await tester.tap(find.text('Stanna'));
    await tester.pumpAndSettle();
    expect(
      container
          .read(questionnaireControllerProvider('testfrm00000001'))
          .answers[qid(7)],
      0,
    );
  });
  testWidgets('rich text keeps emphasis but does not render scripts or images', (
    tester,
  ) async {
    await tester.pumpWidget(
      const CupertinoApp(
        home: QuestionnaireRichText(
          '<p>Hej <strong>världen</strong></p><script>bad()</script><img src="https://example.test/x">',
        ),
      ),
    );
    expect(find.textContaining('världen', findRichText: true), findsWidgets);
    expect(find.textContaining('bad()', findRichText: true), findsNothing);
    expect(find.byType(Image), findsNothing);
  });
  testWidgets('large text and error scrolling keep the form usable', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final gateway = FakeQuestionnaireGateway();
    final container = ProviderContainer(
      overrides: [
        participantContextProvider.overrideWithValue(
          const ParticipantContext('u', 'female'),
        ),
        questionnaireGatewayProvider.overrideWithValue(gateway),
      ],
    );
    addTearDown(container.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: CupertinoApp(
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: const TextScaler.linear(1.8)),
            child: child!,
          ),
          home: const QuestionnaireScreen(questionnaireId: 'testfrm00000001'),
        ),
      ),
    );
    await tester.pumpAndSettle();
    final provider = questionnaireControllerProvider('testfrm00000001');
    container.read(provider.notifier).setAnswer(qid(7), '-');
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.text('Skicka in svar'));
    await tester.tap(find.text('Skicka in svar'));
    await tester.pumpAndSettle();
    expect(find.text('Ange ett heltal.'), findsOneWidget);
    expect(
      tester
          .getRect(find.text('Ange ett heltal.'))
          .overlaps(const Rect.fromLTWH(0, 44, 390, 800)),
      true,
    );
    expect(gateway.submissions, 0);
    expect(tester.takeException(), isNull);
  });
}
