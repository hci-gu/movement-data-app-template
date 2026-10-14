import 'package:flutter/cupertino.dart';
import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/questionnaires/controller.dart';
import 'package:research_steps_template/questionnaires/models.dart';
import 'package:research_steps_template/questionnaires/gateway.dart';
import 'package:research_steps_template/questionnaires/questionnaire_screen.dart';
import 'package:research_steps_template/questionnaires/widgets/question_field.dart';
import 'package:research_steps_template/questionnaires/widgets/question_card.dart';
import 'fixtures.dart';

Future<void> lastPage(WidgetTester tester) async {
  for (var i = 0; i < 20 && find.text('Nästa').evaluate().isNotEmpty; i++) {
    await tester.tap(find.text('Nästa'));
    await tester.pumpAndSettle();
  }
}

Future<ProviderContainer> form(
  WidgetTester tester,
  FakeQuestionnaireGateway gateway, {
  double scale = 1,
}) async {
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
          data: MediaQuery.of(context).copyWith(
            textScaler: TextScaler.linear(scale),
            disableAnimations: true,
          ),
          child: child!,
        ),
        home: const QuestionnaireScreen(questionnaireId: 'testfrm00000001'),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return container;
}

QuestionnaireDefinition small(List<QuestionDefinition> questions) =>
    QuestionnaireDefinition(
      const QuestionnaireSummary(
        id: 'testfrm00000001',
        name: 'Test form',
        description: 'A short description',
      ),
      questions,
    );

