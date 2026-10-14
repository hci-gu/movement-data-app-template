import 'models.dart';

bool isUnanswered(Object? value) => value == null || value == '';

void checkDefinition(QuestionnaireDefinition definition) {
  final byId = {for (final q in definition.questions) q.id: q};
  if (byId.length != definition.questions.length) {
    throw const QuestionnaireConfigurationError('Dubbla frågor.');
  }
  final visiting = <String>{}, done = <String>{};
  void visit(String id) {
    if (done.contains(id)) return;
    if (!visiting.add(id)) {
      throw const QuestionnaireConfigurationError('Cirkulärt frågeberoende.');
    }
    final q = byId[id];
    if (q == null) {
      throw const QuestionnaireConfigurationError('Beroendefråga saknas.');
    }
    if (q.dependency.isNotEmpty) {
      visit(q.dependency);
      final values = q.dependencyValue is List
          ? q.dependencyValue as List
          : [q.dependencyValue];
      if (values.isEmpty ||
          values.any(
            (value) =>
                isUnanswered(value) ||
                validateValue(byId[q.dependency]!, value, DateTime.now()) !=
                    null,
          )) {
        throw const QuestionnaireConfigurationError('Ogiltigt beroendevärde.');
      }
    } else if (q.dependencyValue != null) {
      throw const QuestionnaireConfigurationError('Beroendevärde utan fråga.');
    }
    visiting.remove(id);
    done.add(id);
  }

  for (final id in byId.keys) {
    visit(id);
  }
}

List<QuestionDefinition> visibleQuestions(
  QuestionnaireDefinition definition,
  Map<String, Object?> answers,
  ParticipantContext participant,
) {
  final byId = {for (final q in definition.questions) q.id: q};
  final result = <String, bool>{};
  bool visible(QuestionDefinition q) {
    if (result.containsKey(q.id)) return result[q.id]!;
    result[q.id] = false;
    if (q.audience != 'all' &&
        q.audience != '' &&
        q.audience != participant.gender) {
      return false;
    }
    if (q.dependency.isNotEmpty) {
      final parent = byId[q.dependency];
      final answer = answers[q.dependency];
      final matches = q.dependencyValue is List
          ? (q.dependencyValue as List).contains(answer)
          : q.dependencyValue == answer;
      if (parent == null ||
          !visible(parent) ||
          isUnanswered(answer) ||
          !matches) {
        return false;
      }
    }
    return result[q.id] = true;
  }

  return definition.questions.where(visible).toList();
}

Map<String, Object?> removeHiddenAnswers(
  QuestionnaireDefinition definition,
  Map<String, Object?> answers,
  ParticipantContext participant,
) {
  final ids = visibleQuestions(
    definition,
    answers,
    participant,
  ).map((q) => q.id).toSet();
  return {
    for (final entry in answers.entries)
      if (ids.contains(entry.key) && !isUnanswered(entry.value))
        entry.key: entry.value,
  };
}

String? validateValue(
  QuestionDefinition question,
  Object? value,
  DateTime now,
) {
  if (isUnanswered(value)) return null;
  switch (question.type) {
    case QuestionType.text:
      return value is String ? null : 'Ange text.';
    case QuestionType.integer:
      if (value is! num ||
          !value.isFinite ||
          value != value.roundToDouble() ||
          value.abs() > 9007199254740991) {
        return 'Ange ett heltal.';
      }
      final settings = question.numberSettings;
      if (settings.min != null && value < settings.min!) {
        return 'Ange minst ${settings.min}.';
      }
      if (settings.max != null && value > settings.max!) {
        return 'Ange högst ${settings.max}.';
      }
      return null;
    case QuestionType.singleChoice:
      return question.options.any((o) => o.value == value)
          ? null
          : 'Välj ett giltigt svarsalternativ.';
    case QuestionType.date:
      final monthOnly = question.dateSettings.monthOnly;
      final pattern = monthOnly ? r'^\d{4}-\d{2}$' : r'^\d{4}-\d{2}-\d{2}$';
      if (value is! String || !RegExp(pattern).hasMatch(value)) {
        return 'Ange ett giltigt datum.';
      }
      final parts = value.split('-').map(int.parse).toList();
      final day = monthOnly ? 1 : parts[2];
      final parsed = DateTime(parts[0], parts[1], day);
      if (parts[0] < 1 ||
          parsed.year != parts[0] ||
          parsed.month != parts[1] ||
          parsed.day != day) {
        return 'Ange ett giltigt datum.';
      }
      final today = DateTime(now.year, now.month, monthOnly ? 1 : now.day);
      if (!question.dateSettings.allowFuture && parsed.isAfter(today)) {
        return 'Datumet får inte ligga i framtiden.';
      }
      return null;
  }
}

Map<String, String> validateAnswers(
  QuestionnaireDefinition definition,
  Map<String, Object?> answers,
  ParticipantContext participant, {
  DateTime? now,
}) {
  final errors = <String, String>{};
  for (final q in visibleQuestions(definition, answers, participant)) {
    final error = validateValue(q, answers[q.id], now ?? DateTime.now());
    if (error != null) errors[q.id] = error;
  }
  return errors;
}
