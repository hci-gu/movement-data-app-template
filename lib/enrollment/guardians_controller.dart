import 'dart:async';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../infrastructure/http_client.dart';
import '../session/controller.dart';
import 'bankid/gateway.dart';
import 'gateway.dart';

class GuardiansState {
  final GuardianStatus status;
  final bool busy;
  final String? error;
  const GuardiansState({
    this.status = const GuardianStatus(),
    this.busy = false,
    this.error,
  });
}

class GuardiansController extends StateNotifier<GuardiansState> {
  final EnrollmentGateway gateway;
  final bool Function() isCurrent;
  final Future<void> Function() refreshSession;
  GuardiansController({
    required this.gateway,
    required this.isCurrent,
    required this.refreshSession,
  }) : super(const GuardiansState());
  Future<void> _run(Future<GuardianStatus> Function() action) async {
    if (!mounted || !isCurrent() || state.busy) return;
    state = GuardiansState(status: state.status, busy: true);
    try {
      final status = await action();
      if (!mounted || !isCurrent()) return;
      state = GuardiansState(status: status, busy: true);
      if (status.eligible) await refreshSession();
      if (mounted && isCurrent()) state = GuardiansState(status: status);
    } catch (e) {
      if (mounted && isCurrent()) {
        state = GuardiansState(status: state.status, error: bankIdError(e));
      }
    }
  }

  Future<void> refresh() => _run(gateway.guardians);
  Future<void> create(String input, int count) async {
    final personal = input.replaceAll(RegExp(r'[^0-9]'), '');
    if (!mounted || !isCurrent() || state.busy) return;
    if (personal.length != 12 || (count != 1 && count != 2)) {
      state = GuardiansState(
        status: state.status,
        error: 'Enter a 12-digit personal number (YYYYMMDDXXXX).',
      );
      return;
    }
    await _run(() => gateway.createGuardian(personal, count));
  }
}

final enrollmentGatewayProvider = Provider<EnrollmentGateway>(
  (ref) => HttpEnrollmentGateway(
    ref.watch(httpClientProvider),
    ref.watch(sessionProvider.select((s) => s?.token)) ?? '',
  ),
);
final guardiansProvider =
    StateNotifierProvider.autoDispose<GuardiansController, GuardiansState>((
      ref,
    ) {
      final token = ref.watch(sessionProvider.select((s) => s?.token));
      final controller = GuardiansController(
        gateway: ref.watch(enrollmentGatewayProvider),
        isCurrent: () =>
            token != null && ref.read(sessionProvider)?.token == token,
        refreshSession: () => ref.read(sessionProvider.notifier).refresh(),
      );
      if (token != null) unawaited(controller.refresh());
      return controller;
    });
