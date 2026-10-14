import 'dart:convert';
import 'dart:async';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/questionnaires/gateway.dart';
import 'fixtures.dart';
import 'package:research_steps_template/api.dart';
import 'package:research_steps_template/pocketbase.dart';
import 'package:pocketbase/pocketbase.dart';

class Transport implements HttpClientAdapter {
  final Future<ResponseBody> Function(RequestOptions) respond;
  Transport(this.respond);
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? stream,
    Future<void>? cancel,
  ) => respond(options);
  @override
  void close({bool force = false}) {}
}

ResponseBody body(Object value, [int status = 200]) => ResponseBody.fromString(
  jsonEncode(value),
  status,
  headers: {
    Headers.contentTypeHeader: [Headers.jsonContentType],
  },
);
void main() {
  test('loads expanded relations and paginates forms', () async {
    final pages = <int>[];
    final client = Dio(BaseOptions(baseUrl: 'http://example.test'))
      ..httpClientAdapter = Transport((r) async {
        if (r.path.endsWith('/testfrm00000001')) {
          expect(r.queryParameters['expand'], 'questions.options');
          return body(fixtureJson());
        }
        final page = r.queryParameters['page'] as int;
        pages.add(page);
        return body({
          'items': [
            {
              'id': 'f$page',
              'name': 'Form $page',
              'enabled': true,
              'occurance': 'once',
            },
          ],
          'totalPages': 2,
        });
      });
    final gateway = HttpQuestionnaireGateway(client);
    expect((await gateway.list()).length, 2);
    expect(pages, [1, 2]);
    expect((await gateway.load('testfrm00000001')).questions.length, 12);
  });
  test('lost POST response recovers the existing response', () async {
    final client = Dio(BaseOptions(baseUrl: 'http://example.test'))
      ..httpClientAdapter = Transport((r) async {
        if (r.method == 'POST') {
          throw DioException(
            requestOptions: r,
            type: DioExceptionType.receiveTimeout,
          );
        }
        expect(
          r.queryParameters['filter'],
          contains('questionnaire="testfrm00000001"'),
        );
        return body({
          'items': [
            {
              'id': 'saved',
              'questionnaire': 'testfrm00000001',
              'answers': {qid(7): 0},
            },
          ],
          'totalPages': 1,
        });
      });
    final saved = await HttpQuestionnaireGateway(
      client,
    ).submit('u', 'testfrm00000001', DateTime.now(), {qid(7): 0});
    expect(saved.id, 'saved');
    expect(saved.answers[qid(7)], 0);
  });
  test(
    'maps server field validation and routes consent/guardian errors',
    () async {
      var status = 400;
      Object response = {
        'message': 'Invalid',
        'data': {
          'answers': {
            qid(7): {'code': 'invalidAnswer', 'message': 'Ange ett heltal.'},
          },
        },
      };
      final client = Dio(BaseOptions(baseUrl: 'http://example.test'))
        ..httpClientAdapter = Transport((r) async => body(response, status));
      final reasons = <String>[];
      final gateway = HttpQuestionnaireGateway(
        client,
        onAccessDenied: reasons.add,
      );
      await expectLater(
        gateway.submit('u', 'q', DateTime.now(), {}),
        throwsA(
          isA<AnswerValidationError>().having(
            (e) => e.fields[qid(7)],
            'field',
            'Ange ett heltal.',
          ),
        ),
      );
      for (final reason in ['consentRequired', 'guardianRequired']) {
        status = 403;
        response = {
          'data': {
            'reason': {'code': reason},
          },
        };
        await expectLater(gateway.list(), throwsA(isA<DioException>()));
      }
      expect(reasons, ['consentRequired', 'guardianRequired']);
    },
  );
  test('an expired old request cannot invalidate a newer login', () async {
    final api = Api();
    final oldAdapter = api.api.httpClientAdapter;
    final gate = Completer<ResponseBody>();
    final sent = Completer<void>();
    api.init('http://example.test');
    api.api.httpClientAdapter = Transport((request) async {
      if (!sent.isCompleted) sent.complete();
      return gate.future;
    });
    var invalidations = 0;
    api.onSessionInvalid = () async {
      invalidations++;
    };
    addTearDown(() {
      api.api.httpClientAdapter = oldAdapter;
      api.onSessionInvalid = null;
      pb.authStore.clear();
    });
    pb.authStore.save('old-token', RecordModel({'id': 'old'}));
    final request = api.api.get('/api/collections/questionnaires/records');
    final expectation = expectLater(request, throwsA(isA<DioException>()));
    await sent.future;
    pb.authStore.save('new-token', RecordModel({'id': 'new'}));
    gate.complete(body({'message': 'expired'}, 401));
    await expectation;
    expect(invalidations, 0);
    api.api.httpClientAdapter = Transport(
      (r) async => body({'message': 'expired'}, 401),
    );
    await expectLater(
      api.api.get('/api/collections/questionnaires/records'),
      throwsA(isA<DioException>()),
    );
    expect(invalidations, 1);
  });
}
