import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/enrollment/consent_controller.dart';
import 'package:research_steps_template/enrollment/gateway.dart';
import 'package:research_steps_template/enrollment/guardians_controller.dart';
import '../bankid/fakes.dart' show document;

class EnrollmentApi implements EnrollmentGateway {
  GuardianStatus status = const GuardianStatus();
  Future<GuardianStatus>? pending;
  Object? failure;
  @override
  Future<GuardianStatus> guardians() async => pending ?? status;
  @override
  Future<GuardianStatus> createGuardian(
    String personalNumber,
    int count,
  ) async => status;
  @override
  Future<ConsentReceipt> receipt() async =>
      ConsentReceipt(document, DateTime(2026), 'accepted');
  @override
  Future<void> withdraw() async {
    if (failure != null) throw failure!;
  }
}

void main() {
  test(
    'guardian approval refreshes verified session; stale results cannot change access',
    () async {
      var active = true, refreshes = 0;
      final api = EnrollmentApi();
      final c = GuardiansController(
        gateway: api,
        isCurrent: () => active,
        refreshSession: () async {
          refreshes++;
        },
      );
      addTearDown(c.dispose);
      await c.refresh();
      expect(refreshes, 0);
      api.status = const GuardianStatus(eligible: true);
      await c.refresh();
      expect(refreshes, 1);
      final gate = Completer<GuardianStatus>();
      api.pending = gate.future;
      final refreshing = c.refresh();
      active = false;
      gate.complete(const GuardianStatus(eligible: true));
      await refreshing;
      expect(refreshes, 1);
    },
  );
  test(
    'only confirmed withdrawal ends the current session; failures remain retryable',
    () async {
      var invalidations = 0;
      final api = EnrollmentApi();
      final c = ConsentController(
        gateway: api,
        isCurrent: () => true,
        onWithdrawn: () async {
          invalidations++;
        },
      );
      addTearDown(c.dispose);
      await c.load();
      api.failure = Exception('offline');
      await c.withdraw();
      expect(invalidations, 0);
      expect(c.state.error, isNotNull);
      api.failure = null;
      await c.withdraw();
      expect(invalidations, 1);
    },
  );
}
