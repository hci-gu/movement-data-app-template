import 'package:dio/dio.dart';
import 'package:research_steps_template/api.dart';
import 'package:research_steps_template/bankid/models.dart';

abstract class BankIdGateway {
  Future<ConsentDocument> currentConsent();
  Future<BankIdAttempt> start(
    BankIdAttempt attempt,
    Map<String, dynamic> input,
  );
  Future<BankIdAttempt> status(BankIdAttempt attempt);
  Future<BankIdAttempt> cancel(BankIdAttempt attempt);
  Future<BankIdAttempt> returned(BankIdAttempt attempt, String nonce);
}

class HttpBankIdGateway implements BankIdGateway {
  final Dio client;
  HttpBankIdGateway([Dio? client]) : client = client ?? Api().api;
  Options _options([BankIdAttempt? attempt]) => Options(
    extra: {'public': true},
    headers: attempt == null
        ? null
        : {'Authorization': 'Bearer ${attempt.authorization}'},
  );
  Map<String, dynamic> _json(Response<dynamic> r) =>
      Map<String, dynamic>.from(r.data as Map);
  @override
  Future<ConsentDocument> currentConsent() async {
    final data = _json(
      await client.get('/api/consent/current', options: _options()),
    );
    if (data['bankidAvailable'] != true) throw const BankIdUnavailable();
    return ConsentDocument.fromJson(
      Map<String, dynamic>.from(data['document'] as Map),
    );
  }

  Future<BankIdAttempt> _result(
    BankIdAttempt a,
    Future<Response<dynamic>> call,
  ) async {
    try {
      return BankIdAttempt.fromJson(_json(await call), a.secret);
    } on DioException catch (e) {
      if (e.response?.statusCode == 410) throw const AttemptExpired();
      rethrow;
    }
  }

  @override
  Future<BankIdAttempt> start(
    BankIdAttempt a,
    Map<String, dynamic> input,
  ) async {
    try {
      return await _result(
        a,
        client.post('/api/bankid/attempts', data: input, options: _options()),
      );
    } on DioException catch (e) {
      if (e.response?.statusCode == 400 || e.response?.statusCode == 409) {
        final data = e.response?.data;
        var reason = '';
        if (data is Map && data['data'] is Map) {
          // PocketBase wraps custom error data in validation error objects.
          final detail = data['data']['reason'];
          reason = detail is Map ? detail['code'] as String? ?? '' : '';
        }
        throw BankIdStartRejected(
          reason,
          data is Map && data['message'] is String
              ? data['message'] as String
              : 'This request could not start.',
        );
      }
      rethrow;
    }
  }

  @override
  Future<BankIdAttempt> status(BankIdAttempt a) => _result(
    a,
    client.get('/api/bankid/attempts/${a.id}', options: _options(a)),
  );
  @override
  Future<BankIdAttempt> cancel(BankIdAttempt a) => _result(
    a,
    client.post('/api/bankid/attempts/${a.id}/cancel', options: _options(a)),
  );
  @override
  Future<BankIdAttempt> returned(BankIdAttempt a, String nonce) => _result(
    a,
    client.post(
      '/api/bankid/attempts/${a.id}/return',
      data: {'nonce': nonce},
      options: _options(a),
    ),
  );
}

class BankIdStartRejected implements Exception {
  final String reason, message;
  const BankIdStartRejected(this.reason, this.message);
}

class BankIdUnavailable implements Exception {
  const BankIdUnavailable();
}

class AttemptExpired implements Exception {
  const AttemptExpired();
}

String bankIdError(Object error) {
  if (error is BankIdStartRejected) return error.message;
  if (error is AttemptExpired) {
    return 'This BankID request has expired or the server restarted. Start a new request.';
  }
  if (error is BankIdUnavailable) {
    return 'BankID is not available yet. Please try later.';
  }
  if (error is DioException) {
    final data = error.response?.data;
    if (data is Map &&
        data['message'] is String &&
        error.response!.statusCode! < 500) {
      return data['message'] as String;
    }
    return 'The service could not be reached. Check your connection and check the request status before starting again.';
  }
  return 'The request could not be completed. Please try again.';
}
