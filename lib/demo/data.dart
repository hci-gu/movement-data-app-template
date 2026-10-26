import 'dart:convert';
import '../enrollment/bankid/models.dart';
import '../questionnaires/models.dart';
import '../questionnaires/rules.dart';
import 'study_data.g.dart';

final demoNow = DateTime(2026, 10, 14, 12);

class DemoStudy {
  final ConsentDocument consent;
  final List<QuestionnaireDefinition> forms;
  final String sourceKind;
  DemoStudy(this.consent, this.forms, this.sourceKind);

  factory DemoStudy.generated() {
    final data = jsonObject(
      jsonDecode(utf8.decode(base64Decode(generatedStudyBase64))),
    );
    final forms = (data['questionnaires'] as List)
        .map((item) => QuestionnaireDefinition.fromJson(jsonObject(item)))
        .toList();
    // This extra example exists only in the empty template's demonstration.
    // A seeded study always uses its exported definitions without additions.
    if (data['sourceKind'] == 'template') {
      forms.add(
        const QuestionnaireDefinition(
          QuestionnaireSummary(id: 'demo-text', name: 'Text input example'),
          [
            QuestionDefinition(
              id: 'demo-comment',
              text: 'Any additional comments?',
              type: QuestionType.text,
            ),
          ],
        ),
      );
    }
    for (final form in forms) {
      checkDefinition(form);
    }
    return DemoStudy(
      ConsentDocument.fromJson(jsonObject(data['consent'])),
      forms,
      data['sourceKind'] as String,
    );
  }
}

Object sampleAnswer(QuestionDefinition q) => switch (q.type) {
  QuestionType.text => 'An example answer for this demonstration.',
  QuestionType.integer => 0.clamp(
    q.numberSettings.min ?? -9007199254740991,
    q.numberSettings.max ?? 9007199254740991,
  ),
  QuestionType.singleChoice => q.options.first.value,
  QuestionType.date => q.dateSettings.monthOnly ? '2025-06' : '2025-06-15',
};

/// Establish only the ancestors needed to make a question visible. Normal
/// visibility/validation rules remain authoritative, including audience checks.
({Map<String, Object?> answers, String gender}) questionContext(
  QuestionnaireDefinition form,
  QuestionDefinition target,
) {
  final byId = {for (final q in form.questions) q.id: q};
  final answers = <String, Object?>{};
  final audiences = <String>{};
  void include(QuestionDefinition q) {
    if (q.audience != 'all' && q.audience.isNotEmpty) audiences.add(q.audience);
    if (q.dependency.isEmpty) return;
    final parent = byId[q.dependency]!;
    include(parent);
    answers[parent.id] = q.dependencyValue is List
        ? (q.dependencyValue as List).first
        : q.dependencyValue;
  }

  include(target);
  if (audiences.length > 1) {
    throw StateError(
      'Question ${target.id} has conflicting ancestor audiences.',
    );
  }
  final gender = audiences.isEmpty ? 'female' : audiences.single;
  final participant = ParticipantContext('demo-participant', gender);
  if (!visibleQuestions(
        form,
        answers,
        participant,
      ).any((q) => q.id == target.id) ||
      validateAnswers(form, answers, participant, now: demoNow).isNotEmpty) {
    throw StateError('No valid preview context for ${target.id}.');
  }
  return (answers: answers, gender: gender);
}
