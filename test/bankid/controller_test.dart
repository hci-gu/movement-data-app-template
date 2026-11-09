import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/enrollment/bankid/controller.dart';
import 'package:research_steps_template/enrollment/bankid/gateway.dart';
import 'package:research_steps_template/enrollment/bankid/models.dart';
import 'fakes.dart';

void main() {
  late MemoryStore store;
  late Gateway gateway;
  late BankIdController controller;
  late List<Uri> launches;
  late List<Map<String, dynamic>> grants;
  BankIdController make() => BankIdController(
    gateway: gateway,
    store: store,
    launch: (uri) async {
      expect(store.value!['id'], isNotEmpty);
      launches.add(uri);
      return true;
    },
    onGrant: (value) async {
      grants.add(value);
    },
  );
  setUp(() {
    store = MemoryStore();
    gateway = Gateway(store);
    launches = [];
    grants = [];
    controller = make();
  });
  tearDown(() => controller.dispose());
  test(
    'authentication starts without consent; return links only poll owned attempts',
    () async {
      await controller.initialize();
      await controller.start('sameDevice');
      expect(gateway.starts.single.keys.toSet(), {'clientSecret', 'mode'});
      expect(launches, hasLength(1));
      final before = gateway.statuses;
      await controller.handleReturn(
        Uri.parse('https://untrusted.test/bankid/return'),
      );
      expect(gateway.statuses, before);
      await controller.handleReturn(
        Uri.parse('researchsteps://bankid/return#anything=untrusted'),
      );
      expect(gateway.statuses, before + 1);
      expect(grants, isEmpty);
      gateway.result = accepted;
      await controller.refresh();
      expect(grants, isEmpty);
      await controller.finish();
      expect(grants.single['token'], 'server-grant');
      expect(store.value, isNull);
    },
  );
  test(
    'lost start response restores by credential without starting twice',
    () async {
      gateway.loseStart = true;
      await controller.start('sameDevice');
      expect(gateway.starts, hasLength(1));
      expect(store.value!.keys.toSet(), {'id', 'secret', 'expiresAt'});
      controller.dispose();
      controller = make();
      await controller.restore();
      expect(controller.state.attempt!.id, 'order1');
      expect(gateway.starts, hasLength(1));
      expect(launches, isEmpty);
    },
  );
  test('backend restart discards attempt and allows retry', () async {
    await controller.start('sameDevice');
    gateway.expired = true;
    await controller.refresh();
    expect(controller.state.attempt, isNull);
    expect(store.value, isNull);
    expect(grants, isEmpty);
    expect(controller.state.error, contains('server restarted'));
  });
  test('accepted authentication can resume after app restart', () async {
    gateway.result = accepted;
    await controller.start('sameDevice');
    controller.dispose();
    controller = make();
    await controller.restore();
    expect(controller.state.attempt!.accepted, true);
    await controller.finish();
    expect(grants, hasLength(1));
  });
  test('late status cannot overwrite local cancellation', () async {
    await controller.start('sameDevice');
    gateway.delayedStatus = Completer<BankIdAttempt>();
    final poll = controller.refresh();
    final cancel = controller.cancel();
    gateway.delayedStatus!.complete(pending);
    await poll;
    await cancel;
    expect(controller.state.attempt!.status, 'cancelled');
    expect(grants, isEmpty);
  });
  test('stopped request can be replaced with a new credential', () async {
    await controller.start('sameDevice');
    await controller.cancel();
    await controller.restart();
    expect(store.value, isNull);
    await controller.start('sameDevice');
    expect(gateway.starts, hasLength(2));
    expect(
      gateway.starts.first['clientSecret'],
      isNot(gateway.starts.last['clientSecret']),
    );
  });
  test('expired credential is cleared without server access', () async {
    store.value = {'id': 'old', 'secret': 'secret', 'expiresAt': 1};
    await controller.restore();
    expect(store.value, isNull);
    expect(gateway.statuses, 0);
    expect(grants, isEmpty);
  });
  test('rejected initiation clears fictitious attempt', () async {
    gateway.startError = const BankIdStartRejected(
      'authenticationUnavailable',
      'BankID unavailable.',
    );
    await controller.start('sameDevice');
    expect(store.value, isNull);
    expect(controller.state.attempt, isNull);
    expect(controller.state.error, 'BankID unavailable.');
  });
}
