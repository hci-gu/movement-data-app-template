import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/demo/data.dart';
import 'package:research_steps_template/questionnaires/models.dart';
import 'package:research_steps_template/questionnaires/rules.dart';
import 'package:research_steps_template/questionnaires/scoliosis.dart';

void main() {
  final form = DemoStudy.generated().forms.singleWhere(
    (f) => f.summary.id == scoliosisQuestionnaireId,
  );
  final now = DateTime(2026, 10, 26);
  const female = ParticipantContext('participant', 'female');
  const male = ParticipantContext('participant', 'male');
  test('all treatment branches preserve the supplied instrument', () {
    expect(form.questions.length, 13);
    for (final treatment in ['surgery', 'full_time', 'night']) {
      final dateKey = treatment == 'surgery'
          ? surgeryDateQuestionId
          : braceDateQuestionId;
      final answers = <String, Object?>{
        treatmentQuestionId: treatment,
        dateKey: '2025-04-15',
      };
      expect(validateAnswers(form, answers, female, now: now), isEmpty);
      final ids = visibleQuestions(
        form,
        answers,
        female,
      ).map((q) => q.id).toSet();
      expect(ids.length, treatment == 'surgery' ? 6 : 8);
      expect(ids.contains('scoliquestion01'), true);
      expect(visibleQuestions(form, answers, male).length, ids.length - 1);
      expect(
        ids.contains(
          treatment == 'surgery' ? braceDateQuestionId : surgeryDateQuestionId,
        ),
        false,
      );
    }
  });
  test(
    'type and matching date required while all other answers may be skipped',
    () {
      expect(
        validateAnswers(form, {}, female, now: now).keys,
        contains(treatmentQuestionId),
      );
      expect(
        validateAnswers(
          form,
          {treatmentQuestionId: 'night'},
          female,
          now: now,
        ).keys,
        contains(braceDateQuestionId),
      );
      expect(
        validateTreatment({
          treatmentQuestionId: 'surgery',
          surgeryDateQuestionId: '2025-02-30',
        }, now),
        isNotEmpty,
      );
      expect(
        validateTreatment({
          treatmentQuestionId: 'surgery',
          surgeryDateQuestionId: '2027-01-01',
        }, now),
        isNotEmpty,
      );
      final previous = <String, Object?>{
        treatmentQuestionId: 'surgery',
        surgeryDateQuestionId: '2025-04-15',
        'scoliquestion05': 3,
      };
      final changed = removeHiddenAnswers(form, {
        ...previous,
        treatmentQuestionId: 'night',
      }, female);
      expect(changed.containsKey(surgeryDateQuestionId), false);
      expect(changed.containsKey('scoliquestion05'), false);
      expect(
        validateAnswers(form, changed, female, now: now).keys,
        contains(braceDateQuestionId),
      );
    },
  );
}
