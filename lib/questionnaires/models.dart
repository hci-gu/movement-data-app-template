// PocketBase JSON is decoded here; widgets consume these typed definitions.
class QuestionnaireConfigurationError implements Exception {
  final String detail;
  const QuestionnaireConfigurationError(this.detail);
  @override
  String toString() => 'Formuläret har en ogiltig konfiguration: $detail';
}

Map<String, dynamic> jsonObject(Object? value) {
  if (value is! Map) {
    throw const QuestionnaireConfigurationError('JSON-objekt saknas.');
  }
  return Map<String, dynamic>.from(value);
}

String _string(Map<String, dynamic> json, String key, [String fallback = '']) {
  final value = json[key];
  if (value == null) return fallback;
  if (value is! String) throw QuestionnaireConfigurationError('Ogiltigt $key.');
  return value;
}

enum QuestionType { text, integer, singleChoice, date }

class NumberSettings {
  final int? min, max;
  final String unit;
  const NumberSettings({this.min, this.max, this.unit = ''});
  factory NumberSettings.fromJson(Map<String, dynamic> json) {
    int? bound(String key) {
      final value = json[key];
      if (value == null) return null;
      if (value is! num ||
          !value.isFinite ||
          value != value.roundToDouble() ||
          value.abs() > 9007199254740991) {
        throw QuestionnaireConfigurationError('Ogiltigt $key.');
      }
      return value.toInt();
    }

    final min = bound('min'), max = bound('max');
    if (min != null && max != null && min > max) {
      throw const QuestionnaireConfigurationError('Min är större än max.');
    }
    return NumberSettings(min: min, max: max, unit: _string(json, 'unit'));
  }
}

class DateSettings {
  final bool monthOnly, allowFuture;
  const DateSettings({this.monthOnly = false, this.allowFuture = true});
  factory DateSettings.fromJson(Map<String, dynamic> json) {
    final precision = _string(json, 'precision', 'day');
    if (!['day', 'month'].contains(precision) ||
        (json['allowFuture'] != null && json['allowFuture'] is! bool)) {
      throw const QuestionnaireConfigurationError(
        'Ogiltiga datuminställningar.',
      );
    }
    return DateSettings(
      monthOnly: precision == 'month',
      allowFuture: json['allowFuture'] as bool? ?? true,
    );
  }
}

class QuestionOption {
  final String label, value;
  const QuestionOption(this.label, this.value);
}

class QuestionDefinition {
  final String id, text, introduction, placeholder, audience, dependency;
  final Object? dependencyValue;
  final QuestionType type;
  final NumberSettings numberSettings;
  final DateSettings dateSettings;
  final List<QuestionOption> options;
  const QuestionDefinition({
    required this.id,
    required this.text,
    required this.type,
    this.introduction = '',
    this.placeholder = '',
    this.audience = 'all',
    this.dependency = '',
    this.dependencyValue,
    this.options = const [],
    this.numberSettings = const NumberSettings(),
    this.dateSettings = const DateSettings(),
  });

