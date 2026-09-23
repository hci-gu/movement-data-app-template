import 'dart:async';
import 'dart:convert';
import 'dart:math';
import 'package:crypto/crypto.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/app_config.dart';
import 'package:research_steps_template/bankid/gateway.dart';
import 'package:research_steps_template/bankid/models.dart';
import 'package:research_steps_template/state/auth.dart';
import 'package:research_steps_template/storage.dart';
import 'package:url_launcher/url_launcher.dart';

abstract class BankIdAttemptStore {
  Future<Map<String, dynamic>?> read();
  Future<void> write(Map<String, dynamic> value);
  Future<void> clear();
}

class SecureBankIdAttemptStore implements BankIdAttemptStore {
  @override
  Future<Map<String, dynamic>?> read() => Storage().readBankIdAttempt();
  @override
  Future<void> write(Map<String, dynamic> value) =>
      Storage().writeBankIdAttempt(value);
  @override
  Future<void> clear() => Storage().clearBankIdAttempt();
}

class SigningState {
  final BankIdAttempt? attempt;
  final ConsentDocument? document;
  final bool begun, busy, reviewed;
  final String? error;
  const SigningState({
    this.attempt,
    this.document,
    this.begun = false,
    this.busy = false,
    this.reviewed = false,
    this.error,
  });
  SigningState copyWith({
    BankIdAttempt? attempt,
    ConsentDocument? document,
    bool? begun,
    bool? busy,
    bool? reviewed,
    String? error,
    bool clearAttempt = false,
    bool clearError = false,
  }) => SigningState(
    attempt: clearAttempt ? null : attempt ?? this.attempt,
    document: document ?? this.document,
    begun: begun ?? this.begun,
    busy: busy ?? this.busy,
    reviewed: reviewed ?? this.reviewed,
    error: clearError ? null : error ?? this.error,
  );
}

class BankIdController extends StateNotifier<SigningState> {
  final BankIdGateway gateway;
  final BankIdAttemptStore store;
  final Future<bool> Function(Uri) launch;
  final Future<void> Function(Map<String, dynamic>) onGrant;
  final DateTime Function() now;
  Timer? _timer;
  Future<void>? _refreshing, _restoration;
  BankIdController({
    required this.gateway,
    required this.store,
    required this.launch,
    required this.onGrant,
    DateTime Function()? now,
  }) : now = now ?? DateTime.now,
       super(const SigningState());

  Future<void> _error(Object error) async {
    if (error is AttemptExpired) {
      _timer?.cancel();
      _timer = null;
      await store.clear();
      if (mounted) state = SigningState(error: bankIdError(error));
    } else if (mounted) {
      state = state.copyWith(error: bankIdError(error));
    }
  }

  Future<void> _run(Future<void> Function() action) async {
    if (state.busy) return;
    state = state.copyWith(busy: true, clearError: true);
    try {
      await _refreshing;
      if (mounted) await action();
    } catch (e) {
      await _error(e);
    } finally {
      if (mounted) state = state.copyWith(busy: false);
    }
  }

  Future<void> restore() => _restoration ??= _run(() async {
    final saved = await store.read();
    if (saved == null) return;
    final a = BankIdAttempt.fromJson(saved, saved['secret'] as String);
    if (a.expired(now())) throw const AttemptExpired();
    state = state.copyWith(attempt: a, begun: true);
    await _update(await gateway.status(a));
  });
  Future<void> initialize() async {
    await restore();
    if (mounted && !state.begun) await begin();
  }

  void setReviewed(bool value) => state = state.copyWith(reviewed: value);
  Future<void> begin() => _run(() async {
    final doc = await gateway.currentConsent();
    state = SigningState(begun: true, document: doc, busy: true);
  });
  Future<void> start(String mode) => _run(() => _start(mode));
  Future<void> _start(String mode) async {
    if (state.document == null || !state.reviewed) return;
    if (state.attempt?.pending == true) {
      await _update(await gateway.status(state.attempt!));
      return;
    }
    final secret = base64UrlEncode(
      List<int>.generate(32, (_) => Random.secure().nextInt(256)),
    ).replaceAll('=', '');
    final a = BankIdAttempt(
      id: sha256.convert(utf8.encode(secret)).toString().substring(0, 32),
      secret: secret,
      mode: mode,
      purpose: 'sign',
      expiresAt:
          now().add(const Duration(minutes: 30)).millisecondsSinceEpoch ~/ 1000,
    );
    await store.write(a.toStorage());
    state = state.copyWith(attempt: a);
    final input = <String, dynamic>{
      'clientSecret': secret,
      'consentTextId': state.document!.id,
      'documentHash': state.document!.documentHash,
      'mode': mode,
    };
    BankIdAttempt result;
    try {
      result = await gateway.start(a, input);
    } on BankIdStartRejected catch (error) {
      await store.clear();
      state = state.copyWith(clearAttempt: true, reviewed: false);
      if (error.reason == 'consentChanged') {
        state = state.copyWith(document: await gateway.currentConsent());
      }
      rethrow;
    }
    await _update(result);
    if (result.pending && mode == 'sameDevice' && result.launchUrl != null) {
      await _launch(result.launchUrl!);
    }
  }

