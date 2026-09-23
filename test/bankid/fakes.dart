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
const pending = BankIdAttempt(
  id: 'order1',
  secret: 'secret',
  expiresAt: 4102444800,
  status: 'pending',
  hintCode: 'userSign',
  nonce: 'nonce1',
  launchUrl: 'https://app.bankid.com/?autostarttoken=token',
);
const accepted = BankIdAttempt(
  id: 'order1',
  secret: 'secret',
  expiresAt: 4102444800,
  status: 'accepted',
  document: document,
  grant: {'token': 'server-grant'},
);
const cancelled = BankIdAttempt(
  id: 'order1',
  secret: 'secret',
  expiresAt: 4102444800,
  status: 'cancelled',
  hintCode: 'userCancel',
);

class MemoryStore implements BankIdAttemptStore {
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
  final starts = <Map<String, dynamic>>[];
  BankIdAttempt result = pending;
  bool loseStart = false, expired = false;
  Object? startError;
  int returns = 0, cancels = 0, statuses = 0;
  Completer<BankIdAttempt>? delayedStatus;
  @override
  Future<ConsentDocument> currentConsent() async => document;
  @override
  Future<BankIdAttempt> start(
    BankIdAttempt a,
    Map<String, dynamic> input,
  ) async {
    expect(
      store.value!['secret'],
      a.secret,
      reason: 'Persist credential before starting BankID',
    );
    starts.add(input);
    if (startError != null) throw startError!;
    if (loseStart) throw StateError('lost response');
    return result;
  }

  @override
  Future<BankIdAttempt> status(BankIdAttempt a) async {
    statuses++;
    if (expired) throw const AttemptExpired();
    return delayedStatus == null ? result : delayedStatus!.future;
  }

  @override
  Future<BankIdAttempt> returned(BankIdAttempt a, String nonce) async {
    returns++;
    return result;
  }

  @override
  Future<BankIdAttempt> cancel(BankIdAttempt a) async {
    cancels++;
    return cancelled;
  }
}
