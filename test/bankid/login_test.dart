import 'package:flutter/cupertino.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/bankid/controller.dart';
import 'package:research_steps_template/bankid/models.dart';
import 'package:research_steps_template/screens/login.dart';
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
      expect(button('Sign with BankID').onPressed, isNull);
      expect(button('Use BankID on another device').onPressed, isNull);
      await tester.tap(find.byType(CupertinoSwitch));
      await tester.pumpAndSettle();
      expect(button('Sign with BankID').onPressed, isNotNull);
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
      await tester.ensureVisible(find.text('Use BankID on another device'));
      await tester.tap(find.text('Use BankID on another device'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 300));
      expect(
        find.text('Scan this code with BankID on your other device.'),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(
        const SizedBox(),
      ); // Dispose provider and polling timer.
    },
  );
}