  Future<void> _launch(String value) async {
    final uri = Uri.parse(value);
    if (uri.scheme != 'https' || uri.host != 'app.bankid.com') {
      throw StateError('Invalid launch URL');
    }
    if (!await launch(uri) && mounted) {
      state = state.copyWith(
        error:
            'BankID could not open. Install and set up the BankID app, then try again.',
      );
    }
  }

  Future<void> openBankId() => _run(() async {
    final a = state.attempt;
    if (a == null) return;
    final result = await gateway.status(a);
    await _update(result);
    if (result.launchUrl != null) await _launch(result.launchUrl!);
  });
  Future<void> refresh() {
    if (state.busy || state.attempt == null) return Future.value();
    return _refreshing ??= _refresh().whenComplete(() => _refreshing = null);
  }

  Future<void> _refresh() async {
    try {
      final a = state.attempt!;
      if (a.expired(now())) throw const AttemptExpired();
      await _update(await gateway.status(a));
    } catch (e) {
      await _error(e);
    }
  }

  Future<void> _update(BankIdAttempt a) async {
    if (!mounted) return;
    await store.write(a.toStorage());
    if (!mounted) return;
    state = state.copyWith(
      attempt: a,
      document: a.document,
      begun: true,
      clearError: true,
    );
    if (a.pending) {
      _timer ??= Timer.periodic(const Duration(seconds: 1), (_) => refresh());
      return;
    }
    _timer?.cancel();
    _timer = null;
    if (a.accepted && a.consentRequired) {
      final doc = await gateway.currentConsent();
      state = state.copyWith(
        document: doc,
        clearAttempt: true,
        reviewed: false,
      );
    }
  }

  Future<void> cancel() => _run(() async {
    if (state.attempt != null) {
      await _update(await gateway.cancel(state.attempt!));
    }
  });
  Future<void> reviewAgain() => _run(() async {
    final doc = await gateway.currentConsent();
    await store.clear();
    state = state.copyWith(document: doc, clearAttempt: true, reviewed: false);
  });
  Future<void> extendQR() => _run(() async {
    final a = state.attempt;
    if (a == null || !a.canExtendQR) return;
    final result = await gateway.cancel(a);
    await _update(result);
    if (!result.pending && !result.accepted) {
      state = state.copyWith(clearAttempt: true);
      await _start('qr');
    }
  });
  Future<void> finish() => _run(_finish);
  Future<void> _finish() async {
    final a = state.attempt;
    if (a == null || !a.accepted) return;
    // Recheck current consent and revocation before using a previously displayed receipt.
    final fresh = await gateway.status(a);
    if (fresh.consentRequired) {
      await _update(fresh);
      return;
    }
    if (fresh.grant == null) {
      throw StateError('No authenticated session received');
    }
    await onGrant(fresh.grant!);
    await store.clear();
    _timer?.cancel();
    _timer = null;
    if (mounted) state = const SigningState();
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
    final a = state.attempt;
    if (a == null) return;
    String? nonce;
    try {
      nonce = Uri.splitQueryString(uri.fragment)['nonce'];
    } on FormatException {
      return;
    }
    if (nonce == null || nonce != a.nonce) {
      state = state.copyWith(
        error: 'This BankID return does not match your current request.',
      );
      return;
    }
    await _run(() async {
      await _update(await gateway.returned(a, nonce!));
    });
  }

  Future<void> reset() => _run(() async {
    if (state.attempt?.pending == true) {
      final result = await gateway.cancel(state.attempt!);
      if (result.accepted) {
        await _update(result);
        return;
      }
    }
    _timer?.cancel();
    _timer = null;
    await store.clear();
    final doc = await gateway.currentConsent();
    state = SigningState(begun: true, document: doc, busy: true);
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
    store: SecureBankIdAttemptStore(),
    launch: (uri) =>
        launchUrl(uri, mode: LaunchMode.externalNonBrowserApplication),
    onGrant: (grant) async {
      await ref.read(authProvider.notifier).accept(grant);
      ref.read(dataUploadedProvider.notifier).state = Storage()
          .getHasUploadedData();
    },
  ),
);
