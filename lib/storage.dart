import 'package:shared_preferences/shared_preferences.dart';

class Storage {
  late SharedPreferences prefs;

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

  String? getPassword() {
    return prefs.getString('password');
  }

  Future<void> storePassword(String password) async {
    await reloadPrefs();
    await prefs.setString('password', password);
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
    final sharedPrefs = await SharedPreferences.getInstance();
    await sharedPrefs.remove('participantId');
    await sharedPrefs.remove('password');
    await sharedPrefs.remove('hasUploadedData');
    await sharedPrefs.remove('lastUploadAt');
  }

  static final Storage _instance = Storage._internal();

  factory Storage() {
    return _instance;
  }

  Storage._internal();
}
