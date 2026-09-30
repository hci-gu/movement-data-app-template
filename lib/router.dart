import 'package:research_steps_template/bankid/controller.dart';
import 'package:research_steps_template/screens/consent_receipt.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/screens/introduction.dart';
import 'package:research_steps_template/screens/login.dart';
import 'package:research_steps_template/screens/steps.dart';
import 'package:research_steps_template/screens/guardians.dart';
import 'package:research_steps_template/screens/summary.dart';
import 'package:research_steps_template/state/auth.dart';

class RouterNotifier extends ChangeNotifier {
  final Ref _ref;

  RouterNotifier(this._ref) {
    _ref.listen<String?>(
      authProvider.select((value) => value?.token),
      (_, _) => notifyListeners(),
    );
    _ref.listen<bool>(dataUploadedProvider, (_, _) => notifyListeners());
    _ref.listen<bool?>(guardianRequiredProvider, (_, _) => notifyListeners());
  }

  String? redirect(BuildContext context, GoRouterState state) {
    if (state.matchedLocation == '/loading') {
      return null;
    }

    final loggedIn = _ref.read(authProvider) != null;
    final hasUploadedData = _ref.read(dataUploadedProvider);
    final guardianRequired = _ref.read(guardianRequiredProvider) == true;
    final isPublicRoute =
        state.matchedLocation == '/introduction' ||
        state.matchedLocation == '/introduction/login';

    if (!loggedIn && !isPublicRoute) {
      return '/introduction';
    }

    if (loggedIn && isPublicRoute) {
      return guardianRequired
          ? '/guardians'
          : hasUploadedData
          ? '/summary'
          : '/upload';
    }

    if (loggedIn &&
        guardianRequired &&
        (state.matchedLocation == '/upload' ||
            state.matchedLocation == '/summary')) {
      return '/guardians';
    }

    if (loggedIn &&
        !guardianRequired &&
        state.matchedLocation == '/guardians') {
      return hasUploadedData ? '/summary' : '/upload';
    }

    return null;
  }
}

final routerProvider = Provider<GoRouter>((ref) {
  final routerNotifier = RouterNotifier(ref);

  return GoRouter(
    initialLocation: '/loading',
    routes: [
      GoRoute(
        path: '/loading',
        name: 'loading',
        builder: (context, state) => const LoadingScreen(),
      ),
      GoRoute(
        path: '/introduction',
        name: 'introduction',
        builder: (context, state) => const IntroductionScreen(),
        routes: [
          GoRoute(
            path: 'login',
            name: 'login',
            builder: (context, state) => const LoginScreen(),
          ),
        ],
      ),
      GoRoute(
        path: '/guardians',
        name: 'guardians',
        builder: (context, state) => const GuardiansScreen(),
      ),
      GoRoute(
        path: '/upload',
        name: 'upload',
        builder: (context, state) => const UploadStepsScreen(),
      ),
      GoRoute(
        path: '/consent-receipt',
        name: 'consentReceipt',
        builder: (context, state) => const ConsentReceiptScreen(),
      ),
      GoRoute(
        path: '/summary',
        name: 'summary',
        builder: (context, state) => const SummaryScreen(),
      ),
    ],
    refreshListenable: routerNotifier,
    redirect: routerNotifier.redirect,
  );
});

class LoadingScreen extends HookConsumerWidget {
  const LoadingScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    useEffect(() {
      ref.read(authProvider.notifier).tryAutoLogin().then((_) async {
        await ref.read(bankIdProvider.notifier).restore();
        if (!context.mounted) {
          return;
        }

        final loggedIn = ref.read(authProvider) != null;
        final hasUploadedData = ref.read(dataUploadedProvider);

        if (!loggedIn) {
          context.goNamed(
            !ref.read(bankIdProvider).begun ? 'introduction' : 'login',
          );
          return;
        }

        context.goNamed(
          ref.read(guardianRequiredProvider) == true
              ? 'guardians'
              : hasUploadedData
              ? 'summary'
              : 'upload',
        );
      });

      return null;
    }, const []);

    return const CupertinoPageScaffold(
      child: Center(child: CupertinoActivityIndicator()),
    );
  }
}
