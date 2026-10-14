import 'package:shared_preferences/shared_preferences.dart';

abstract class UploadStore {
  Future<DateTime?> read();
  Future<void> write(DateTime completedAt, bool Function() isCurrent);
}

class PreferencesUploadStore implements UploadStore {
  final SharedPreferences preferences;
  final String key;
  PreferencesUploadStore(this.preferences, String baseUrl, String participantId)
    : key = '$baseUrl:steps:$participantId:completedAt';

  @override
  Future<DateTime?> read() async {
    // Legacy flags have no backend identity. They cannot safely prove completion
    // for any endpoint and are intentionally retired, never assigned to a user.
    for (final legacy in ['hasUploadedData', 'lastUploadAt', 'userId']) {
      await preferences.remove(legacy);
    }
    final value = preferences.getString(key);
    return value == null ? null : DateTime.tryParse(value);
  }

  @override
  Future<void> write(DateTime completedAt, bool Function() isCurrent) async {
    if (!isCurrent()) return;
    final previous = preferences.getString(key);
    final value = completedAt.toIso8601String();
    await preferences.setString(key, value);
    if (!isCurrent() && preferences.getString(key) == value) {
      if (previous == null) {
        await preferences.remove(key);
      } else {
        await preferences.setString(key, previous);
      }
    }
  }
}
