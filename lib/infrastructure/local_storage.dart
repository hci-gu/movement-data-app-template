import 'dart:convert';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../app/environment.dart';

final preferencesProvider = Provider<SharedPreferences>(
  (ref) => throw StateError('Preferences must be initialized at startup.'),
);

// Shared by the two actual credential stores: session and BankID attempt.
class SecureJsonStorage {
  final FlutterSecureStorage storage;
  final String baseUrl;
  const SecureJsonStorage(this.storage, this.baseUrl);
  String key(String name) => '$baseUrl:$name';
  Future<Map<String, dynamic>?> read(String name) async {
    final value = await storage.read(key: key(name));
    if (value == null) return null;
    try {
      return Map<String, dynamic>.from(jsonDecode(value) as Map);
    } catch (_) {
      await clear(name);
      return null;
    }
  }

  Future<void> write(String name, Map<String, dynamic> value) =>
      storage.write(key: key(name), value: jsonEncode(value));
  Future<void> clear(String name) => storage.delete(key: key(name));
}

final secureStorageProvider = Provider(
  (ref) => const SecureJsonStorage(
    FlutterSecureStorage(
      iOptions: IOSOptions(
        accessibility: KeychainAccessibility.first_unlock_this_device,
      ),
    ),
    Environment.apiBaseUrl,
  ),
);
