import 'dart:async';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../session/controller.dart';
import 'gateway.dart';
import 'guardians_controller.dart';

class ReceiptState {
  final ConsentReceipt? receipt;
  final bool loading, withdrawing;
  final String? error;
  const ReceiptState({
    this.receipt,
    this.loading = false,
    this.withdrawing = false,
    this.error,
  });
}

class ConsentController extends StateNotifier<ReceiptState> {
  final EnrollmentGateway gateway;
  final bool Function() isCurrent;
  final Future<void> Function() onWithdrawn;
  ConsentController({
    required this.gateway,
    required this.isCurrent,
    required this.onWithdrawn,
  }) : super(const ReceiptState());
  Future<void> load() async {
    if (!mounted || !isCurrent() || state.loading) return;
    state = const ReceiptState(loading: true);
    try {
      final receipt = await gateway.receipt();
      if (mounted && isCurrent()) state = ReceiptState(receipt: receipt);
    } catch (_) {
      if (mounted && isCurrent()) {
        state = const ReceiptState(
          error: 'The consent receipt could not be loaded.',
        );
      }
    }
  }

  Future<void> withdraw() async {
    if (!mounted ||
        !isCurrent() ||
        state.withdrawing ||
        state.receipt == null) {
      return;
    }
    state = ReceiptState(receipt: state.receipt, withdrawing: true);
    try {
      await gateway.withdraw();
      if (mounted && isCurrent()) await onWithdrawn();
    } catch (_) {
      if (mounted && isCurrent()) {
        state = ReceiptState(
          receipt: state.receipt,
          error:
              'Withdrawal could not be confirmed. Please retry or contact the study team.',
        );
      }
    }
  }
}

final consentProvider =
    StateNotifierProvider.autoDispose<ConsentController, ReceiptState>((ref) {
      final token = ref.watch(sessionProvider.select((s) => s?.token));
      final controller = ConsentController(
        gateway: ref.watch(enrollmentGatewayProvider),
        isCurrent: () =>
            token != null && ref.read(sessionProvider)?.token == token,
        onWithdrawn: () => ref.read(sessionProvider.notifier).invalidate(),
      );
      if (token != null) unawaited(controller.load());
      return controller;
    });
