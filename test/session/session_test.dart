import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/infrastructure/http_client.dart';
import 'package:research_steps_template/session/controller.dart';
import 'package:research_steps_template/session/gateway.dart';
import 'package:research_steps_template/session/models.dart';
import 'package:research_steps_template/session/store.dart';

class MemorySessionStore implements SessionStore {
  Map<String, dynamic>? value;
  Future<void>? pendingWrite;
  @override
  Future<Map<String, dynamic>?> read() async => value;
  @override
  Future<void> write(Map<String, dynamic> data) async {
    await pendingWrite;
    value = data;
  }

  @override
  Future<void> clear() async {
    value = null;
  }
}

class SessionApi implements SessionGateway {
  Participant participant = const Participant(id: 'u', gender: 'female');
  Future<Participant>? pending;
  Object? failure;
  final loggedOut = <String>[];
  @override
  Future<Participant> current(String token) async {
    if (failure != null) throw failure!;
    return pending ?? participant;
  }

  @override
  Future<void> logout(String token) async {
    loggedOut.add(token);
  }
}

Map<String, dynamic> grant({
  String token = 'token',
  String id = 'u',
  bool guardian = false,
  DateTime? expires,
}) => {
  'token': token,
  'expiresAt':
      (expires ?? DateTime.now().add(const Duration(hours: 1)))
          .millisecondsSinceEpoch ~/
      1000,
  'record': {
    'id': id,
    'gender': 'female',
    'consentRequired': false,
    'guardianRequired': guardian,
  },
};

void main() {
  late MemorySessionStore store;
  late SessionApi gateway;
  late SessionConnection connection;
  late SessionController controller;
  setUp(() {
    store = MemorySessionStore();
    gateway = SessionApi();
    connection = SessionConnection();
    controller = SessionController(
      gateway: gateway,
      store: store,
      connection: connection,
    );
  });
  tearDown(() => controller.dispose());
  test(
    'legacy session is verified before access; expired and invalid grants are cleared',
    () async {
      store.value = grant();
      final pending = Completer<Participant>();
      gateway.pending = pending.future;
      final restoring = controller.restore();
      await Future<void>.delayed(Duration.zero);
      expect(controller.state, isNull);
      pending.complete(const Participant(id: 'u', guardianRequired: true));
      await restoring;
      expect(controller.state!.participant.canParticipate, false);
      expect(connection.token, 'token');
      store.value = grant(
        expires: DateTime.now().subtract(const Duration(seconds: 1)),
      );
      await controller.restore();
      expect(controller.state, isNull);
      expect(store.value, isNull);
      store.value = {'invalid': true};
      await controller.restore();
      expect(store.value, isNull);
    },
  );
  test('late restore and refresh cannot overwrite a new participant', () async {
    store.value = grant();
    final pending = Completer<Participant>();
    gateway.pending = pending.future;
    final restoring = controller.restore();
    await Future<void>.delayed(Duration.zero);
    await controller.accept(grant(token: 'new', id: 'new-user'));
    pending.complete(const Participant(id: 'u'));
    await restoring;
    expect(controller.state!.participant.id, 'new-user');
    final refreshGate = Completer<Participant>();
    gateway.pending = refreshGate.future;
    final refreshing = controller.refresh();
    await controller.logout();
    await controller.accept(grant(token: 'third', id: 'third'));
    refreshGate.complete(
      const Participant(id: 'new-user', guardianRequired: true),
    );
    await refreshing;
    expect(controller.state!.token, 'third');
    expect(gateway.loggedOut, ['new']);
    await controller.handleAccessDenied('new', 'consentRequired');
    expect(controller.state!.token, 'third');
    await controller.handleAccessDenied('third', 'guardianRequired');
    expect(controller.state!.participant.canParticipate, false);
    await controller.handleAccessDenied('third', 'consentRequired');
    expect(controller.state, isNull);
    expect(connection.token, isEmpty);
  });
  test('logout during secure write leaves no persisted credentials', () async {
    final gate = Completer<void>();
    store.pendingWrite = gate.future;
    final accepting = controller.accept(grant());
    await Future<void>.delayed(Duration.zero);
    final loggingOut = controller.logout();
    gate.complete();
    await accepting;
    await loggingOut;
    expect(controller.state, isNull);
    expect(store.value, isNull);
  });
  test(
    'server verification rejects changed identity or revoked consent',
    () async {
      for (final participant in [
        const Participant(id: 'other'),
        const Participant(id: 'u', consentRequired: true),
      ]) {
        store.value = grant();
        gateway.participant = participant;
        await controller.restore();
        expect(controller.state, isNull);
        expect(store.value, isNull);
      }
    },
  );
  test('session expiry clears access and stored token', () async {
    await controller.accept(
      grant(expires: DateTime.now().add(const Duration(seconds: 2))),
    );
    await Future<void>.delayed(const Duration(seconds: 3));
    expect(controller.state, isNull);
    expect(store.value, isNull);
  });
}
