import 'package:dio/dio.dart';
import 'models.dart';

abstract class QuestionnaireGateway {
  Future<List<QuestionnaireSummary>> list();
  Future<List<QuestionnaireResponse>> responses(String participantId);
  Future<QuestionnaireDefinition> load(String id);
  Future<QuestionnaireResponse?> response(
    String participantId,
    String questionnaireId,
  );
  Future<QuestionnaireResponse> submit(
    String participantId,
    String questionnaireId,
    DateTime started,
    Map<String, Object?> answers,
  );
}

class AnswerValidationError implements Exception {
  final Map<String, String> fields;
  const AnswerValidationError(this.fields);
}

class HttpQuestionnaireGateway implements QuestionnaireGateway {
  final Dio client;
  final String? sessionToken;
  HttpQuestionnaireGateway(this.client, {this.sessionToken});
  Options get _options => Options(
    headers: sessionToken == null
        ? null
        : {'Authorization': 'Bearer $sessionToken'},
  );

  String _quoted(String value) =>
      '"${value.replaceAll('\\', '\\\\').replaceAll('"', '\\"')}"';

  Future<List<Map<String, dynamic>>> _records(
    String collection,
    String filter,
  ) async {
    final records = <Map<String, dynamic>>[];
    var page = 1;
    while (true) {
      final result = await client.get(
        '/api/collections/$collection/records',
        options: _options,
        queryParameters: {
          'filter': filter,
          'page': page,
          'perPage': 100,
          'sort': 'created,id',
        },
      );
      final data = jsonObject(result.data);
      final items = data['items'];
      if (items is! List || data['totalPages'] is! num) {
        throw const QuestionnaireConfigurationError('Ogiltigt listsvar.');
      }
      records.addAll(items.map(jsonObject));
      if (page >= (data['totalPages'] as num)) break;
      page++;
    }
    return records;
  }

  @override
  Future<List<QuestionnaireSummary>> list() async => (await _records(
    'questionnaires',
    'enabled=true',
  )).map(QuestionnaireSummary.fromJson).toList();

  @override
  Future<List<QuestionnaireResponse>> responses(String participantId) async =>
      (await _records(
        'answers',
        'user=${_quoted(participantId)}',
      )).map(QuestionnaireResponse.fromJson).toList();

  @override
  Future<QuestionnaireDefinition> load(String id) async {
    final result = await client.get(
      '/api/collections/questionnaires/records/${Uri.encodeComponent(id)}',
      options: _options,
      queryParameters: {'expand': 'questions.options'},
    );
    final definition = QuestionnaireDefinition.fromJson(
      jsonObject(result.data),
    );
    return definition;
  }

  @override
  Future<QuestionnaireResponse?> response(
    String participantId,
    String questionnaireId,
  ) async {
    final rows = await _records(
      'answers',
      'user=${_quoted(participantId)} && questionnaire=${_quoted(questionnaireId)}',
    );
    return rows.isEmpty ? null : QuestionnaireResponse.fromJson(rows.first);
  }

  @override
  Future<QuestionnaireResponse> submit(
    String participantId,
    String questionnaireId,
    DateTime started,
    Map<String, Object?> answers,
  ) async {
    try {
      final result = await client.post(
        '/api/collections/answers/records',
        options: _options,
        data: {
          'user': participantId,
          'questionnaire': questionnaireId,
          'started': started.toUtc().toIso8601String(),
          'answers': answers,
        },
      );
      return QuestionnaireResponse.fromJson(jsonObject(result.data));
    } on DioException catch (e) {
      final body = e.response?.data;
      final data = body is Map ? body['data'] : null;
      final fields = data is Map ? data['answers'] : null;
      if (fields is Map) {
        throw AnswerValidationError({
          for (final entry in fields.entries)
            if (entry.value is Map && entry.value['message'] is String)
              entry.key.toString(): entry.value['message'] as String,
        });
      }
      if (e.response == null ||
          (e.response?.statusCode ?? 0) >= 500 ||
          e.response?.statusCode == 409) {
        // The write may have succeeded even if its response was lost.
        try {
          final saved = await response(participantId, questionnaireId);
          if (saved != null) return saved;
        } catch (_) {
          /* Preserve the original error and in-memory draft. */
        }
      }
      rethrow;
    }
  }
}

String questionnaireError(Object error) {
  if (error is QuestionnaireConfigurationError) return error.toString();
  if (error is DioException) {
    if (error.response?.statusCode == 404) {
      return 'Formuläret finns inte längre.';
    }
    if (error.response?.statusCode == 401) {
      return 'Logga in igen för att fortsätta.';
    }
    final body = error.response?.data;
    if (body is Map &&
        body['message'] is String &&
        (error.response?.statusCode ?? 500) < 500) {
      return body['message'] as String;
    }
    return 'Kunde inte nå servern. Dina svar finns kvar medan formuläret är öppet. Försök igen.';
  }
  return 'Kunde inte läsa formuläret. Försök igen.';
}
