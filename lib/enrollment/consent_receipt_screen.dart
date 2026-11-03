import 'package:flutter/cupertino.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../ui/theme.dart';
import '../ui/formatting.dart';
import 'consent_controller.dart';

class ConsentReceiptScreen extends ConsumerWidget {
  const ConsentReceiptScreen({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(consentProvider);
    final controller = ref.read(consentProvider.notifier);
    final receipt = state.receipt;
    return AppScaffold(
      title: 'Ditt samtycke',
      child: AppScrollBody(
        centerContent: receipt == null,
        children: [
          if (state.loading)
            const Padding(
              padding: EdgeInsets.all(24),
              child: CupertinoActivityIndicator(),
            ),
          if (state.error != null) ...[
            Text(
              state.error!,
              textAlign: receipt == null ? TextAlign.center : TextAlign.start,
              style: AppTheme.body,
            ),
            if (receipt == null)
              CupertinoButton(
                onPressed: controller.load,
                child: const Text('Försök igen'),
              ),
          ],
          if (receipt != null) ...[
            Text(receipt.document.title, style: AppTheme.sectionTitle),
            const SizedBox(height: 12),
            Text(
              'Version ${receipt.document.version}\nSignerat: ${formatDateTime(receipt.receivedAt.toLocal())}\nStatus: ${receipt.status == 'withdrawn' ? 'Återkallat' : 'Signerat'}',
              style: AppTheme.bodyMuted,
            ),
            const SizedBox(height: 16),
            AppCard(child: Text(receipt.document.text, style: AppTheme.body)),
            const SizedBox(height: 16),
            const Text(
              'Om du återkallar ditt samtycke stoppas fortsatt deltagande. Kontakta forskargruppen om uppgifter som redan skickats in.',
              style: AppTheme.body,
            ),
            const SizedBox(height: 16),
            CupertinoButton(
              key: const ValueKey('withdraw-consent'),
              onPressed: state.withdrawing || receipt.status == 'withdrawn'
                  ? null
                  : () async {
                      final confirmed = await showCupertinoDialog<bool>(
                        context: context,
                        builder: (context) => CupertinoAlertDialog(
                          title: const Text('Återkalla samtycket?'),
                          content: const Text(
                            'Ditt fortsatta deltagande stoppas och du loggas ut. Dokumentationen av ditt samtycke finns kvar.',
                          ),
                          actions: [
                            CupertinoDialogAction(
                              onPressed: () => Navigator.pop(context, false),
                              child: const Text('Behåll samtycket'),
                            ),
                            CupertinoDialogAction(
                              isDestructiveAction: true,
                              onPressed: () => Navigator.pop(context, true),
                              child: const Text('Återkalla'),
                            ),
                          ],
                        ),
                      );
                      if (confirmed == true && context.mounted) {
                        await controller.withdraw();
                      }
                    },
              child: state.withdrawing
                  ? const CupertinoActivityIndicator()
                  : const Text('Återkalla samtycket'),
            ),
          ],
        ],
      ),
    );
  }
}
