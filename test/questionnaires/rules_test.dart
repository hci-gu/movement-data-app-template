import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/questionnaires/models.dart';
import 'package:research_steps_template/questionnaires/rules.dart';
import 'fixtures.dart';

void main() {
  test(
    'fixture expansion preserves question order across branches and audiences',
    () {
      final d = fixtureDefinition();
      checkDefinition(d);
      expect(
        d.questions.map((q) => q.id),
        List.generate(12, (i) => qid(i + 1)),
      );
      final paths = {
        'branch_a': [2, 3, 5, 7, 8],
        'branch_b': [2, 4, 6, 7, 8, 9, 10],
        'branch_c': [2, 4, 6, 7, 8, 11, 12],
      };
      for (final gender in ['female', 'male', 'unknown']) {
        for (final entry in paths.entries) {
          final visible = visibleQuestions(d, {
            qid(2): entry.key,
          }, ParticipantContext('u', gender));
          expect(visible.map((q) => q.id), [
            if (gender == 'female') qid(1),
            ...entry.value.map(qid),
          ]);
        }
      }
      expect(
        visibleQuestions(
          d,
          {},
          const ParticipantContext('u', 'male'),
        ).map((q) => q.id),
        [qid(2), qid(7), qid(8)],
      );
    },
  );
  test(
    'branch changes remove stale answers, including a hidden parent chain',
    () {
      final d = fixtureDefinition();
      final clean = removeHiddenAnswers(d, {
        qid(2): 'branch_c',
        qid(3): '2024-01-01',
        qid(5): 4,
        qid(11): 0,
      }, const ParticipantContext('u', 'female'));
      expect(clean, {qid(2): 'branch_c', qid(11): 0});
      final chain = QuestionnaireDefinition(d.summary, [
        const QuestionDefinition(id: 'a', text: '', type: QuestionType.text),
        const QuestionDefinition(
          id: 'b',
          text: '',
          type: QuestionType.text,
          dependency: 'a',
          dependencyValue: 'yes',
        ),
        const QuestionDefinition(
          id: 'c',
          text: '',
          type: QuestionType.text,
          dependency: 'b',
          dependencyValue: 'yes',
        ),
      ]);
      checkDefinition(chain);
      expect(
        removeHiddenAnswers(chain, {
          'a': 'no',
          'b': 'yes',
          'c': 'old',
        }, const ParticipantContext('u', 'female')),
        {'a': 'no'},
      );
    },
  );
  test(
    'validation allows skipped answers and zero, rejects bad types and ranges',
    () {
      final d = fixtureDefinition();
      const p = ParticipantContext('u', 'female');
      expect(validateAnswers(d, {}, p), isEmpty);
      expect(validateAnswers(d, {qid(7): 0}, p), isEmpty);
      for (final bad in [-1, 2.5, '2', '-', true, double.infinity]) {
        expect(validateAnswers(d, {qid(7): bad}, p), contains(qid(7)));
      }
      expect(
        validateAnswers(d, {qid(2): 'branch_c', qid(11): 8}, p),
        contains(qid(11)),
      );
      expect(validateAnswers(d, {qid(2): 'invented'}, p), contains(qid(2)));
      expect(validateAnswers(d, {qid(1): '2026-02'}, p), isEmpty);
      expect(validateAnswers(d, {qid(1): '2026-02-01'}, p), contains(qid(1)));
      const date = QuestionDefinition(
        id: 'd',
        text: '',
        type: QuestionType.date,
        dateSettings: DateSettings(allowFuture: false),
      );
      expect(validateValue(date, '2026-02-30', DateTime(2026, 10)), isNotNull);
      expect(validateValue(date, '2026-11-01', DateTime(2026, 10)), isNotNull);
      expect(validateValue(date, '2024-02-29', DateTime(2026, 10)), isNull);
    },
  );
  test('bad definitions fail clearly instead of omitting fields', () {
    void fails(void Function(Map<String, dynamic>) change) {
      final json = fixtureJson();
      change(json);
      expect(() {
        final d = QuestionnaireDefinition.fromJson(json);
        checkDefinition(d);
      }, throwsA(isA<QuestionnaireConfigurationError>()));
    }

    Map<String, dynamic> question(Map<String, dynamic> j) =>
        (j['expand']['questions'] as List).first as Map<String, dynamic>;
    fails((j) => j['occurance'] = 'weekly');
    fails((j) => j['questions'].add(j['questions'].first));
    fails((j) => j['expand']['questions'].removeLast());
    fails((j) => question(j)['type'] = 'slider');
    fails((j) => question(j)['settings'] = {'required': true});
    fails((j) => question(j)['settings'] = {'min': 5, 'max': 2});
    fails((j) => question(j)['valueFromQuestion'] = qid(1));
    fails((j) => question(j)['dependency'] = 'missing');
    fails((j) => question(j)['dependency'] = question(j)['id']);
  });
}
