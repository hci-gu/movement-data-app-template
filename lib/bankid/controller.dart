import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/app_config.dart';
import 'package:research_steps_template/bankid/gateway.dart';
import 'package:research_steps_template/bankid/models.dart';
import 'package:research_steps_template/state/auth.dart';
import 'package:research_steps_template/storage.dart';
import 'package:url_launcher/url_launcher.dart';

abstract class BankIdFlowStore {
  Future<Map<String, dynamic>?> read();
  Future<void> write(Map<String, dynamic> value);
  Future<void> clear();
}

class SecureBankIdFlowStore implements BankIdFlowStore {
  @override
  Future<Map<String, dynamic>?> read() => Storage().readBankIdFlow();
  @override
  Future<void> write(Map<String, dynamic> value) =>
      Storage().writeBankIdFlow(value);
  @override
  Future<void> clear() => Storage().clearBankIdFlow();
}

class SigningState {
  final BankIdFlow? flow;
  final ConsentDocument? document;
  final BankIdOrder? order;
  final bool busy, reviewed;
  final String? error;
  const SigningState({
    this.flow,
    this.document,
    this.order,
    this.busy = false,
    this.reviewed = false,
    this.error,
  });
  SigningState copyWith({
    BankIdFlow? flow,
    ConsentDocument? document,
    BankIdOrder? order,
    bool? busy,
    bool? reviewed,
    String? error,
    bool clearOrder = false,
    bool clearError = false,
  }) => SigningState(
    flow: flow ?? this.flow,
    document: document ?? this.document,
    order: clearOrder ? null : order ?? this.order,
    busy: busy ?? this.busy,
    reviewed: reviewed ?? this.reviewed,
    error: clearError ? null : error ?? this.error,
  );
}

class BankIdController extends StateNotifier<SigningState> {
  final BankIdGateway gateway;
  final BankIdFlowStore store;
  final Future<bool> Function(Uri) launch;
  final Future<void> Function(Map<String, dynamic>) onGrant;
  final DateTime Function() now;
  Timer? _timer;
  Future<void>? _refreshing;
  Future<void>? _restoration;

  BankIdController({
    required this.gateway,
    required this.store,
    required this.launch,
    required this.onGrant,
    DateTime Function()? now,
  }) : now = now ?? DateTime.now,
       super(const SigningState());
  String _random() => base64UrlEncode(
    List<int>.generate(32, (_) => Random.secure().nextInt(256)),
  ).replaceAll('=', '');

  Future<void> _save(BankIdFlow flow) async {
    await store.write(flow.toJson());
    state = state.copyWith(flow: flow);
  }

  Future<void> _run(Future<void> Function() action) async {
    if (state.busy) return;
    state = state.copyWith(busy: true, clearError: true);
    try {
      await _refreshing;
      if (mounted) await action();
    } catch (error) {
      if (mounted) state = state.copyWith(error: bankIdError(error));
    } finally {
      if (mounted) state = state.copyWith(busy: false);
    }
  }

  Future<void> restore() => _restoration ??= _run(_restore);
  Future<void> _restore() async {
    try {
      final saved = await store.read();
      if (saved == null) return;
      final flow = BankIdFlow.fromJson(saved);
      if (flow.expired(now())) {
        await store.clear();
        return;
      }
      state = SigningState(flow: flow, document: flow.document);
      if (flow.id.isEmpty) {
        return; // user can retry the persisted enrollment request
      }
      if (flow.requestKey.isNotEmpty) {
        // Same key recovers the launch response after a lost HTTP response.
        final order = await gateway.start(flow);
        await _save(
          flow.copyWith(orderId: order.id, nonce: order.nonce ?? flow.nonce),
        );
        await _updateOrder(order);
      } else if (flow.kind == 'enroll' || flow.document != null) {
        final document = await gateway.currentConsent();
        state = state.copyWith(document: document);
      }
    } catch (error) {
      if (mounted) state = state.copyWith(error: bankIdError(error));
    }
  }

  void setReviewed(bool value) => state = state.copyWith(reviewed: value);

  Future<void> begin({String invitationCode = '', bool returning = false}) =>
      _run(() async {
        _timer?.cancel();
        _timer = null;
        // Reuse an unacknowledged creation request after a network failure.
        var flow = state.flow;
        if (flow == null || flow.id.isNotEmpty || flow.expired(now())) {
          flow = BankIdFlow(
            id: '',
            clientSecret: _random(),
            kind: returning ? 'login' : 'enroll',
            invitationCode: invitationCode.trim(),
            expiresAt:
                now().add(const Duration(minutes: 20)).millisecondsSinceEpoch ~/
                1000,
          );
          state = const SigningState(busy: true);
          await _save(flow);
        }
        final response = await gateway.createFlow(flow);
        flow = flow.copyWith(
          id: response['id'] as String,
          expiresAt: response['expiresAt'] as int,
        );
        await _save(flow);
        if (flow.kind == 'enroll') {
          final document = await gateway.currentConsent();
          state = state.copyWith(
            document: document,
            reviewed: false,
            clearOrder: true,
          );
        }
      });

