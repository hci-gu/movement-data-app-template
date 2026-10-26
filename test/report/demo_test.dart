import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/demo/cases.dart';
import 'package:research_steps_template/demo/data.dart';
import 'package:research_steps_template/demo/gallery.dart';
import 'package:research_steps_template/demo/services.dart';
import 'package:research_steps_template/infrastructure/http_client.dart';
import 'package:research_steps_template/questionnaires/models.dart';
import 'package:research_steps_template/study/content.dart';

void main() {
  testWidgets('gallery opens the real introduction and returns to case list', (
    tester,
  ) async {
    await tester.pumpWidget(DemoGallery(study: DemoStudy.generated()));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Welcome'));
    await tester.pumpAndSettle();
    expect(find.text(StudyContent.title), findsOneWidget);
    expect(find.text('DEMO · sample data'), findsOneWidget);
    await tester.tap(find.text('Gallery'));
    await tester.pumpAndSettle();
    expect(find.text('Explore every screen'), findsOneWidget);
    expect(find.byType(DemoPreview), findsNothing);
  });

  test(
    'unexpected HTTP calls are blocked before reaching a transport',
    () async {
      final runtime = DemoRuntime(
        DemoStudy.generated(),
        const PreviewCase('test', 'Test', 'Test', '/home'),
      );
      try {
        await expectLater(
          runtime.container
              .read(httpClientProvider)
              .get('https://example.invalid'),
          throwsException,
        );
        expect(runtime.blockedRequests, 1);
      } finally {
        runtime.dispose();
      }
    },
  );

  test(
    'unreachable audience branches fail instead of disappearing from report',
    () {
      const form = QuestionnaireDefinition(
        QuestionnaireSummary(id: 'f', name: 'Form'),
        [
          QuestionDefinition(
            id: 'parent',
            text: 'Parent',
            type: QuestionType.singleChoice,
            audience: 'female',
            options: [QuestionOption('Yes', 'yes')],
          ),
          QuestionDefinition(
            id: 'child',
            text: 'Child',
            type: QuestionType.text,
            audience: 'male',
            dependency: 'parent',
            dependencyValue: 'yes',
          ),
        ],
      );
      expect(
        () => questionContext(form, form.questions.last),
        throwsStateError,
      );
    },
  );
}
