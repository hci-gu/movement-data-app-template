import 'package:flutter/cupertino.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/enrollment/bankid/controller.dart';
import 'package:research_steps_template/enrollment/bankid/models.dart';
import 'package:research_steps_template/enrollment/login_screen.dart';
import 'fakes.dart';

void main() {
  testWidgets(
    'authentication precedes consent and QR actions remain reachable on small phones',
    (tester) async {
      tester.view.physicalSize = const Size(320, 900);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final store = MemoryStore();
      final gateway = Gateway(store);
      final controller = BankIdController(
        gateway: gateway,
        store: store,
        launch: (_) async => true,
        onGrant: (_) async {},
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [bankIdProvider.overrideWith((_) => controller)],
          child: const CupertinoApp(home: LoginScreen()),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text(document.text), findsNothing);
      expect(find.byType(CupertinoSwitch), findsNothing);
      expect(find.text('Öppna BankID'), findsOneWidget);
      gateway.result = const BankIdAttempt(
        id: 'qr1',
        secret: 'secret',
        expiresAt: 4102444800,
        mode: 'qr',
        qrSecondsRemaining: 30,
      );
      await tester.tap(find.text('Använd BankID på en annan enhet'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 300));
      expect(find.byType(Image), findsOneWidget);
      expect(tester.takeException(), isNull);
      tester.view.physicalSize = const Size(320, 568);
      tester.platformDispatcher.textScaleFactorTestValue = 2;
      addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
      await tester.pump();
      await tester.ensureVisible(find.text('Avbryt förfrågan'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Avbryt förfrågan'));
      await tester.pumpAndSettle();
      expect(gateway.cancels, 1);
      expect(find.text('Försök igen'), findsOneWidget);
      await tester.ensureVisible(find.text('Försök igen'));
      await tester.tap(find.text('Försök igen'));
      await tester.pumpAndSettle();
      expect(store.value, isNull);
      expect(find.text('Bekräfta din identitet'), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );
}
