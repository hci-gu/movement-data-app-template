import 'dart:convert';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/enrollment/bankid/gateway.dart';
import 'package:research_steps_template/enrollment/bankid/models.dart';

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
    'authentication result preserves credential and session grant',
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
        'purpose': 'participant',
        'mode': 'sameDevice',
        'grant': {'token': 'authenticated-token'},
      };
      final result = await gateway.status(a);
      expect(transport.request!.path, '/api/bankid/attempts/attempt');
      expect(
        transport.request!.headers['Authorization'],
        'Bearer attempt.private-secret',
      );
      expect(result.secret, a.secret);
      expect(result.grant!['token'], 'authenticated-token');
      transport.status = 410;
      transport.body = {'message': 'Expired'};
      await expectLater(gateway.status(a), throwsA(isA<AttemptExpired>()));
    },
  );
  test(
    'start rejection decodes PocketBase reason code',
    () async {
      final transport = Transport()
        ..status = 409
        ..body = {
          'message': 'Start a new authentication request.',
          'data': {
            'reason': {
              'code': 'requestChanged',
              'message': 'Start a new authentication request.',
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
            'requestChanged',
          ),
        ),
      );
    },
  );
}
