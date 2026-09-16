import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/bankid/controller.dart';
import 'package:research_steps_template/bankid/gateway.dart';
import 'package:research_steps_template/bankid/models.dart';
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
    onGrant: (grant) async {
      grants.add(grant);
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
  test('review gates signing and callback never grants access', () async {
    await controller.restore();
    await controller.begin(invitationCode: 'invite');
    await controller.start('sameDevice');
    expect(gateway.starts, isEmpty);
    controller.setReviewed(true);
    await controller.start('sameDevice');
    expect(launches, hasLength(1));
    await controller.handleReturn(
      Uri.parse('researchsteps://bankid/return#nonce=wrong'),
    );
    expect(gateway.returns, 0);
    await controller.handleReturn(
      Uri.parse('researchsteps://bankid/return#nonce=nonce1'),
    );
    expect(gateway.returns, 1);
    expect(grants, isEmpty);
    gateway.result = accepted;
    await controller.refresh();
    expect(grants, isEmpty, reason: 'Show signing receipt before continuing');
    await controller.finish();
    expect(grants.single['token'], 'server-grant');
    expect(store.value, isNull);
  });
  test(
    'app restart recovers lost start response by status without another start',
    () async {
      await controller.begin(invitationCode: 'invite');
      controller.setReviewed(true);
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
  test('backend restart discards attempt and offers clean start', () async {
    await controller.begin(invitationCode: 'invite');
    controller.setReviewed(true);
    await controller.start('qr');
    gateway.expired = true;
    await controller.refresh();
    expect(controller.state.begun, false);
    expect(controller.state.attempt, isNull);
    expect(store.value, isNull);
    expect(controller.state.error, contains('server restarted'));
    expect(grants, isEmpty);
  });
  test(
    'returning login requiring consent must review before signing',
    () async {
      await controller.begin(returning: true);
      gateway.result = const BankIdAttempt(
        id: 'auth1',
        secret: 'secret',
        expiresAt: 4102444800,
        status: 'accepted',
        purpose: 'auth',
        mode: 'qr',
        consentRequired: true,
      );
      await controller.start('qr');
      expect(controller.state.document!.id, document.id);
      expect(controller.state.reviewed, false);
      expect(controller.state.attempt, isNull);
      expect(grants, isEmpty);
      await controller.start('qr');
      expect(gateway.starts, hasLength(1));
      controller.setReviewed(true);
      gateway.result = accepted;
      await controller.start('qr');
      expect(gateway.starts.last['authAttempt'], 'auth1.secret');
      expect(gateway.starts.last['consentTextId'], document.id);
      await controller.finish();
      expect(grants, hasLength(1));
    },
  );
  test(
    'review can resume after app restart from accepted authentication credential',
    () async {
      gateway.result = const BankIdAttempt(
        id: 'auth1',
        secret: 'secret',
        expiresAt: 4102444800,
        status: 'accepted',
        purpose: 'auth',
        consentRequired: true,
      );
      await controller.begin(returning: true);
      await controller.start('sameDevice');
      controller.dispose();
      controller = make();
      await controller.restore();
      expect(controller.state.document!.id, document.id);
      expect(controller.state.reviewed, false);
      expect(controller.state.attempt, isNull);
    },
  );
  test('late status cannot overwrite cancellation', () async {
    await controller.begin(invitationCode: 'invite');
    controller.setReviewed(true);
    await controller.start('qr');
    gateway.delayedStatus = Completer<BankIdAttempt>();
    final polling = controller.refresh();
    final cancellation = controller.cancel();
    gateway.delayedStatus!.complete(pending);
    await polling;
    await cancellation;
    expect(controller.state.attempt!.status, 'cancelled');
  });
  test('start over cancels live attempt and clears storage', () async {
    await controller.begin(invitationCode: 'invite');
    controller.setReviewed(true);
    await controller.start('qr');
    await controller.reset();
    expect(gateway.cancels, 1);
    expect(store.value, isNull);
    expect(controller.state.begun, false);
  });
  test('expired credential cannot restore an attempt', () async {
    store.value = {'id': 'old', 'secret': 'secret', 'expiresAt': 1};
    await controller.restore();
    expect(store.value, isNull);
    expect(gateway.statuses, 0);
    expect(grants, isEmpty);
  });
  test(
    'QR renewal creates one new attempt after cancelling the old one',
    () async {
      await controller.begin(invitationCode: 'invite');
      controller.setReviewed(true);
      gateway.result = const BankIdAttempt(
        id: 'qr1',
        secret: 'secret',
        expiresAt: 4102444800,
        mode: 'qr',
        qrSecondsRemaining: 0,
      );
      await controller.start('qr');
      await controller.extendQR();
      expect(gateway.cancels, 1);
      expect(gateway.starts, hasLength(2));
      expect(
        gateway.starts.first['clientSecret'],
        isNot(gateway.starts.last['clientSecret']),
      );
    },
  );
  test('rejected initiation leaves no fictitious pending attempt', () async {
    await controller.begin(invitationCode: 'invite');
    controller.setReviewed(true);
    gateway.startError = const BankIdStartRejected(
      'consentChanged',
      'Review the new consent.',
    );
    await controller.start('sameDevice');
    expect(controller.state.attempt, isNull);
    expect(controller.state.reviewed, false);
    expect(store.value, isNull);
    expect(controller.state.error, 'Review the new consent.');
  });
}
