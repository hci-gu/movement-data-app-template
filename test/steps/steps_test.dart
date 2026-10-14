import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:health/health.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:research_steps_template/steps/controller.dart';
import 'package:research_steps_template/steps/health_reader.dart';
import 'package:research_steps_template/steps/summary.dart';
import 'package:research_steps_template/steps/upload_gateway.dart';
import 'package:research_steps_template/steps/upload_store.dart';
import '../questionnaires/gateway_test.dart' show Transport, body;

HealthDataPoint sample([int day = 1]) => HealthDataPoint(
  NumericHealthValue(100),
  HealthDataType.STEPS,
  HealthDataUnit.COUNT,
  DateTime(2026, 1, day),
  DateTime(2026, 1, day, 1),
  PlatformType.IOS,
  'device',
  'source',
  'Watch',
);

class Reader implements HealthReader {
  HealthRead result = HealthRead(true, [sample()]);
  Object? failure;
  Future<HealthRead>? pending;
  int reads = 0;
  @override
  Future<HealthRead> read() async {
    reads++;
    if (failure != null) throw failure!;
    return pending ?? result;
  }
}

class Uploader implements UploadGateway {
  Object? failure;
  Future<void>? pending;
  CancelToken? cancellation;
  int uploads = 0;
  @override
  Future<void> upload(
    List<HealthDataPoint> samples,
    CancelToken token,
    void Function(double) progress,
  ) async {
    uploads++;
    cancellation = token;
    progress(.5);
    await pending;
    if (failure != null) throw failure!;
    progress(1);
  }
}

class Marker implements UploadStore {
  DateTime? value;
  int writes = 0;
  @override
  Future<DateTime?> read() async => value;
  @override
  Future<void> write(DateTime time, bool Function() current) async {
    if (current()) {
      writes++;
      value = time;
    }
  }
}

void main() {
  late Reader reader;
  late Uploader gateway;
  late Marker store;
  late StepsController controller;
  late bool active;
  setUp(() {
    reader = Reader();
    gateway = Uploader();
    store = Marker();
    active = true;
    controller = StepsController(
      reader: reader,
      gateway: gateway,
      store: store,
      isCurrent: () => active,
    );
  });
  tearDown(() {
    if (controller.mounted) controller.dispose();
  });
  test(
    'restoring never reads health; denied, empty, and failed reads remain incomplete',
    () async {
      await controller.restore();
      expect(reader.reads, 0);
      reader.result = const HealthRead(false, []);
      await controller.read();
      expect(controller.state.phase, StepsPhase.denied);
      reader.result = const HealthRead(true, []);
      await controller.read();
      expect(controller.state.phase, StepsPhase.empty);
      reader.failure = Exception('native error');
      await controller.read();
      expect(controller.state.phase, StepsPhase.error);
      expect(controller.state.completed, false);
      expect(store.writes, 0);
      expect(gateway.uploads, 0);
    },
  );
  test(
    'partial failure retains samples for retry and only full success persists completion',
    () async {
      await controller.restore();
      await controller.read();
      gateway.failure = Exception('chunk failed');
      await controller.upload();
      expect(controller.state.completed, false);
      expect(store.writes, 0);
      expect(controller.state.samples.length, 1);
      gateway.failure = null;
      final gate = Completer<void>();
      gateway.pending = gate.future;
      final uploading = controller.upload();
      await controller.upload();
      expect(gateway.uploads, 2);
      expect(store.writes, 0);
      gate.complete();
      await uploading;
      expect(controller.state.completed, true);
      expect(store.writes, 1);
      await controller.restore();
      expect(controller.state.completed, true);
      expect(reader.reads, 1);
    },
  );
  test(
    'late reads and uploads cannot complete an ended session; disposal cancels upload',
    () async {
      await controller.restore();
      final readGate = Completer<HealthRead>();
      reader.pending = readGate.future;
      final reading = controller.read();
      active = false;
      readGate.complete(HealthRead(true, [sample()]));
      await reading;
      expect(controller.state.samples, isEmpty);
      controller.dispose();
      active = true;
      controller = StepsController(
        reader: Reader(),
        gateway: gateway,
        store: store,
        isCurrent: () => active,
      );
      await controller.restore();
      await controller.read();
      final gate = Completer<void>();
      gateway.pending = gate.future;
      final uploading = controller.upload();
      controller.dispose();
      expect(gateway.cancellation!.isCancelled, true);
      gate.complete();
      await uploading;
      expect(store.writes, 0);
    },
  );
  test(
    'completion keys isolate backend and participant and retire ambiguous legacy flags',
    () async {
      SharedPreferences.setMockInitialValues({
        'userId': 'u',
        'hasUploadedData': true,
        'lastUploadAt': '2026-01-01',
      });
      final prefs = await SharedPreferences.getInstance();
      final a = PreferencesUploadStore(prefs, 'https://a', 'u');
      expect(await a.read(), isNull);
      expect(prefs.containsKey('hasUploadedData'), false);
      final now = DateTime(2026, 10, 14);
      await a.write(now, () => true);
      expect(await a.read(), now);
      expect(
        await PreferencesUploadStore(prefs, 'https://b', 'u').read(),
        isNull,
      );
      expect(
        await PreferencesUploadStore(prefs, 'https://a', 'v').read(),
        isNull,
      );
      await a.write(DateTime(2027), () => false);
      expect(await a.read(), now);
    },
  );
  test(
    'gzip chunks preserve raw samples and order; failure stops later chunks',
    () async {
      final chunks = <Map<String, dynamic>>[];
      final wireChunks = <Map<String, dynamic>>[];
      var fail = true;
      final client = Dio(BaseOptions(baseUrl: 'http://example.test'))
        ..httpClientAdapter = Transport(
          (r) async {
            expect(r.headers['Authorization'], 'Bearer old-token');
            expect(r.headers['Content-Encoding'], 'gzip');
            chunks.add(
              jsonDecode(utf8.decode(gzip.decode(r.data as List<int>)))
                  as Map<String, dynamic>,
            );
            return body({}, fail && chunks.length == 2 ? 500 : 200);
          },
          consume: (stream) async {
            final bytes = await stream!.expand((chunk) => chunk).toList();
            wireChunks.add(
              jsonDecode(utf8.decode(gzip.decode(bytes)))
                  as Map<String, dynamic>,
            );
          },
        );
      final upload = HttpUploadGateway(client, 'u', 'old-token');
      final samples = List.generate(21, (i) => sample(i + 1));
      await expectLater(
        upload.upload(samples, CancelToken(), (_) {}),
        throwsA(isA<DioException>()),
      );
      expect(chunks.length, 2);
      expect(wireChunks, chunks);
      chunks.clear();
      wireChunks.clear();
      fail = false;
      final progress = <double>[];
      await upload.upload(samples, CancelToken(), progress.add);
      expect(chunks.length, 7);
      expect(wireChunks, chunks);
      expect(chunks.map((c) => c['chunkIndex']), List.generate(7, (i) => i));
      expect(chunks.every((c) => c['userId'] == 'u'), true);
      expect(
        chunks.expand((c) => c['data'] as List),
        samples.map((s) => s.toJson()),
      );
      expect(progress.last, 1);
      final summary = StepImportSummary.fromHealthData([
        sample(),
        sample(),
        sample(2),
      ]);
      expect(summary.daysWithData, 2);
      expect(summary.totalSteps, 300);
    },
  );
}
