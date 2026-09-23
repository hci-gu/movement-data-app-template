import 'dart:convert';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/bankid/gateway.dart';
import 'package:research_steps_template/bankid/models.dart';

class Transport implements HttpClientAdapter {
  int status = 200;
  Map<String, dynamic> body = {};
  RequestOptions? request;
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? stream,
    Future<void>? cancel,
  ) async {
    request = options;
    return ResponseBody.fromString(
      jsonEncode(body),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  test(
    'attempt result preserves credential, consent document and grant',
    () async {
      final transport = Transport();
      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'));
      dio.httpClientAdapter = transport;
      final gateway = HttpBankIdGateway(dio);
      const a = BankIdAttempt(
        id: 'attempt',
        secret: 'private-secret',
        expiresAt: 4102444800,
      );
      transport.body = {
        'id': 'attempt',
        'expiresAt': 4102444800,
        'status': 'accepted',
        'purpose': 'sign',
        'mode': 'sameDevice',
        'grant': {'token': 'signed-token'},
        'document': {
          'id': 'doc',
          'version': 'v1',
          'title': 'Consent',
          'text': 'Exact text',
          'documentHash': 'hash',
        },
      };
      final result = await gateway.status(a);
      expect(transport.request!.path, '/api/bankid/attempts/attempt');
      expect(
        transport.request!.headers['Authorization'],
        'Bearer attempt.private-secret',
      );
      expect(result.secret, a.secret);
      expect(result.grant!['token'], 'signed-token');
      expect(result.document!.text, 'Exact text');
      transport.status = 410;
      transport.body = {'message': 'Expired'};
      await expectLater(gateway.status(a), throwsA(isA<AttemptExpired>()));
    },
  );
  test(
    'start rejection decodes PocketBase reason code for re-review',
    () async {
      final transport = Transport()
        ..status = 409
        ..body = {
          'message': 'Read the current consent.',
          'data': {
            'reason': {
              'code': 'consentChanged',
              'message': 'Read the current consent.',
            },
          },
        };
      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = transport;
      final gateway = HttpBankIdGateway(dio);
      const a = BankIdAttempt(
        id: 'attempt',
        secret: 'private-secret',
        expiresAt: 4102444800,
      );
      await expectLater(
        gateway.start(a, {'clientSecret': a.secret}),
        throwsA(
          isA<BankIdStartRejected>().having(
            (e) => e.reason,
            'reason',
            'consentChanged',
          ),
        ),
      );
    },
  );
}
