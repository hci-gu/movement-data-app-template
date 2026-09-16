import 'dart:convert';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:research_steps_template/app_config.dart';
import 'package:shared_preferences/shared_preferences.dart';

class Storage {
  late SharedPreferences prefs;
  final FlutterSecureStorage _secure = const FlutterSecureStorage(
    iOptions: IOSOptions(
      accessibility: KeychainAccessibility.first_unlock_this_device,
    ),
  );
  String _key(String name) => '${AppConfig.apiBaseUrl}:$name';
  Future<Map<String, dynamic>?> _read(String name) async {
    final value = await _secure.read(key: _key(name));
    if (value == null) return null;
    try {
      return Map<String, dynamic>.from(jsonDecode(value) as Map);
    } catch (_) {
      await _secure.delete(key: _key(name));
      return null;
    }
  }

  Future<Map<String, dynamic>?> readAuthSession() => _read('authSessionV2');
  Future<void> writeAuthSession(Map<String, dynamic> session) =>
      _secure.write(key: _key('authSessionV2'), value: jsonEncode(session));
  Future<Map<String, dynamic>?> readBankIdAttempt() => _read('bankidAttempt');
  Future<void> writeBankIdAttempt(Map<String, dynamic> attempt) =>
      _secure.write(key: _key('bankidAttempt'), value: jsonEncode(attempt));
  Future<void> clearBankIdAttempt() =>
      _secure.delete(key: _key('bankidAttempt'));

  Future<void> reloadPrefs() async {
    prefs = await SharedPreferences.getInstance();
  }

  String? getParticipantId() {
    return prefs.getString('participantId');
  }

  Future<void> storeParticipantId(String participantId) async {
    await reloadPrefs();
    await prefs.setString('participantId', participantId);
  }

  bool getHasUploadedData() {
    return prefs.getBool('hasUploadedData') ?? false;
  }

  Future<void> setHasUploadedData(bool value) async {
    await reloadPrefs();
    await prefs.setBool('hasUploadedData', value);
  }

  DateTime? getLastUploadAt() {
    final rawValue = prefs.getString('lastUploadAt');
    if (rawValue == null) {
      return null;
    }

    return DateTime.tryParse(rawValue);
  }

  Future<void> storeLastUploadAt(DateTime value) async {
    await reloadPrefs();
    await prefs.setString('lastUploadAt', value.toIso8601String());
  }

  Future<void> clearSession() async {
    await _secure.delete(key: _key('authSessionV2'));
    final sharedPrefs = await SharedPreferences.getInstance();
    await sharedPrefs.remove('participantId');
    await sharedPrefs.remove('hasUploadedData');
    await sharedPrefs.remove('lastUploadAt');
  }

  static final Storage _instance = Storage._internal();

  factory Storage() {
    return _instance;
  }

  Storage._internal();
}
