import 'dart:async';
import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';
import 'package:crypto/crypto.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../app/environment.dart';
import '../../session/controller.dart';
import '../../infrastructure/http_client.dart';
import '../../infrastructure/local_storage.dart';
import 'gateway.dart';
import 'models.dart';

abstract class BankIdAttemptStore {
  Future<Map<String, dynamic>?> read();
  Future<void> write(Map<String, dynamic> value);
  Future<void> clear();
}

class SecureBankIdAttemptStore implements BankIdAttemptStore {
  final SecureJsonStorage storage;
  SecureBankIdAttemptStore(this.storage);
  @override
  Future<Map<String, dynamic>?> read() => storage.read('kontoapiAttempt');
  @override
  Future<void> write(Map<String, dynamic> value) =>
      storage.write('kontoapiAttempt', value);
  @override
  Future<void> clear() => storage.clear('kontoapiAttempt');
}

class AuthenticationState {
  final BankIdAttempt? attempt;
  final bool begun, busy;
  final String? error;
  final Uint8List? qrImage;
  const AuthenticationState({
    this.attempt,
    this.begun = false,
    this.busy = false,
    this.error,
    this.qrImage,
  });
  AuthenticationState copyWith({
    BankIdAttempt? attempt,
    bool? begun,
    bool? busy,
    String? error,
    Uint8List? qrImage,
    bool clearAttempt = false,
    bool clearError = false,
    bool clearQR = false,
  }) => AuthenticationState(
    attempt: clearAttempt ? null : attempt ?? this.attempt,
    begun: begun ?? this.begun,
    busy: busy ?? this.busy,
    error: clearError ? null : error ?? this.error,
    qrImage: clearQR ? null : qrImage ?? this.qrImage,
  );
}

class BankIdController extends StateNotifier<AuthenticationState> {
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
       super(const AuthenticationState());
  Future<void> _error(Object error) async {
    if (error is AttemptExpired) {
      _timer?.cancel();
      _timer = null;
      await store.clear();
      if (mounted) {
        state = AuthenticationState(begun: true, error: bankIdError(error));
      }
    } else if (mounted) {
      state = state.copyWith(error: bankIdError(error), clearQR: true);
    }
  }

