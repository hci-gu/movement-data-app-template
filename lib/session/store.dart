import '../infrastructure/local_storage.dart';

abstract class SessionStore {
  Future<Map<String, dynamic>?> read();
  Future<void> write(Map<String, dynamic> value);
  Future<void> clear();
}

class SecureSessionStore implements SessionStore {
  final SecureJsonStorage storage;
  SecureSessionStore(this.storage);
  @override
  Future<Map<String, dynamic>?> read() => storage.read('authSessionV2');
  @override
  Future<void> write(Map<String, dynamic> value) =>
      storage.write('authSessionV2', value);
  @override
  Future<void> clear() => storage.clear('authSessionV2');
}