  factory QuestionDefinition.fromJson(Map<String, dynamic> json) {
    final name = _string(json, 'type');
    final supported = QuestionType.values.where((v) => v.name == name);
    if (supported.isEmpty) {
      throw QuestionnaireConfigurationError('Frågetypen $name stöds inte.');
    }
    final type = supported.single;
    if (_string(json, 'valueFromQuestion').isNotEmpty) {
      throw const QuestionnaireConfigurationError(
        'valueFromQuestion stöds inte.',
      );
    }
    final settings = json['settings'] == null
        ? <String, dynamic>{}
        : jsonObject(json['settings']);
    final allowed = switch (type) {
      QuestionType.integer => {'min', 'max', 'unit'},
      QuestionType.date => {'precision', 'allowFuture'},
      _ => <String>{},
    };
    if (settings.keys.any((key) => !allowed.contains(key))) {
      throw const QuestionnaireConfigurationError(
        'Inställningen stöds inte för frågetypen.',
      );
    }
    final audience = _string(json, 'audience');
    if (!['', 'all', 'female', 'male'].contains(audience)) {
      throw const QuestionnaireConfigurationError('Ogiltig målgrupp.');
    }
    final options = <QuestionOption>[];
    if (type == QuestionType.singleChoice) {
      final expanded = jsonObject(json['expand']);
      final optionRecord = jsonObject(expanded['options']);
      if (_string(optionRecord, 'id') != _string(json, 'options') ||
          _string(json, 'options').isEmpty) {
        throw const QuestionnaireConfigurationError('Svarsalternativ saknas.');
      }
      final values = optionRecord['value'];
      if (values is! List || values.isEmpty) {
        throw const QuestionnaireConfigurationError('Svarsalternativ saknas.');
      }
      final seen = <String>{};
      for (final value in values) {
        final item = jsonObject(value);
        final label = _string(item, 'label'), stored = _string(item, 'value');
        if (label.isEmpty || stored.isEmpty || !seen.add(stored)) {
          throw const QuestionnaireConfigurationError(
            'Ogiltiga eller dubbla svarsalternativ.',
          );
        }
        options.add(QuestionOption(label, stored));
      }
    }
    final id = _string(json, 'id');
    if (id.isEmpty) {
      throw const QuestionnaireConfigurationError('Fråge-ID saknas.');
    }
    return QuestionDefinition(
      id: id,
      text: _string(json, 'text'),
      type: type,
      introduction: _string(json, 'introduction'),
      placeholder: _string(json, 'placeholder'),
      audience: audience.isEmpty ? 'all' : audience,
      dependency: _string(json, 'dependency'),
      dependencyValue: json['dependencyValue'],
      options: List.unmodifiable(options),
      numberSettings: type == QuestionType.integer
          ? NumberSettings.fromJson(settings)
          : const NumberSettings(),
      dateSettings: type == QuestionType.date
          ? DateSettings.fromJson(settings)
          : const DateSettings(),
    );
  }
}

class QuestionnaireSummary {
  final String id, name, description, occurrence;
  final bool enabled;
  const QuestionnaireSummary({
    required this.id,
    required this.name,
    this.description = '',
    this.occurrence = 'once',
    this.enabled = true,
  });
  factory QuestionnaireSummary.fromJson(Map<String, dynamic> json) =>
      QuestionnaireSummary(
        id: _string(json, 'id'),
        name: _string(json, 'name'),
        description: _string(json, 'description'),
        occurrence: _string(json, 'occurance'),
        enabled: json['enabled'] == true,
      );
}

class QuestionnaireDefinition {
  final QuestionnaireSummary summary;
  final List<QuestionDefinition> questions;
  const QuestionnaireDefinition(this.summary, this.questions);
  factory QuestionnaireDefinition.fromJson(Map<String, dynamic> json) {
    final summary = QuestionnaireSummary.fromJson(json);
    if (summary.occurrence != 'once') {
      throw const QuestionnaireConfigurationError(
        'Återkommande formulär stöds inte ännu.',
      );
    }
    final ids = json['questions'];
    if (ids is! List ||
        ids.any((id) => id is! String) ||
        ids.toSet().length != ids.length) {
      throw const QuestionnaireConfigurationError('Ogiltig frågeordning.');
    }
    final expanded = json['expand'] == null
        ? <String, dynamic>{}
        : jsonObject(json['expand']);
    final records = expanded['questions'] ?? <dynamic>[];
    if (records is! List) {
      throw const QuestionnaireConfigurationError('Frågor saknas.');
    }
    final byId = <String, QuestionDefinition>{};
    for (final record in records) {
      final q = QuestionDefinition.fromJson(jsonObject(record));
      if (byId.containsKey(q.id)) {
        throw const QuestionnaireConfigurationError('Dubbla frågor.');
      }
      byId[q.id] = q;
    }
    if (ids.any((id) => !byId.containsKey(id))) {
      throw const QuestionnaireConfigurationError('Frågor saknas.');
    }
    return QuestionnaireDefinition(
      summary,
      List.unmodifiable(ids.map((id) => byId[id]!)),
    );
  }
}

class ParticipantContext {
  final String id, gender;
  const ParticipantContext(this.id, this.gender);
}

class QuestionnaireResponse {
  final String id, questionnaire;
  final Map<String, Object?> answers;
  const QuestionnaireResponse(this.id, this.questionnaire, this.answers);
  factory QuestionnaireResponse.fromJson(Map<String, dynamic> json) =>
      QuestionnaireResponse(
        _string(json, 'id'),
        _string(json, 'questionnaire'),
        Map.unmodifiable(jsonObject(json['answers'])),
      );
}
