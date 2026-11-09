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
      title: 'Identify with BankID',
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
            const Text('Verify your identity', style: AppTheme.sectionTitle),
            const SizedBox(height: 12),
            const Text(
              'Use BankID to sign in. You will then review the study consent and choose whether to participate.',
              style: AppTheme.body,
            ),
            const SizedBox(height: 24),
            CupertinoButton.filled(
              onPressed: state.busy
                  ? null
                  : () => controller.start('sameDevice'),
              child: const Text('Open BankID'),
            ),
            const SizedBox(height: 12),
            CupertinoButton(
              onPressed: state.busy ? null : () => controller.start('qr'),
              child: const Text('Use BankID on another device'),
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
                        ? 'Identity verified'
                        : 'Continue with BankID',
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
                      'After confirming your identity in BankID, return to this app.',
                      style: AppTheme.bodyMuted,
                    ),
                  ],
                  if (order.pending && order.mode == 'qr') ...[
                    const SizedBox(height: 20),
                    const Text(
                      'Scan this code with BankID on your other device.',
                      style: AppTheme.body,
                    ),
                    const SizedBox(height: 12),
                    Center(
                      child: order.canExtendQR
                          ? const Text(
                              'This QR code has expired.',
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
                                  'BankID QR code. Scan with the BankID app on your other device.',
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
                child: const Text('Continue'),
              ),
            if (order.pending && order.mode == 'sameDevice')
              CupertinoButton.filled(
                onPressed: state.busy ? null : controller.openBankId,
                child: const Text('Open BankID'),
              ),
            if (order.canExtendQR)
              CupertinoButton.filled(
                onPressed: state.busy ? null : controller.extendQR,
                child: const Text('Show a new QR code'),
              ),
            if (order.pending)
              CupertinoButton(
                onPressed: state.busy ? null : controller.cancel,
                child: const Text('Stop this request'),
              ),
            if (!order.pending && !order.accepted)
              CupertinoButton.filled(
                onPressed: state.busy ? null : controller.restart,
                child: const Text('Try again'),
              ),
            if (state.error != null)
              CupertinoButton(
                onPressed: state.busy ? null : controller.refresh,
                child: const Text('Check request status'),
              ),
          ],
        ],
      ),
    );
  }
}
