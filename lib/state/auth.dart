import 'dart:async';

import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:pocketbase/pocketbase.dart';
import 'package:research_steps_template/api.dart';
import 'package:research_steps_template/pocketbase.dart';
import 'package:research_steps_template/state/health.dart';
import 'package:research_steps_template/storage.dart';

class Auth extends StateNotifier<RecordAuth?> {
  Timer? _expiry;
  Auth() : super(null) {
    Api().onSessionInvalid = invalidate;
  }

  Future<void> tryAutoLogin() async {
    final saved = await Storage().readAuthSession();
    if (saved == null) return;
    final expiresAt = saved['expiresAt'] as int? ?? 0;
    if (DateTime.now().millisecondsSinceEpoch ~/ 1000 >= expiresAt) {
      await invalidate();
      return;
    }
    try {
      pb.authStore.save(
        saved['token'] as String,
        RecordModel(Map<String, dynamic>.from(saved['record'] as Map)),
      );
      final user = await Api().currentParticipant();
      if (user['consentRequired'] == true) {
        await invalidate();
        return;
      }
      await accept({...saved, 'record': user});
    } catch (_) {
      // Never grant HealthKit/upload access on an unverified cached session.
      await invalidate();
    }
  }

  Future<void> accept(Map<String, dynamic> grant) async {
    final expiresAt = grant['expiresAt'] as int;
    if (grant['consentRequired'] == true ||
        DateTime.now().millisecondsSinceEpoch ~/ 1000 >= expiresAt) {
      throw StateError('The server did not grant an active session.');
    }
    final auth = RecordAuth.fromJson(grant);
    final previousParticipant = Storage().getParticipantId();
    if (previousParticipant != auth.record.get<String>('username')) {
      HealthManager().reset();
      await Storage().setHasUploadedData(false);
    }
    await Storage().writeAuthSession(grant);
    await Storage().storeParticipantId(auth.record.get<String>('username'));
    pb.authStore.save(auth.token, auth.record);
    state = auth;
    _expiry?.cancel();
    _expiry = Timer(
      DateTime.fromMillisecondsSinceEpoch(
        expiresAt * 1000,
      ).difference(DateTime.now()),
      invalidate,
    );
  }

  Future<void> validateSession() async {
    if (state == null) return;
    try {
      final user = await Api().currentParticipant();
      if (user['consentRequired'] == true) await invalidate();
    } catch (_) {
      await invalidate();
    }
  }

  Future<void> invalidate() async {
    _expiry?.cancel();
    pb.authStore.clear();
    state = null;
    HealthManager().reset();
    await Storage().clearSession();
  }

  Future<void> logout() async {
    try {
      if (state != null) await Api().logout();
    } finally {
      await invalidate();
    }
  }

  @override
  void dispose() {
    _expiry?.cancel();
    Api().onSessionInvalid = null;
    super.dispose();
  }
}

final authProvider = StateNotifierProvider<Auth, RecordAuth?>((ref) => Auth());
final dataUploadedProvider = StateProvider<bool>(
  (ref) => Storage().getHasUploadedData(),
);