  Future<void> start(String mode, {bool retry = false}) => _run(() async {
    var flow = state.flow;
    if (flow == null || flow.id.isEmpty || flow.expired(now())) {
      throw StateError('No active enrollment.');
    }
    final isConsent = state.document != null;
    if (!retry && isConsent && !state.reviewed) return;
    if (!retry) {
      flow = flow.copyWith(
        requestKey: _random(),
        mode: mode,
        orderId: '',
        nonce: '',
        document: state.document,
      );
      await _save(
        flow,
      ); // must complete before the BankID request or app launch
    }
    final order = await gateway.start(flow);
    flow = flow.copyWith(orderId: order.id, nonce: order.nonce ?? flow.nonce);
    await _save(flow);
    await _updateOrder(order);
    if (order.pending && mode == 'sameDevice' && order.launchUrl != null) {
      await _launch(order.launchUrl!);
    }
  });

  Future<void> _launch(String value) async {
    final uri = Uri.parse(value);
    if (uri.scheme != 'https' || uri.host != 'app.bankid.com') {
      throw StateError('Invalid BankID launch URL.');
    }
    if (!await launch(uri)) {
      state = state.copyWith(
        error:
            'BankID could not open. Install and set up the BankID app, then try again.',
      );
    }
  }

  Future<void> openBankId() => _run(() async {
    final flow = state.flow;
    if (flow == null) return;
    final order = await gateway.start(
      flow,
    ); // recover the original launch token
    await _updateOrder(order);
    if (order.launchUrl != null) await _launch(order.launchUrl!);
  });

  Future<void> refresh() {
    if (state.busy || state.flow?.orderId.isEmpty != false) {
      return Future.value();
    }
    return _refreshing ??= _refresh().whenComplete(() => _refreshing = null);
  }

  Future<void> _refresh() async {
    try {
      if (state.flow!.expired(now())) {
        _timer?.cancel();
        _timer = null;
        state = state.copyWith(
          error: 'Your enrollment session expired. Start again.',
        );
        return;
      }
      await _updateOrder(await gateway.status(state.flow!));
    } catch (error) {
      if (mounted) state = state.copyWith(error: bankIdError(error));
    }
  }

  Future<void> _updateOrder(BankIdOrder order) async {
    if (!mounted) return;
    state = state.copyWith(order: order, clearError: true);
    if (order.pending) {
      _timer ??= Timer.periodic(const Duration(seconds: 1), (_) => refresh());
    } else {
      _timer?.cancel();
      _timer = null;
      if (order.accepted && order.purpose == 'auth') await _complete();
    }
  }

  Future<void> cancel() => _run(() async {
    if (state.flow == null) return;
    await _updateOrder(await gateway.cancel(state.flow!));
  });

  Future<void> reviewAgain() => _run(() async {
    final document = await gateway.currentConsent();
    final flow = state.flow!;
    await _save(
      flow.copyWith(requestKey: '', orderId: '', nonce: '', document: document),
    );
    state = state.copyWith(
      document: document,
      clearOrder: true,
      reviewed: false,
    );
  });

  Future<void> extendQR() => _run(() async {
    if (state.flow == null || state.order?.canExtendQR != true) return;
    final result = await gateway.cancel(state.flow!);
    await _updateOrder(result);
    if (result.accepted || result.pending) return;
    final flow = state.flow!.copyWith(
      requestKey: _random(),
      orderId: '',
      nonce: '',
    );
    await _save(flow);
    final order = await gateway.start(flow);
    await _save(flow.copyWith(orderId: order.id));
    await _updateOrder(order);
  });

  Future<void> finish() => _run(_complete);
  Future<void> _complete() async {
    final flow = state.flow;
    if (flow == null || state.order?.accepted != true) return;
    final grant = await gateway.complete(flow);
    if (grant['consentRequired'] == true) {
      final document = await gateway.currentConsent();
      await _save(
        flow.copyWith(
          requestKey: '',
          orderId: '',
          nonce: '',
          document: document,
        ),
      );
      state = state.copyWith(
        document: document,
        clearOrder: true,
        reviewed: false,
      );
      return;
    }
    await onGrant(grant);
    await store.clear();
    _timer?.cancel();
    _timer = null;
    state = const SigningState();
  }

  Future<void> handleReturn(Uri uri) async {
    final expected = Uri.parse(AppConfig.bankidReturnUrl);
    if (uri.scheme != expected.scheme ||
        uri.host != expected.host ||
        uri.path != expected.path ||
        uri.userInfo.isNotEmpty) {
      return;
    }
    await restore();
    if (state.flow == null || state.flow!.orderId.isEmpty) return;
    String? nonce;
    try {
      nonce = Uri.splitQueryString(uri.fragment)['nonce'];
    } on FormatException {
      return;
    }
    if (nonce == null || nonce != state.flow!.nonce) {
      state = state.copyWith(
        error: 'This BankID return does not match your current session.',
      );
      return;
    }
    await _run(() async {
      await _updateOrder(await gateway.returned(state.flow!, nonce!));
    });
  }

  Future<void> reset() => _run(() async {
    _timer?.cancel();
    _timer = null;
    final flow = state.flow;
    if (flow != null && flow.id.isNotEmpty && !flow.expired(now())) {
      await gateway.abandon(flow);
    }
    await store.clear();
    state = const SigningState(busy: true);
  });

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }
}

final bankIdProvider = StateNotifierProvider<BankIdController, SigningState>(
  (ref) => BankIdController(
    gateway: HttpBankIdGateway(),
    store: SecureBankIdFlowStore(),
    launch: (uri) =>
        launchUrl(uri, mode: LaunchMode.externalNonBrowserApplication),
    onGrant: (grant) async {
      await ref.read(authProvider.notifier).accept(grant);
      ref.read(dataUploadedProvider.notifier).state = Storage()
          .getHasUploadedData();
    },
  ),
);
