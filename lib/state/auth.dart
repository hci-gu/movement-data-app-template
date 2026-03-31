import 'dart:math';

import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:pocketbase/pocketbase.dart';
import 'package:research_steps_template/api.dart';
import 'package:research_steps_template/pocketbase.dart';
import 'package:research_steps_template/storage.dart';

String _generatePassword() {
  final random = Random.secure();
  const chars =
      'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
  return List.generate(32, (_) => chars[random.nextInt(chars.length)]).join();
}

class Auth extends StateNotifier<RecordAuth?> {
  final String? _participantId;

  Auth([this._participantId]) : super(null);

  Future<void> tryAutoLogin() async {
    if (_participantId == null) {
      return;
    }

    try {
      await login(_participantId);
    } catch (_) {
      await Storage().clearSession();
    }
  }

  Future<void> login(String participantId) async {
    final password = Storage().getPassword();
    if (password == null) {
      throw Exception('No stored password');
    }

    final loginState = await pb
        .collection('users')
        .authWithPassword(participantId, password);
    await Storage().storeParticipantId(participantId);
    state = loginState;
  }

  Future<void> signup(
    String participantId, {
    required bool consentAccepted,
  }) async {
    final password = _generatePassword();

    await Api().registerParticipant(participantId, password, consentAccepted);

    state = await pb
        .collection('users')
        .authWithPassword(participantId, password);
    await Storage().storePassword(password);
    await Storage().storeParticipantId(participantId);
  }

  Future<void> logout() async {
    state = null;
    await Storage().clearSession();
  }
}

final authProvider = StateNotifierProvider<Auth, RecordAuth?>((ref) => Auth());

final dataUploadedProvider = StateProvider<bool>(
  (ref) => Storage().getHasUploadedData(),
);
