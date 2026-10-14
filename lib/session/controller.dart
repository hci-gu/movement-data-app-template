import 'dart:async';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../infrastructure/http_client.dart';
import '../infrastructure/local_storage.dart';
import 'gateway.dart';
import 'models.dart';
import 'store.dart';

class SessionController extends StateNotifier<Session?> {
  final SessionGateway gateway;
  final SessionStore store;
  final SessionConnection connection;
  final DateTime Function() now;
  Timer? _expiry;
  int _generation = 0;
  Future<void> _writes = Future.value();
  SessionController({
    required this.gateway,
    required this.store,
    required this.connection,
    DateTime Function()? now,
  }) : now = now ?? DateTime.now,
       super(null) {
    connection.onAccessDenied = handleAccessDenied;
  }

  bool _current(int generation) => mounted && generation == _generation;
  Future<void> _persist(Future<void> Function() action) {
    final result = _writes.then((_) => action());
    _writes = result.catchError((Object _) {});
    return result;
  }

  Future<void> restore() async {
    final generation = ++_generation;
    try {
      final saved = await store.read();
      if (!_current(generation) || saved == null) return;
      final session = Session.fromJson(saved);
      if (!session.expiresAt.isAfter(now())) {
        await invalidate();
        return;
      }
      connection.token = session.token;
      final participant = await gateway.current(session.token);
      if (!_current(generation)) return;
      if (participant.id != session.participant.id ||
          participant.consentRequired) {
        await invalidate();
        return;
      }
      await accept(session.withParticipant(participant).toJson());
    } catch (_) {
      if (_current(generation)) await invalidate();
    }
  }

  Future<void> accept(Map<String, dynamic> grant) async {
    final session = Session.fromJson(grant);
    if (!session.expiresAt.isAfter(now()) ||
        session.participant.consentRequired) {
      throw StateError('The server did not grant an active session.');
    }
    final generation = ++_generation;
    _expiry?.cancel();
    // Drop the old session before awaiting persistence; dependent work must stop.
    state = null;
    connection.token = '';
    await _persist(
      () =>
          _current(generation) ? store.write(session.toJson()) : Future.value(),
    );
    if (!_current(generation)) return;
    connection.token = session.token;
    state = session;
    _expiry = Timer(session.expiresAt.difference(now()), invalidate);
  }

  Future<void> refresh() async {
    final current = state;
    final generation = _generation;
    if (current == null) return;
    try {
      final participant = await gateway.current(current.token);
      if (!_current(generation)) return;
      if (participant.id != current.participant.id ||
          participant.consentRequired ||
          !current.expiresAt.isAfter(now())) {
        await invalidate();
        return;
      }
      state = current.withParticipant(participant);
    } catch (_) {
      if (_current(generation)) await invalidate();
    }
  }

  Future<void> handleAccessDenied(String token, String reason) async {
    if (!mounted || token != connection.token) return;
    if (reason == 'sessionExpired' || reason == 'consentRequired') {
      await invalidate();
    } else if (reason == 'guardianRequired' && state != null) {
      state = state!.withParticipant(state!.participant.requiringGuardian());
    }
  }

  Future<void> invalidate() async {
    ++_generation;
    _expiry?.cancel();
    connection.token = '';
    if (mounted) state = null;
    await _persist(store.clear);
  }

  Future<void> logout() async {
    final token = state?.token;
    await invalidate();
    if (token != null) {
      try {
        await gateway.logout(token);
      } catch (_) {
        /* Local session is already revoked. */
      }
    }
  }

  @override
  void dispose() {
    ++_generation;
    _expiry?.cancel();
    connection.onAccessDenied = null;
    connection.token = '';
    super.dispose();
  }
}

final sessionStoreProvider = Provider<SessionStore>(
  (ref) => SecureSessionStore(ref.watch(secureStorageProvider)),
);
final sessionGatewayProvider = Provider<SessionGateway>(
  (ref) => HttpSessionGateway(ref.watch(httpClientProvider)),
);
final sessionProvider = StateNotifierProvider<SessionController, Session?>(
  (ref) => SessionController(
    gateway: ref.watch(sessionGatewayProvider),
    store: ref.watch(sessionStoreProvider),
    connection: ref.watch(sessionConnectionProvider),
  ),
);
