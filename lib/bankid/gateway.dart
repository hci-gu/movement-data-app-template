import 'package:dio/dio.dart';
import 'package:research_steps_template/api.dart';
import 'package:research_steps_template/bankid/models.dart';

abstract class BankIdGateway {
  Future<ConsentDocument> currentConsent();
  Future<Map<String, dynamic>> createFlow(BankIdFlow flow);
  Future<BankIdOrder> start(BankIdFlow flow);
  Future<BankIdOrder> status(BankIdFlow flow);
  Future<BankIdOrder> cancel(BankIdFlow flow);
  Future<BankIdOrder> returned(BankIdFlow flow, String nonce);
  Future<Map<String, dynamic>> complete(BankIdFlow flow);
  Future<void> abandon(BankIdFlow flow);
}

class HttpBankIdGateway implements BankIdGateway {
  final Dio client;
  HttpBankIdGateway([Dio? client]) : client = client ?? Api().api;
  Options _options([BankIdFlow? flow]) => Options(
    extra: {'public': true},
    headers: flow == null
        ? null
        : {'Authorization': 'Bearer ${flow.authorization}'},
  );
  Map<String, dynamic> _json(Response<dynamic> response) =>
      Map<String, dynamic>.from(response.data as Map);
  @override
  Future<ConsentDocument> currentConsent() async {
    final data = _json(
      await client.get('/api/study/consent/current', options: _options()),
    );
    if (data['bankidAvailable'] != true) throw const BankIdUnavailable();
    return ConsentDocument.fromJson(
      Map<String, dynamic>.from(data['document'] as Map),
    );
  }

  @override
  Future<Map<String, dynamic>> createFlow(BankIdFlow flow) async => _json(
    await client.post(
      '/api/study/enrollments',
      data: {
        'kind': flow.kind,
        'invitationCode': flow.invitationCode,
        'clientSecret': flow.clientSecret,
      },
      options: _options(),
    ),
  );
  @override
  Future<BankIdOrder> start(BankIdFlow flow) async => BankIdOrder.fromJson(
    _json(
      await client.post(
        '/api/study/bankid-orders',
        data: {
          'consentVersionId': flow.document?.id ?? '',
          'documentHash': flow.document?.documentHash ?? '',
          'mode': flow.mode,
          'requestKey': flow.requestKey,
        },
        options: _options(flow),
      ),
    ),
  );
  @override
  Future<BankIdOrder> status(BankIdFlow flow) async => BankIdOrder.fromJson(
    _json(
      await client.get(
        '/api/study/bankid-orders/${flow.orderId}',
        options: _options(flow),
      ),
    ),
  );
  @override
  Future<BankIdOrder> cancel(BankIdFlow flow) async => BankIdOrder.fromJson(
    _json(
      await client.post(
        '/api/study/bankid-orders/${flow.orderId}/cancel',
        options: _options(flow),
      ),
    ),
  );
  @override
  Future<BankIdOrder> returned(BankIdFlow flow, String nonce) async =>
      BankIdOrder.fromJson(
        _json(
          await client.post(
            '/api/study/bankid-orders/${flow.orderId}/return',
            data: {'nonce': nonce},
            options: _options(flow),
          ),
        ),
      );
  @override
  Future<void> abandon(BankIdFlow flow) async {
    await client.post(
      '/api/study/enrollments/${flow.id}/abandon',
      options: _options(flow),
    );
  }

  @override
  Future<Map<String, dynamic>> complete(BankIdFlow flow) async => _json(
    await client.post(
      '/api/study/enrollments/${flow.id}/complete',
      options: _options(flow),
    ),
  );
}

class BankIdUnavailable implements Exception {
  const BankIdUnavailable();
}

String bankIdError(Object error) {
  if (error is BankIdUnavailable) {
    return 'BankID signing is not available yet. Please try later.';
  }
  if (error is DioException) {
    final data = error.response?.data;
    if (data is Map &&
        data['message'] is String &&
        error.response!.statusCode! < 500) {
      return data['message'] as String;
    }
    return 'The service could not be reached. Check your connection and try again. Your request can be resumed.';
  }
  return 'The request could not be completed. Please try again.';
}
