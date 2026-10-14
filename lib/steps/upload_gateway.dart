import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:health/health.dart';

abstract class UploadGateway {
  Future<void> upload(
    List<HealthDataPoint> samples,
    CancelToken cancellation,
    void Function(double) onProgress,
  );
}

class HttpUploadGateway implements UploadGateway {
  final Dio client;
  final String participantId, token;
  HttpUploadGateway(this.client, this.participantId, this.token);
  @override
  Future<void> upload(
    List<HealthDataPoint> samples,
    CancelToken cancellation,
    void Function(double) onProgress,
  ) async {
    if (samples.isEmpty) throw StateError('No step data to upload.');
    final size = (samples.length / 10).ceil();
    final count = (samples.length / size).ceil();
    for (
      var index = 0, chunk = 0;
      index < samples.length;
      index += size, chunk++
    ) {
      if (cancellation.isCancelled) throw cancellation.cancelError!;
      await client.post(
        '/data',
        cancelToken: cancellation,
        options: Options(
          headers: {
            'Authorization': 'Bearer $token',
            'Content-Encoding': 'gzip',
            'Content-Type': 'application/json; charset=UTF-8',
          },
        ),
        data: Uint8List.fromList(
          gzip.encode(
            utf8.encode(
              jsonEncode({
                'userId': participantId,
                'chunkIndex': chunk,
                'data': samples
                    .sublist(index, min(index + size, samples.length))
                    .map((p) => p.toJson())
                    .toList(),
              }),
            ),
          ),
        ),
      );
      if (cancellation.isCancelled) throw cancellation.cancelError!;
      onProgress((chunk + 1) / count);
    }
  }
}
