import 'package:flutter/cupertino.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../ui/theme.dart';
import 'bankid/controller.dart';
import 'bankid/models.dart';

class LoginScreen extends HookConsumerWidget {
  const LoginScreen({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(bankIdProvider);
    final controller = ref.read(bankIdProvider.notifier);
    useEffect(() {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (controller.mounted) controller.initialize();
      });
      return null;
    }, const []);
    final order = state.attempt;
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
      title: 'Identifiera dig med BankID',
      child: AppScrollBody(
        centerContent: true,
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
          if (order == null) ...[
            const Text('Bekräfta din identitet', style: AppTheme.sectionTitle),
            const SizedBox(height: 12),
            const Text(
              'Identifiera dig med BankID. Därefter får du läsa samtycket och välja om du vill delta.',
              style: AppTheme.body,
            ),
            const SizedBox(height: 24),
            CupertinoButton.filled(
              onPressed: state.busy
                  ? null
                  : () => controller.start('sameDevice'),
              child: const Text('Öppna BankID'),
            ),
            const SizedBox(height: 12),
            CupertinoButton(
              onPressed: state.busy ? null : () => controller.start('qr'),
              child: const Text('Använd BankID på en annan enhet'),
            ),
          ] else ...[
            AppCard(
              backgroundColor: order.accepted
                  ? AppTheme.foam
                  : CupertinoColors.white,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    order.accepted
                        ? 'Identiteten är bekräftad'
                        : 'Fortsätt med BankID',
                    style: AppTheme.sectionTitle,
                  ),
                  const SizedBox(height: 12),
                  Semantics(
                    liveRegion: true,
                    child: Text(bankIdMessage(order), style: AppTheme.body),
                  ),
                  if (order.pending && order.mode == 'sameDevice') ...[
                    const SizedBox(height: 12),
                    const Text(
                      'Återvänd till appen när du har bekräftat din identitet i BankID.',
                      style: AppTheme.bodyMuted,
                    ),
                  ],
                  if (order.pending && order.mode == 'qr') ...[
                    const SizedBox(height: 20),
                    const Text(
                      'Skanna koden med BankID på din andra enhet.',
                      style: AppTheme.body,
                    ),
                    const SizedBox(height: 12),
                    Center(
                      child: order.canExtendQR
                          ? const Text(
                              'QR-koden har gått ut.',
                              style: AppTheme.bodyMuted,
                            )
                          : state.qrImage == null
                          ? const Padding(
                              padding: EdgeInsets.all(24),
                              child: CupertinoActivityIndicator(),
                            )
                          : Image.memory(
                              state.qrImage!,
                              width: 240,
                              height: 240,
                              gaplessPlayback: true,
                              semanticLabel:
                                  'BankID QR-kod. Skanna med BankID på din andra enhet.',
                            ),
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 16),
            if (order.accepted)
              CupertinoButton.filled(
                onPressed: state.busy ? null : controller.finish,
                child: const Text('Fortsätt'),
              ),
            if (order.pending && order.mode == 'sameDevice')
              CupertinoButton.filled(
                onPressed: state.busy ? null : controller.openBankId,
                child: const Text('Öppna BankID'),
              ),
            if (order.canExtendQR)
              CupertinoButton.filled(
                onPressed: state.busy ? null : controller.extendQR,
                child: const Text('Visa en ny QR-kod'),
              ),
            if (order.pending)
              CupertinoButton(
                onPressed: state.busy ? null : controller.cancel,
                child: const Text('Avbryt förfrågan'),
              ),
            if (!order.pending && !order.accepted)
              CupertinoButton.filled(
                onPressed: state.busy ? null : controller.restart,
                child: const Text('Försök igen'),
              ),
            if (state.error != null)
              CupertinoButton(
                onPressed: state.busy ? null : controller.refresh,
                child: const Text('Kontrollera status'),
              ),
          ],
        ],
      ),
    );
  }
}
