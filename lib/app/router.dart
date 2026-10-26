import 'package:flutter/cupertino.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../enrollment/bankid/controller.dart';
import '../enrollment/login_screen.dart';
import '../enrollment/consent_receipt_screen.dart';
import '../questionnaires/questionnaire_screen.dart';
import '../session/controller.dart';
import '../study/introduction_screen.dart';
import 'home_screen.dart';
import 'home_state.dart';
import 'done_screen.dart';

final startupProvider = FutureProvider<void>((ref) async {
  await ref.read(sessionProvider.notifier).restore();
  if (ref.read(sessionProvider) == null) {
    await ref.read(bankIdProvider.notifier).restore();
  }
});

String? participantRedirect({
  required String path,
  required bool ready,
  required bool signedIn,
  required bool pendingBankId,
  required bool eligible,
  required bool canFinish,
}) {
  if (!ready) return path == '/loading' ? null : '/loading';
  final public = path == '/introduction' || path == '/introduction/login';
  if (!signedIn) {
    return public
        ? null
        : pendingBankId
        ? '/introduction/login'
        : '/introduction';
  }
  if (public || path == '/loading') return '/home';
  if (!eligible && path.startsWith('/questionnaires/')) return '/home';
  if (path == '/done' && !canFinish) return '/home';
  return null;
}

class _RouterRefresh extends ChangeNotifier {
  void changed() => notifyListeners();
}

// Alternate entry points can choose a starting location; all redirects still run.
final initialLocationProvider = Provider<String>((ref) => '/loading');

final routerProvider = Provider<GoRouter>((ref) {
  final refresh = _RouterRefresh();
  ref.listen(startupProvider, (_, _) => refresh.changed());
  ref.listen(sessionProvider, (_, _) => refresh.changed());
  ref.listen(
    bankIdProvider.select((s) => s.begun),
    (_, _) => refresh.changed(),
  );
  ref.listen(
    homeStateProvider.select((s) => s.canFinish),
    (_, _) => refresh.changed(),
  );
  final router = GoRouter(
    initialLocation: ref.watch(initialLocationProvider),
    refreshListenable: refresh,
    redirect: (context, state) {
      final startup = ref.read(startupProvider);
      final session = ref.read(sessionProvider);
      return participantRedirect(
        path: state.uri.path,
        ready: startup.hasValue && !startup.isLoading && !startup.hasError,
        signedIn: session != null,
        pendingBankId: ref.read(bankIdProvider).begun,
        eligible: session?.participant.canParticipate == true,
        canFinish: ref.read(homeStateProvider).canFinish,
      );
    },
    routes: [
      GoRoute(path: '/loading', builder: (_, _) => const _StartupScreen()),
      GoRoute(
        path: '/introduction',
        name: 'introduction',
        builder: (_, _) => const IntroductionScreen(),
      ),
      GoRoute(
        path: '/introduction/login',
        name: 'login',
        builder: (_, _) => const LoginScreen(),
      ),
      GoRoute(
        path: '/home',
        name: 'home',
        builder: (_, _) => const HomeScreen(),
      ),
      GoRoute(
        path: '/done',
        name: 'done',
        builder: (_, _) => const DoneScreen(),
      ),
      GoRoute(
        path: '/consent-receipt',
        name: 'consent-receipt',
        builder: (_, _) => const ConsentReceiptScreen(),
      ),
      GoRoute(
        path: '/questionnaires/:id',
        name: 'questionnaire',
        builder: (_, state) => QuestionnaireScreen(
          questionnaireId: state.pathParameters['id']!,
          initialQuestionId: state.uri.queryParameters['question'],
        ),
      ),
      for (final path in [
        '/upload',
        '/summary',
        '/questionnaires',
        '/guardians',
      ])
        GoRoute(path: path, redirect: (_, _) => '/home'),
    ],
  );
  ref.onDispose(() {
    router.dispose();
    refresh.dispose();
  });
  return router;
});

class _StartupScreen extends ConsumerWidget {
  const _StartupScreen();
  @override
  Widget build(BuildContext context, WidgetRef ref) => CupertinoPageScaffold(
    child: Center(
      child: ref.watch(startupProvider).hasError
          ? Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Text('The app could not start. Please try again.'),
                CupertinoButton(
                  onPressed: () => ref.invalidate(startupProvider),
                  child: const Text('Retry'),
                ),
              ],
            )
          : const CupertinoActivityIndicator(),
    ),
  );
}
