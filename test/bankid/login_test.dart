import 'package:flutter/cupertino.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/enrollment/bankid/controller.dart';
import 'package:research_steps_template/enrollment/bankid/models.dart';
import 'package:research_steps_template/enrollment/login_screen.dart';
import 'fakes.dart';

void main() {
  testWidgets(
    'consent review gates both signing choices and QR fits a small phone',
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
      expect(find.text(document.text), findsOneWidget);
      expect(gateway.starts, isEmpty);
      CupertinoButton button(String title) => tester.widget<CupertinoButton>(
        find.widgetWithText(CupertinoButton, title),
      );
      expect(button('Signera med BankID').onPressed, isNull);
      expect(button('Använd BankID på en annan enhet').onPressed, isNull);
      await tester.tap(find.byType(CupertinoSwitch));
      await tester.pumpAndSettle();
      expect(button('Signera med BankID').onPressed, isNotNull);
      gateway.result = const BankIdAttempt(
        id: 'qr1',
        secret: 'secret',
        expiresAt: 4102444800,
        status: 'pending',
        hintCode: 'outstandingTransaction',
        purpose: 'sign',
        mode: 'qr',
        qrData: 'bankid.token.0.hash',
        qrSecondsRemaining: 30,
      );
      await tester.ensureVisible(find.text('Använd BankID på en annan enhet'));
      await tester.tap(find.text('Använd BankID på en annan enhet'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 300));
      expect(
        find.text('Skanna koden med BankID på din andra enhet.'),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
      expect(find.text('Start over'), findsNothing);
      // The QR and its actions must stay reachable on a short phone with large
      // text, even after changing size while the request is active.
      tester.view.physicalSize = const Size(320, 568);
      tester.platformDispatcher.textScaleFactorTestValue = 2;
      addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
      await tester.pump();
      await tester.ensureVisible(find.text('Avbryt signering'));
      await tester.pumpAndSettle();
      expect(find.text('Avbryt signering').hitTestable(), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.tap(find.text('Avbryt signering'));
      await tester.pumpAndSettle();
      expect(gateway.cancels, 1);
      expect(find.text('Du avbröt signeringen.'), findsOneWidget);
      await tester.ensureVisible(find.text('Läs samtycket och försök igen'));
      await tester.tap(find.text('Läs samtycket och försök igen'));
      await tester.pumpAndSettle();
      expect(store.value, isNull);
      expect(
        tester.widget<CupertinoSwitch>(find.byType(CupertinoSwitch)).value,
        false,
      );
      expect(button('Signera med BankID').onPressed, isNull);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(
        const SizedBox(),
      ); // Dispose provider and polling timer.
    },
  );
}
