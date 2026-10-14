import 'package:app_links/app_links.dart';
import 'package:flutter/cupertino.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'app/app.dart';
import 'infrastructure/local_storage.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  AppLinks(); // Subscribe early enough to receive a cold-start return link.
  final preferences = await SharedPreferences.getInstance();
  runApp(
    ProviderScope(
      overrides: [preferencesProvider.overrideWithValue(preferences)],
      child: const App(),
    ),
  );
}