void main() {
  testWidgets(
    'keyboard and large text keep navigation reachable with accessible progress',
    (tester) async {
      tester.view.physicalSize = const Size(320, 640);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetViewInsets);
      final semantics = tester.ensureSemantics();
      final gateway = FakeQuestionnaireGateway()
        ..definition = small(const [
          QuestionDefinition(id: 'a', text: 'First', type: QuestionType.text),
          QuestionDefinition(id: 'b', text: 'Second', type: QuestionType.text),
        ]);
      await form(tester, gateway, scale: 1.8);
      await tester.ensureVisible(find.byType(CupertinoTextField).first);
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byType(CupertinoTextField).hitTestable(),
        'Draft',
      );
      tester.view.viewInsets = const FakeViewPadding(bottom: 260);
      await tester.pump();
      expect(find.text('Nästa').hitTestable(), findsOneWidget);
      expect(find.bySemanticsLabel('Fråga 1 av 2: First'), findsOneWidget);
      await tester.tap(find.text('Nästa'));
      await tester.pump();
      // Reduced motion uses an immediate jump, with no transition to wait for.
      expect(find.text('Fråga 2 av 2'), findsOneWidget);
      expect(find.text('Skicka in svar').hitTestable(), findsOneWidget);
      expect(tester.takeException(), isNull);
      semantics.dispose();
    },
  );
  testWidgets(
    'submission reveals an invalid input beneath long instructions on a rebuilt page',
    (tester) async {
      final gateway = FakeQuestionnaireGateway()
        ..definition = small([
          QuestionDefinition(
            id: 'a',
            text: 'Invalid integer',
            type: QuestionType.integer,
            introduction: List.filled(25, '<p>Instructions</p>').join(),
          ),
          const QuestionDefinition(
            id: 'b',
            text: 'Middle',
            type: QuestionType.text,
          ),
          const QuestionDefinition(
            id: 'c',
            text: 'Last',
            type: QuestionType.text,
          ),
        ]);
      final container = await form(tester, gateway);
      container
          .read(questionnaireControllerProvider('testfrm00000001').notifier)
          .setAnswer('a', '-');
      await lastPage(tester);
      await tester.tap(find.text('Skicka in svar'));
      await tester.pumpAndSettle();
      expect(find.text('Fråga 1 av 3'), findsOneWidget);
      expect(find.text('Ange ett heltal.').hitTestable(), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );
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
      await tester.tap(find.text('Nästa'));
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
      await lastPage(tester);
      await tester.tap(find.text('Skicka in svar'));
      await tester.pumpAndSettle();
      expect(gateway.saved, isNotNull);
      expect(gateway.saved!.answers[qid(7)], 3);
      expect(find.text('Skicka in svar'), findsNothing);
      expect(find.text('Dina sparade svar').hitTestable(), findsOneWidget);
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
    await tester.tap(find.byIcon(CupertinoIcons.xmark));
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
    await lastPage(tester);
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

  testWidgets(
    'vertical snap and controls retain partial input, skip without defaults, and never auto submit',
    (tester) async {
      final gateway = FakeQuestionnaireGateway()
        ..definition = small(const [
          QuestionDefinition(
            id: 'a',
            text: 'First integer',
            type: QuestionType.integer,
          ),
          QuestionDefinition(
            id: 'b',
            text: 'Second text',
            type: QuestionType.text,
          ),
          QuestionDefinition(
            id: 'c',
            text: 'Last text',
            type: QuestionType.text,
          ),
        ]);
      final container = await form(tester, gateway);
      expect(find.text('Fråga 1 av 3'), findsOneWidget);
      expect(find.text('Second text').hitTestable(), findsNothing);
      await tester.enterText(
        find.byType(CupertinoTextField).hitTestable(),
        '-',
      );
      await tester.tap(find.text('Nästa'));
      await tester.pumpAndSettle();
      expect(find.text('Fråga 2 av 3'), findsOneWidget);
      await tester.drag(find.byType(PageView), const Offset(0, -400));
      await tester.pumpAndSettle();
      expect(find.text('Fråga 3 av 3'), findsOneWidget);
      expect(gateway.submissions, 0);
      await tester.tap(find.text('Föregående'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Föregående'));
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<CupertinoTextField>(
              find.byType(CupertinoTextField).hitTestable(),
            )
            .controller!
            .text,
        '-',
      );
      expect(
        container
            .read(questionnaireControllerProvider('testfrm00000001'))
            .answers,
        {'a': '-'},
      );
      await lastPage(tester);
      await tester.tap(find.text('Skicka in svar'));
      await tester.pumpAndSettle();
      expect(find.text('Fråga 1 av 3'), findsOneWidget);
      expect(find.text('Ange ett heltal.'), findsOneWidget);
      expect(gateway.submissions, 0);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'branch removal preserves active question by ID and uses preceding page if it disappears',
    (tester) async {
      final gateway = FakeQuestionnaireGateway()
        ..definition = small(const [
          QuestionDefinition(
            id: 'a',
            text: 'Branch',
            type: QuestionType.singleChoice,
            options: [QuestionOption('Yes', 'yes'), QuestionOption('No', 'no')],
          ),
          QuestionDefinition(
            id: 'b',
            text: 'Conditional',
            type: QuestionType.text,
            dependency: 'a',
            dependencyValue: 'yes',
          ),
          QuestionDefinition(id: 'c', text: 'Always', type: QuestionType.text),
        ]);
      final container = await form(tester, gateway);
      final c = container.read(
        questionnaireControllerProvider('testfrm00000001').notifier,
      );
      await tester.tap(find.text('Yes'));
      await tester.pumpAndSettle();
      expect(find.text('Fråga 1 av 3'), findsOneWidget);
      await tester.tap(find.text('Nästa'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byType(CupertinoTextField).hitTestable(),
        'hidden draft',
      );
      c.setAnswer('a', 'no');
      await tester.pumpAndSettle();
      expect(find.text('Fråga 1 av 2'), findsOneWidget);
      expect(c.state.answers.containsKey('b'), false);
      await tester.tap(find.text('Nästa'));
      await tester.pumpAndSettle();
      c.setAnswer('a', 'yes');
      await tester.pumpAndSettle();
      expect(find.text('Fråga 3 av 3'), findsOneWidget);
      expect(find.text('Always').hitTestable(), findsWidgets);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'server errors jump back; network retry preserves page and blocks duplicate submit',
    (tester) async {
      final gateway = FakeQuestionnaireGateway()
        ..definition = small(const [
          QuestionDefinition(id: 'a', text: 'First', type: QuestionType.text),
          QuestionDefinition(id: 'b', text: 'Last', type: QuestionType.text),
        ]);
      await form(tester, gateway);
      await tester.enterText(
        find.byType(CupertinoTextField).hitTestable(),
        'retained',
      );
      await lastPage(tester);
      gateway.failure = const AnswerValidationError({
        'a': 'Server rejected this answer',
      });
      await tester.tap(find.text('Skicka in svar'));
      await tester.pumpAndSettle();
      expect(find.text('Fråga 1 av 2'), findsOneWidget);
      expect(find.text('Server rejected this answer'), findsOneWidget);
      await lastPage(tester);
      gateway.failure = Exception('offline');
      await tester.tap(find.text('Skicka in svar'));
      await tester.pumpAndSettle();
      expect(find.text('Fråga 2 av 2'), findsOneWidget);
      gateway.failure = null;
      final gate = Completer<QuestionnaireResponse>();
      gateway.pendingSubmit = gate.future;
      await tester.tap(find.text('Skicka in svar'));
      await tester.pump();
      final buttons = tester.widgetList<CupertinoButton>(
        find.byType(CupertinoButton),
      );
      expect(buttons.every((b) => b.onPressed == null), true);
      await tester.drag(
        find.byType(PageView),
        const Offset(0, 300),
        warnIfMissed: false,
      );
      await tester.pump();
      expect(find.text('Fråga 2 av 2'), findsOneWidget);
      expect(gateway.submissions, 3);
      gate.complete(
        const QuestionnaireResponse('saved', 'testfrm00000001', {
          'a': 'retained',
        }),
      );
      await tester.pumpAndSettle();
      expect(find.text('Skicka in svar'), findsNothing);
    },
  );

  testWidgets(
    'empty and single-question forms require explicit submit; saved response is read-only',
    (tester) async {
      final gateway = FakeQuestionnaireGateway()..definition = small([]);
      await form(tester, gateway);
      expect(find.text('Inga frågor att visa'), findsOneWidget);
      expect(gateway.submissions, 0);
      await tester.tap(find.text('Skicka in svar'));
      await tester.pumpAndSettle();
      expect(gateway.saved!.answers, isEmpty);
      await tester.pumpWidget(const SizedBox());
      gateway.definition = small(const [
        QuestionDefinition(id: 'a', text: 'One', type: QuestionType.text),
      ]);
      gateway.saved = const QuestionnaireResponse('saved', 'testfrm00000001', {
        'a': 'Answer',
      });
      await form(tester, gateway);
      expect(find.text('Fråga 1 av 1'), findsOneWidget);
      expect(
        tester
            .widget<CupertinoTextField>(find.byType(CupertinoTextField))
            .readOnly,
        true,
      );
      expect(find.text('Skicka in svar'), findsNothing);
    },
  );

  testWidgets(
    'long instructions scroll inside the page before changing questions at the boundary',
    (tester) async {
      final gateway = FakeQuestionnaireGateway()
        ..definition = small([
          QuestionDefinition(
            id: 'a',
            text: 'Long question',
            type: QuestionType.text,
            introduction: List.filled(
              25,
              '<p>Long instructions for this question.</p>',
            ).join(),
          ),
          const QuestionDefinition(
            id: 'b',
            text: 'Next question',
            type: QuestionType.text,
          ),
        ]);
      await form(tester, gateway, scale: 1.8);
      await tester.drag(find.byType(PageView), const Offset(0, -250));
      await tester.pumpAndSettle();
      expect(find.text('Fråga 1 av 2'), findsOneWidget);
      // Explicit navigation remains reachable even before reaching the inner end.
      await tester.tap(find.text('Nästa'));
      await tester.pumpAndSettle();
      expect(find.text('Fråga 2 av 2'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );
}
