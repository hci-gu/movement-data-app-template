import 'dart:async';
import 'package:dio/dio.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../app/environment.dart';

// Transport has one credential snapshot. Only the session controller writes it.
class SessionConnection {
  String token = '';
  Future<void> Function(String token, String reason)? onAccessDenied;
}

final sessionConnectionProvider = Provider((ref) => SessionConnection());

Dio createHttpClient(String baseUrl, SessionConnection session) {
  final client = Dio(
    BaseOptions(
      baseUrl: baseUrl,
      headers: {'Content-Type': 'application/json'},
      connectTimeout: const Duration(seconds: 15),
      receiveTimeout: const Duration(seconds: 30),
      sendTimeout: const Duration(seconds: 60),
    ),
  );
  client.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) {
        if (options.extra['public'] != true && session.token.isNotEmpty) {
          // Feature gateways may pin a request to the session that started it.
          options.headers.putIfAbsent(
            'Authorization',
            () => 'Bearer ${session.token}',
          );
        }
        handler.next(options);
      },
      onError: (error, handler) {
        final token = session.token;
        if (token.isNotEmpty &&
            error.requestOptions.extra['public'] != true &&
            error.requestOptions.headers['Authorization'] == 'Bearer $token') {
          var reason = error.response?.statusCode == 401
              ? 'sessionExpired'
              : '';
          if (error.response?.statusCode == 403) {
            final body = error.response?.data;
            final data = body is Map ? body['data'] : null;
            final detail = data is Map ? data['reason'] : null;
            if (detail is Map && detail['code'] is String) {
              reason = detail['code'];
            }
          }
          if (reason.isNotEmpty) {
            final callback = session.onAccessDenied;
            if (callback != null) unawaited(callback(token, reason));
          }
        }
        handler.next(error);
      },
    ),
  );
  return client;
}

final httpClientProvider = Provider<Dio>((ref) {
  final client = createHttpClient(
    Environment.apiBaseUrl,
    ref.watch(sessionConnectionProvider),
  );
  ref.onDispose(() => client.close(force: true));
  return client;
});
