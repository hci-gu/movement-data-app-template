import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart' show Colors;
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:research_steps_template/enrollment/bankid/controller.dart';
import 'package:research_steps_template/enrollment/bankid/models.dart';
import 'package:research_steps_template/ui/theme.dart';
import '../study/information_sheet.dart';
import '../ui/sheet.dart';

class LoginScreen extends HookConsumerWidget {
  const LoginScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(bankIdProvider);
    final controller = ref.read(bankIdProvider.notifier);
    useEffect(() {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        controller.initialize();
      });
      return null;
    }, const []);
    final order = state.attempt;
    final document = state.document;
    final disabled = state.busy;
    final finishing = useRef<String?>(null);
    useEffect(() {
      if (order?.accepted == true &&
          !state.busy &&
          finishing.value != order!.id) {
        finishing.value = order.id;
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (context.mounted && controller.mounted) controller.finish();
        });
      }
      return null;
    }, [order?.id, order?.accepted, state.busy]);

    return AppScaffold(
      title: document != null ? 'Samtycke' : 'Delta i studien',
      child: AppScrollBody(
        key: ValueKey(
          order == null ? 'consent-${document?.id}' : 'bankid-${order.id}',
        ),
        centerContent: order != null || document == null,
        children: [
          if (state.error != null) ...[
            AppCard(
              backgroundColor: AppTheme.sand,
              child: Semantics(
                liveRegion: true,
                child: Text(state.error!, style: AppTheme.body),
              ),
            ),
            const SizedBox(height: 16),
          ],
          if (state.busy)
            const Padding(
              padding: EdgeInsets.all(12),
              child: CupertinoActivityIndicator(),
            ),
          if (!state.begun) ...[
            const Text('Läs samtycket', style: AppTheme.sectionTitle),
            const SizedBox(height: 12),
            const Text(
              'När du signerar samtycket med BankID skapas ditt konto eller loggas du in.',
              style: AppTheme.body,
            ),
            const SizedBox(height: 16),
            CupertinoButton.filled(
              onPressed: disabled ? null : controller.begin,
              child: const Text('Hämta samtycke'),
            ),
          ] else if (order != null) ...[
            AppCard(
              backgroundColor: order.accepted ? AppTheme.foam : Colors.white,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    order.accepted
                        ? 'Samtycket är signerat'
                        : 'Fortsätt med BankID',
                    style: AppTheme.sectionTitle,
                  ),
                  const SizedBox(height: 12),
                  Semantics(
                    liveRegion: true,
                    child: Text(bankIdMessage(order), style: AppTheme.body),
                  ),
                  if (order.accepted && document != null) ...[
                    const SizedBox(height: 16),
                    Text(document.title, style: AppTheme.cardTitle),
                    Text(
                      'Samtyckesversion ${document.version}',
                      style: AppTheme.bodyMuted,
                    ),
                  ],
                  if (order.qrData != null) ...[
                    const SizedBox(height: 20),
                    const Text(
                      'Skanna koden med BankID på din andra enhet.',
                      style: AppTheme.body,
                    ),
                    const SizedBox(height: 12),
                    Center(
                      child: Semantics(
                        label:
                            'QR-kod för BankID. Skanna med BankID på din andra enhet.',
                        child: ExcludeSemantics(
                          child: QrImageView(
                            data: order.qrData!,
                            size: 240,
                            backgroundColor: Colors.white,
                          ),
                        ),
                      ),
                    ),
                    ExcludeSemantics(
                      child: Center(
                        child: Text(
                          'Skanna inom ${order.qrSecondsRemaining} sekunder',
                          style: AppTheme.bodyMuted,
                        ),
                      ),
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 16),
            if (order.accepted)
              CupertinoButton.filled(
                onPressed: disabled ? null : controller.finish,
                child: const Text('Fortsätt'),
              ),
            if (order.pending && order.mode == 'sameDevice')
              CupertinoButton.filled(
                onPressed: disabled ? null : controller.openBankId,
                child: const Text('Öppna BankID'),
              ),
            if (order.canExtendQR)
              CupertinoButton.filled(
                onPressed: disabled ? null : controller.extendQR,
                child: const Text('Visa en ny QR-kod'),
              ),
            if (order.pending)
              CupertinoButton(
                onPressed: disabled ? null : controller.cancel,
                child: const Text('Avbryt signering'),
              ),
            if (!order.pending && !order.accepted)
              CupertinoButton.filled(
                onPressed: disabled ? null : controller.reviewAgain,
                child: const Text('Läs samtycket och försök igen'),
              ),
            if (state.error != null)
              CupertinoButton(
                onPressed: disabled ? null : controller.refresh,
                child: const Text('Kontrollera signeringen'),
              ),
          ] else if (document != null) ...[
            Text(document.title, style: AppTheme.sectionTitle),
            const SizedBox(height: 8),
            Text('Version ${document.version}', style: AppTheme.bodyMuted),
            CupertinoButton(
              key: const ValueKey('consent-information'),
              onPressed: () =>
                  showAppSheet(context, const StudyInformationSheet()),
              child: const Text('Läs studieinformationen'),
            ),
            const SizedBox(height: 16),
            AppCard(child: Text(document.text, style: AppTheme.body)),
            const SizedBox(height: 16),
            Row(
              children: [
                CupertinoSwitch(
                  value: state.reviewed,
                  onChanged: disabled ? null : controller.setReviewed,
                ),
                const SizedBox(width: 12),
                const Expanded(
                  child: Text(
                    'Jag har läst studieinformationen och samtycket ovan och vill signera.',
                    style: AppTheme.body,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 20),
            CupertinoButton.filled(
              onPressed: disabled || !state.reviewed
                  ? null
                  : () => controller.start('sameDevice'),
              child: const Text('Signera med BankID'),
            ),
            CupertinoButton(
              onPressed: disabled || !state.reviewed
                  ? null
                  : () => controller.start('qr'),
              child: const Text('Använd BankID på en annan enhet'),
            ),
          ] else ...[
            CupertinoButton.filled(
              onPressed: disabled ? null : controller.reviewAgain,
              child: const Text('Hämta samtycket'),
            ),
          ],
        ],
      ),
    );
  }
}