  Future<void> _run(Future<void> Function() action) async {
    if (!mounted || state.busy) return;
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
    if (!mounted || saved == null) return;
    final a = BankIdAttempt.fromJson(saved, saved['secret'] as String);
    if (a.expired(now())) throw const AttemptExpired();
    state = state.copyWith(attempt: a, begun: true);
    await _update(await gateway.status(a));
  });
  Future<void> initialize() async {
    await restore();
    if (mounted && !state.begun) await begin();
  }

  Future<void> begin() => _run(() async {
    state = const AuthenticationState(begun: true, busy: true);
  });
  Future<void> start(String mode) => _run(() => _start(mode));
  Future<void> _start(String mode) async {
    if (state.attempt?.pending == true) {
      await _update(await gateway.status(state.attempt!));
      return;
    }
    final secret = base64UrlEncode(
      List.generate(32, (_) => Random.secure().nextInt(256)),
    ).replaceAll('=', '');
    final a = BankIdAttempt(
      id: sha256.convert(utf8.encode(secret)).toString().substring(0, 32),
      secret: secret,
      mode: mode,
      expiresAt:
          now().add(const Duration(minutes: 3)).millisecondsSinceEpoch ~/ 1000,
    );
    await store.write(a.toStorage());
    if (!mounted) return;
    state = state.copyWith(attempt: a, begun: true, clearQR: true);
    BankIdAttempt result;
    try {
      result = await gateway.start(a, {'clientSecret': secret, 'mode': mode});
    } on BankIdStartRejected {
      await store.clear();
      if (mounted) state = state.copyWith(clearAttempt: true);
      rethrow;
    }
    await _update(result);
    if (mounted &&
        result.pending &&
        mode == 'sameDevice' &&
        result.launchUrl != null) {
      await _launch(result.launchUrl!);
    }
  }

  Future<void> _launch(String value) async {
    final uri = Uri.parse(value);
    if (uri.scheme != 'https' ||
        uri.host != 'app.bankid.com' ||
        uri.userInfo.isNotEmpty) {
      throw StateError('Invalid launch URL');
    }
    if (!await launch(uri) && mounted) {
      state = state.copyWith(
        error:
            'BankID kunde inte öppnas. Installera och aktivera BankID och försök igen.',
      );
    }
  }

  Future<void> openBankId() => _run(() async {
    final a = state.attempt;
    if (a == null) return;
    final result = await gateway.status(a);
    await _update(result);
    if (mounted && result.pending && result.launchUrl != null) {
      await _launch(result.launchUrl!);
    }
  });
  Future<void> refresh() {
    if (!mounted || state.busy || state.attempt == null) return Future.value();
    return _refreshing ??= _refresh().whenComplete(() => _refreshing = null);
  }

  Future<void> _refresh() async {
    try {
      await _update(await gateway.status(state.attempt!));
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
      begun: true,
      clearError: true,
      clearQR:
          !a.pending ||
          a.mode != 'qr' ||
          a.canExtendQR ||
          state.attempt?.id != a.id,
    );
    if (a.pending) {
      _timer ??= Timer.periodic(const Duration(seconds: 1), (_) => refresh());
      if (a.mode == 'qr' && (a.qrSecondsRemaining ?? 1) > 0) {
        final bytes = await gateway.qrImage(a);
        if (mounted && state.attempt?.id == a.id) {
          state = state.copyWith(qrImage: bytes);
        }
      }
    } else {
      _timer?.cancel();
      _timer = null;
    }
  }

  Future<void> cancel() => _run(() async {
    if (state.attempt != null) {
      await _update(await gateway.cancel(state.attempt!));
    }
  });
  Future<void> restart() => _run(() async {
    if (state.attempt?.pending == true) await gateway.cancel(state.attempt!);
    await store.clear();
    _timer?.cancel();
    _timer = null;
    if (mounted) state = const AuthenticationState(begun: true, busy: true);
  });
  Future<void> extendQR() => _run(() async {
    final a = state.attempt;
    if (a == null || !a.canExtendQR) return;
    final result = await gateway.cancel(a);
    await _update(result);
    if (mounted && !result.pending && !result.accepted) {
      state = state.copyWith(clearAttempt: true);
      await _start('qr');
    }
  });
  Future<void> finish() => _run(() async {
    final a = state.attempt;
    if (a == null || !a.accepted) return;
    final fresh = await gateway.status(a);
    if (!mounted) return;
    if (!fresh.accepted || fresh.grant == null) {
      throw StateError('No authenticated session received');
    }
    await onGrant(fresh.grant!);
    // Accepting a session disposes this controller. Still remove the finished credential.
    await store.clear();
    _timer?.cancel();
    _timer = null;
    if (mounted) state = const AuthenticationState();
  });
  Future<void> handleReturn(Uri uri) async {
    final expected = Uri.parse(Environment.bankIdReturnUrl);
    if (uri.scheme != expected.scheme ||
        uri.host != expected.host ||
        uri.path != expected.path ||
        uri.userInfo.isNotEmpty) {
      return;
    }
    await restore();
    if (!mounted) return;
    // KontoAPI does not document returnUrl support. A link is only a signal to
    // poll our owned attempt; no URL parameter is an authentication result.
    await refresh();
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }
}

final bankIdProvider =
    StateNotifierProvider<BankIdController, AuthenticationState>((ref) {
      ref.watch(sessionProvider.select((s) => s?.token));
      late final BankIdController controller;
      controller = BankIdController(
        gateway: HttpBankIdGateway(ref.watch(httpClientProvider)),
        store: SecureBankIdAttemptStore(ref.watch(secureStorageProvider)),
        launch: (uri) =>
            launchUrl(uri, mode: LaunchMode.externalNonBrowserApplication),
        onGrant: (grant) async {
          if (controller.mounted) {
            await ref.read(sessionProvider.notifier).accept(grant);
          }
        },
      );
      return controller;
    });
