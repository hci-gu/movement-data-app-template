import 'dart:convert';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:research_steps_template/pocketbase.dart';
import 'package:research_steps_template/state/health.dart';

class Api {
  Future<void> Function()? onSessionInvalid;
  final Dio api = Dio(
    BaseOptions(
      headers: {'Content-Type': 'application/json'},
      connectTimeout: const Duration(seconds: 15),
      receiveTimeout: const Duration(seconds: 30),
      sendTimeout: const Duration(seconds: 60),
    ),
  );

  void init(String baseUrl) {
    api.options.baseUrl = baseUrl;
  }

  Future<void> uploadDataInChunks(
    String userId,
    List<HealthDataPoint> data,
  ) async {
    final chunks = <Map<String, dynamic>>[];
    final chunkSize = (data.length / 10).ceil();
    var chunkIndex = 0;

    for (var index = 0; index < data.length; index += chunkSize) {
      var endIndex = index + chunkSize;
      if (endIndex > data.length) {
        endIndex = data.length;
      }

      chunks.add({
        'userId': userId,
        'chunkIndex': chunkIndex,
        'data': data
            .sublist(index, endIndex)
            .map((entry) => entry.toJson())
            .toList(),
      });
      chunkIndex++;
    }

    for (final chunk in chunks) {
      await api.post(
        '/data',
        options: Options(
          headers: {
            'Content-Encoding': 'gzip',
            'Content-Type': 'application/json; charset=UTF-8',
          },
        ),
        data: gzip.encode(utf8.encode(jsonEncode(chunk))),
      );
    }
  }

  Future<Map<String, dynamic>> currentParticipant() async =>
      Map<String, dynamic>.from((await api.get('/api/me')).data as Map);

  Future<Map<String, dynamic>> guardianStatus() async =>
      Map<String, dynamic>.from((await api.get('/api/guardians')).data as Map);

  Future<Map<String, dynamic>> createGuardianRequest(
    String personalNumber,
    int guardianCount,
  ) async => Map<String, dynamic>.from(
    (await api.post(
          '/api/guardians',
          data: {
            'personalNumber': personalNumber,
            'guardianCount': guardianCount,
          },
        )).data
        as Map,
  );
  Future<void> logout() async {
    await api.post('/api/logout');
  }

  Future<void> withdrawConsent() async {
    await api.post('/api/consent/withdraw');
  }

  Future<Map<String, dynamic>> consentReceipt() async =>
      Map<String, dynamic>.from(
        (await api.get('/api/consent/receipt')).data as Map,
      );

  Future<void> uploadData(String userId, List<HealthDataPoint> data) async {
    if (data.isEmpty) {
      return;
    }

    await uploadDataInChunks(userId, data);
  }

  static final Api _instance = Api._internal();

  factory Api() {
    return _instance;
  }

  Api._internal() {
    api.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          if (options.extra['public'] != true &&
              pb.authStore.token.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer ${pb.authStore.token}';
          }
          handler.next(options);
        },
        onError: (error, handler) {
          if (error.requestOptions.extra['public'] != true &&
              error.response?.statusCode == 401) {
            onSessionInvalid?.call();
          }
          handler.next(error);
        },
      ),
    );
  }
}
