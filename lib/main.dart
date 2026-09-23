import 'dart:async';
import 'package:app_links/app_links.dart';
import 'package:research_steps_template/bankid/controller.dart';
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
  AppLinks(); // Subscribe early enough to receive a cold-start return link.
  Api().init(AppConfig.apiBaseUrl);

  runApp(ProviderScope(child: const App()));
}

class App extends ConsumerStatefulWidget {
  const App({super.key});
  @override
  ConsumerState<App> createState() => _AppState();
}

class _AppState extends ConsumerState<App> with WidgetsBindingObserver {
  StreamSubscription<Uri>? _links;
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    final links = AppLinks();
    _links = links.uriLinkStream.listen(_returned, onError: (Object _) {});
    links
        .getInitialLink()
        .then((uri) {
          if (uri != null) _returned(uri);
        })
        .catchError((Object _) {});
  }

  Future<void> _returned(Uri uri) async {
    await ref.read(bankIdProvider.notifier).handleReturn(uri);
    if (mounted && ref.read(bankIdProvider).begun) {
      ref.read(routerProvider).goNamed('login');
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      ref.read(authProvider.notifier).validateSession();
      ref.read(bankIdProvider.notifier).refresh();
    }
  }

  @override
  void dispose() {
    _links?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
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
