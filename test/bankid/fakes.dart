import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/bankid/controller.dart';
import 'package:research_steps_template/bankid/gateway.dart';
import 'package:research_steps_template/bankid/models.dart';

const document = ConsentDocument(
  id: 'version1',
  version: '1',
  title: 'Consent',
  text: 'I agree to share step data.',
  documentHash: 'hash1',
);
const pending = BankIdOrder(
  id: 'order1',
  status: 'pending',
  hintCode: 'userSign',
  purpose: 'sign',
  mode: 'sameDevice',
  nonce: 'nonce1',
  launchUrl: 'https://app.bankid.com/?autostarttoken=token',
);
const accepted = BankIdOrder(
  id: 'order1',
  status: 'accepted',
  hintCode: '',
  purpose: 'sign',
  mode: 'sameDevice',
);
const cancelled = BankIdOrder(
  id: 'order1',
  status: 'cancelled',
  hintCode: 'userCancel',
  purpose: 'sign',
  mode: 'sameDevice',
);

class MemoryStore implements BankIdFlowStore {
  Map<String, dynamic>? value;
  @override
  Future<Map<String, dynamic>?> read() async => value;
  @override
  Future<void> write(Map<String, dynamic> next) async {
    value = Map.of(next);
  }

  @override
  Future<void> clear() async {
    value = null;
  }
}

class Gateway extends BankIdGateway {
  final MemoryStore store;
  Gateway(this.store);
  final starts = <BankIdFlow>[];
  BankIdOrder result = pending;
  bool loseStart = false, needsConsent = false;
  int completes = 0, returns = 0, abandons = 0;
  Completer<BankIdOrder>? delayedStatus;
  @override
  Future<ConsentDocument> currentConsent() async => document;
  @override
  Future<Map<String, dynamic>> createFlow(BankIdFlow flow) async {
    expect(
      store.value!['clientSecret'],
      flow.clientSecret,
      reason: 'Persist secret before sending enrollment',
    );
    return {'id': 'flow1', 'expiresAt': flow.expiresAt};
  }

  @override
  Future<BankIdOrder> start(BankIdFlow flow) async {
    expect(
      store.value!['requestKey'],
      flow.requestKey,
      reason: 'Persist idempotency key before starting BankID',
    );
    starts.add(flow);
    if (loseStart) throw StateError('lost response');
    return result;
  }

  @override
  Future<BankIdOrder> status(BankIdFlow flow) async =>
      delayedStatus == null ? result : delayedStatus!.future;
  @override
  Future<BankIdOrder> returned(BankIdFlow flow, String nonce) async {
    returns++;
    return result;
  }

  @override
  Future<BankIdOrder> cancel(BankIdFlow flow) async => cancelled;
  @override
  Future<void> abandon(BankIdFlow flow) async {
    abandons++;
  }

  @override
  Future<Map<String, dynamic>> complete(BankIdFlow flow) async {
    completes++;
    return needsConsent ? {'consentRequired': true} : {'token': 'server-grant'};
  }
}
