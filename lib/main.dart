import 'package:flutter/cupertino.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/api.dart';
import 'package:research_steps_template/app_config.dart';
import 'package:research_steps_template/router.dart';
import 'package:research_steps_template/state/auth.dart';
import 'package:research_steps_template/storage.dart';
import 'package:research_steps_template/theme.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Storage().reloadPrefs();
  final participantId = Storage().getParticipantId();
  Api().init(AppConfig.apiBaseUrl);

  runApp(
    ProviderScope(
      overrides: participantId != null
          ? [authProvider.overrideWith((ref) => Auth(participantId))]
          : [],
      child: const App(),
    ),
  );
}

class App extends ConsumerWidget {
  const App({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);

    return GestureDetector(
      onTap: () {
        final currentFocus = FocusScope.of(context);
        if (!currentFocus.hasPrimaryFocus) {
          FocusManager.instance.primaryFocus?.unfocus();
        }
      },
      child: CupertinoApp.router(
        debugShowCheckedModeBanner: false,
        title: AppConfig.appName,
        theme: AppTheme.cupertinoTheme,
        routerConfig: router,
      ),
    );
  }
}
