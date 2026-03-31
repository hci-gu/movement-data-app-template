import 'dart:convert';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:research_steps_template/state/health.dart';

class Api {
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
    String participantId,
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
        'participantId': participantId,
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

  Future<void> registerParticipant(
    String participantId,
    String password,
    bool consentAccepted,
  ) async {
    await api.post(
      '/users',
      data: jsonEncode({
        'participantId': participantId,
        'password': password,
        'consentAccepted': consentAccepted,
      }),
    );
  }

  Future<void> uploadMetadata(
    String participantId,
    Map<String, dynamic> metadata,
  ) async {
    await api.post(
      '/info',
      options: Options(
        headers: {'Content-Type': 'application/json; charset=UTF-8'},
      ),
      data: jsonEncode({'participantId': participantId, 'data': metadata}),
    );
  }

  Future<void> uploadData(
    String participantId,
    List<HealthDataPoint> data,
  ) async {
    if (data.isEmpty) {
      return;
    }

    await uploadDataInChunks(participantId, data);
  }

  static final Api _instance = Api._internal();

  factory Api() {
    return _instance;
  }

  Api._internal();
}
