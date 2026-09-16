import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/bankid/controller.dart';
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
      expect(store.value!['nonce'], 'nonce1');
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

  test(
    'review is mandatory; callback and pending order never grant access',
    () async {
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
      expect(gateway.completes, 0);
      gateway.result = accepted;
      await controller.refresh();
      expect(
        grants,
        isEmpty,
        reason: 'Participant sees signed receipt before continuing',
      );
      await controller.finish();
      expect(grants.single['token'], 'server-grant');
      expect(store.value, isNull);
    },
  );

  test(
    'cold restart retries exact saved request after a lost start response',
    () async {
      await controller.begin(invitationCode: 'invite');
      controller.setReviewed(true);
      gateway.loseStart = true;
      await controller.start('sameDevice');
      final first = gateway.starts.single;
      expect(first.document!.documentHash, document.documentHash);
      expect(controller.state.error, isNotNull);
      controller.dispose();
      controller = make();
      gateway.loseStart = false;
      await controller.restore();
      expect(gateway.starts.last.requestKey, first.requestKey);
      expect(gateway.starts.last.clientSecret, first.clientSecret);
      expect(controller.state.order!.id, 'order1');
      expect(
        launches,
        isEmpty,
        reason: 'Restoration must not auto launch BankID',
      );
    },
  );

  test(
    'returning login requiring new consent displays review before signing',
    () async {
      await controller.begin(returning: true);
      gateway.result = const BankIdOrder(
        id: 'auth1',
        status: 'accepted',
        hintCode: '',
        purpose: 'auth',
        mode: 'qr',
      );
      gateway.needsConsent = true;
      await controller.start('qr');
      expect(controller.state.document!.id, document.id);
      expect(controller.state.reviewed, false);
      expect(controller.state.order, isNull);
      expect(grants, isEmpty);
      await controller.start('qr');
      expect(gateway.starts, hasLength(1));
      gateway.needsConsent = false;
      gateway.result = accepted;
      controller.setReviewed(true);
      await controller.start('qr');
      expect(gateway.starts.last.document!.id, document.id);
      await controller.finish();
      expect(grants, hasLength(1));
    },
  );

  test('late status response cannot overwrite a cancellation', () async {
    await controller.begin(invitationCode: 'invite');
    controller.setReviewed(true);
    await controller.start('qr');
    gateway.delayedStatus = Completer<BankIdOrder>();
    final polling = controller.refresh();
    final cancellation = controller.cancel();
    gateway.delayedStatus!.complete(pending);
    await polling;
    await cancellation;
    expect(controller.state.order!.status, 'cancelled');
  });

  test(
    'start over releases invitation only after backend acknowledgement',
    () async {
      await controller.begin(invitationCode: 'invite');
      await controller.reset();
      expect(gateway.abandons, 1);
      expect(store.value, isNull);
      expect(controller.state.flow, isNull);
    },
  );

  test('expired stored flow is discarded without an order or grant', () async {
    store.value = const BankIdFlow(
      id: 'old',
      clientSecret: 'secret',
      kind: 'enroll',
      expiresAt: 1,
    ).toJson();
    await controller.restore();
    expect(controller.state.flow, isNull);
    expect(store.value, isNull);
    expect(gateway.starts, isEmpty);
    expect(grants, isEmpty);
  });
}
