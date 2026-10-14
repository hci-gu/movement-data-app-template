import 'dart:async';
import 'package:dio/dio.dart';
import 'package:health/health.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../app/environment.dart';
import '../infrastructure/http_client.dart';
import '../infrastructure/local_storage.dart';
import '../session/controller.dart';
import 'health_reader.dart';
import 'summary.dart';
import 'upload_gateway.dart';
import 'upload_store.dart';

enum StepsPhase {
  restoring,
  idle,
  reading,
  denied,
  empty,
  ready,
  uploading,
  complete,
  error,
}

class StepsState {
  final StepsPhase phase;
  final List<HealthDataPoint> samples;
  final DateTime? completedAt;
  final String? error;
  final double progress;
  const StepsState({
    this.phase = StepsPhase.restoring,
    this.samples = const [],
    this.completedAt,
    this.error,
    this.progress = 0,
  });
  bool get completed => completedAt != null;
  bool get busy =>
      phase == StepsPhase.restoring ||
      phase == StepsPhase.reading ||
      phase == StepsPhase.uploading;
  StepImportSummary get summary => StepImportSummary.fromHealthData(samples);
}

class StepsController extends StateNotifier<StepsState> {
  final HealthReader reader;
  final UploadGateway gateway;
  final UploadStore store;
  final bool Function() isCurrent;
  final DateTime Function() now;
  CancelToken? _upload;
  StepsController({
    required this.reader,
    required this.gateway,
    required this.store,
    required this.isCurrent,
    DateTime Function()? now,
  }) : now = now ?? DateTime.now,
       super(const StepsState());
  bool get active => mounted && isCurrent();

  Future<void> restore() async {
    try {
      final completed = await store.read();
      if (active) {
        state = StepsState(
          phase: completed == null ? StepsPhase.idle : StepsPhase.complete,
          completedAt: completed,
        );
      }
    } catch (_) {
      if (active) {
        state = const StepsState(
          phase: StepsPhase.error,
          error: 'Could not restore upload status. Try again.',
        );
      }
    }
  }

  Future<void> read() async {
    if (!active || state.busy) return;
    state = const StepsState(phase: StepsPhase.reading);
    try {
      final result = await reader.read();
      if (!active) return;
      state = StepsState(
        phase: !result.authorized
            ? StepsPhase.denied
            : result.samples.isEmpty
            ? StepsPhase.empty
            : StepsPhase.ready,
        samples: List.unmodifiable(result.samples),
      );
    } catch (_) {
      if (active) {
        state = const StepsState(
          phase: StepsPhase.error,
          error: 'Step data could not be read. Please try again.',
        );
      }
    }
  }

  Future<void> upload() async {
    if (!active || state.busy || state.samples.isEmpty || state.completed) {
      return;
    }
    final samples = state.samples;
    final cancellation = _upload = CancelToken();
    state = StepsState(phase: StepsPhase.uploading, samples: samples);
    try {
      await gateway.upload(samples, cancellation, (progress) {
        if (active) {
          state = StepsState(
            phase: StepsPhase.uploading,
            samples: samples,
            progress: progress,
          );
        }
      });
      if (!active) return;
      final completed = now();
      await store.write(completed, () => active);
      if (active) {
        state = StepsState(
          phase: StepsPhase.complete,
          samples: samples,
          completedAt: completed,
          progress: 1,
        );
      }
    } catch (_) {
      if (active) {
        state = StepsState(
          phase: StepsPhase.error,
          samples: samples,
          error:
              'Upload could not be confirmed. Your checklist is not complete. Please retry.',
        );
      }
    } finally {
      if (identical(_upload, cancellation)) _upload = null;
    }
  }

  @override
  void dispose() {
    _upload?.cancel('Participant session ended');
    super.dispose();
  }
}

final healthReaderProvider = Provider<HealthReader>(
  (ref) => NativeHealthReader(),
);
final stepsProvider = StateNotifierProvider<StepsController, StepsState>((ref) {
  final scope = ref.watch(
    sessionProvider.select(
      (s) => (
        token: s?.token,
        id: s?.participant.id,
        allowed: s?.participant.canParticipate == true,
      ),
    ),
  );
  final controller = StepsController(
    reader: ref.watch(healthReaderProvider),
    gateway: HttpUploadGateway(
      ref.watch(httpClientProvider),
      scope.id ?? '',
      scope.token ?? '',
    ),
    store: PreferencesUploadStore(
      ref.watch(preferencesProvider),
      Environment.apiBaseUrl,
      scope.id ?? '',
    ),
    isCurrent: () {
      final current = ref.read(sessionProvider);
      return scope.allowed &&
          current?.token == scope.token &&
          current?.participant.canParticipate == true;
    },
  );
  if (scope.allowed) unawaited(controller.restore());
  return controller;
});
