import 'dart:async';
import 'package:app_links/app_links.dart';
import 'package:flutter/cupertino.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../enrollment/bankid/controller.dart';
import '../enrollment/guardians_controller.dart';
import '../session/controller.dart';
import '../questionnaires/controller.dart';
import '../study/content.dart';
import '../ui/theme.dart';
import 'router.dart';

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
          if (uri != null && mounted) _returned(uri);
        })
        .catchError((Object _) {});
  }

  Future<void> _returned(Uri uri) async {
    await ref.read(startupProvider.future);
    if (!mounted || ref.read(sessionProvider) != null) return;
    await ref.read(bankIdProvider.notifier).handleReturn(uri);
    if (mounted &&
        ref.read(sessionProvider) == null &&
        ref.read(bankIdProvider).begun) {
      ref.read(routerProvider).goNamed('login');
    }
  }

  Future<void> _resume() async {
    await ref.read(startupProvider.future);
    if (!mounted) return;
    await ref.read(sessionProvider.notifier).refresh();
    if (!mounted) return;
    await ref.read(bankIdProvider.notifier).refresh();
    if (!mounted) return;
    ref.invalidate(questionnaireListProvider);
    ref.invalidate(guardiansProvider);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      unawaited(_resume().catchError((Object _) {}));
    }
  }

  @override
  void dispose() {
    _links?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => const AppView();
}

/// The real app UI, shared by native startup and the isolated demo entry point.
class AppView extends ConsumerWidget {
  const AppView({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) => GestureDetector(
    onTap: () => FocusManager.instance.primaryFocus?.unfocus(),
    child: CupertinoApp.router(
      debugShowCheckedModeBanner: false,
      title: StudyContent.appName,
      theme: AppTheme.cupertinoTheme,
      routerConfig: ref.watch(routerProvider),
    ),
  );
}
