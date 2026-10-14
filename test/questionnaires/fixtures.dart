import 'dart:convert';
import 'dart:io';
import 'package:research_steps_template/questionnaires/models.dart';
import 'package:research_steps_template/questionnaires/gateway.dart';

Map<String, dynamic> fixtureJson() {
  final records =
      (jsonDecode(
                File(
                  'api/internal/questionnaires/testdata/records.json',
                ).readAsStringSync(),
              )
              as List)
          .cast<Map<String, dynamic>>();
  final options = {
    for (final r in records.where((r) => r['collection'] == 'questionOptions'))
      r['data']['id']: r['data'],
  };
  final questions = records.where((r) => r['collection'] == 'questions').map((
    r,
  ) {
    final q = Map<String, dynamic>.from(r['data'] as Map);
    if (q['options'] != '') q['expand'] = {'options': options[q['options']]};
    return q;
  }).toList();
  return {
    ...Map<String, dynamic>.from(
      records.firstWhere((r) => r['collection'] == 'questionnaires')['data']
          as Map,
    ),
    'expand': {'questions': questions.reversed.toList()},
  };
}

QuestionnaireDefinition fixtureDefinition() =>
    QuestionnaireDefinition.fromJson(fixtureJson());
String qid(int i) => 'testqst${i.toString().padLeft(8, '0')}';

class FakeQuestionnaireGateway implements QuestionnaireGateway {
  QuestionnaireDefinition definition = fixtureDefinition();
  QuestionnaireResponse? saved;
  int submissions = 0;
  Object? failure;
  Future<QuestionnaireDefinition>? pendingLoad;
  Future<QuestionnaireResponse>? pendingSubmit;
  @override
  Future<List<QuestionnaireSummary>> list() async => [definition.summary];
  @override
  Future<QuestionnaireDefinition> load(String id) async =>
      pendingLoad ?? definition;
  @override
  Future<QuestionnaireResponse?> response(
    String participantId,
    String questionnaireId,
  ) async => saved;
  @override
  Future<List<QuestionnaireResponse>> responses(String participantId) async =>
      saved == null ? [] : [saved!];
  @override
  Future<QuestionnaireResponse> submit(
    String participantId,
    String questionnaireId,
    DateTime started,
    Map<String, Object?> answers,
  ) async {
    submissions++;
    if (failure != null) throw failure!;
    if (pendingSubmit != null) return pendingSubmit!;
    return saved = QuestionnaireResponse(
      'response',
      questionnaireId,
      Map.of(answers),
    );
  }
}
