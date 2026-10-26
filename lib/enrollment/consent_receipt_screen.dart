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
      title: 'Signed consent',
      child: ListView(
        children: [
          if (state.loading)
            const Padding(
              padding: EdgeInsets.all(24),
              child: CupertinoActivityIndicator(),
            ),
          if (state.error != null) ...[
            Text(state.error!, style: AppTheme.body),
            if (receipt == null)
              CupertinoButton(
                onPressed: controller.load,
                child: const Text('Retry'),
              ),
          ],
          if (receipt != null) ...[
            Text(receipt.document.title, style: AppTheme.sectionTitle),
            const SizedBox(height: 12),
            Text(
              'Version ${receipt.document.version}\nSigned: ${formatDateTime(receipt.receivedAt.toLocal())}\nStatus: ${receipt.status}',
              style: AppTheme.bodyMuted,
            ),
            const SizedBox(height: 16),
            AppCard(child: Text(receipt.document.text, style: AppTheme.body)),
            const SizedBox(height: 16),
            const Text(
              'Withdrawing consent stops future participation. Contact the study team about data already submitted.',
              style: AppTheme.body,
            ),
            CupertinoButton(
              key: const ValueKey('withdraw-consent'),
              onPressed: state.withdrawing || receipt.status == 'withdrawn'
                  ? null
                  : () async {
                      final confirmed = await showCupertinoDialog<bool>(
                        context: context,
                        builder: (context) => CupertinoAlertDialog(
                          title: const Text('Withdraw consent?'),
                          content: const Text(
                            'This stops future participation and signs you out. Your signed consent record is retained.',
                          ),
                          actions: [
                            CupertinoDialogAction(
                              onPressed: () => Navigator.pop(context, false),
                              child: const Text('Keep consent'),
                            ),
                            CupertinoDialogAction(
                              isDestructiveAction: true,
                              onPressed: () => Navigator.pop(context, true),
                              child: const Text('Withdraw'),
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
                  : const Text('Withdraw consent'),
            ),
          ],
        ],
      ),
    );
  }
}
