import 'package:flutter/cupertino.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/api.dart';
import 'package:research_steps_template/bankid/models.dart';
import 'package:research_steps_template/state/auth.dart';
import 'package:research_steps_template/theme.dart';

class ConsentReceiptScreen extends HookConsumerWidget {
  const ConsentReceiptScreen({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final future = useMemoized(() => Api().consentReceipt());
    final busy = useState(false);
    final error = useState<String?>(null);
    return AppScaffold(
      title: 'Signed consent',
      child: FutureBuilder<Map<String, dynamic>>(
        future: future,
        builder: (context, snapshot) {
          if (!snapshot.hasData) {
            return Center(
              child: snapshot.hasError
                  ? const Text('The consent receipt could not be loaded.')
                  : const CupertinoActivityIndicator(),
            );
          }
          final data = snapshot.data!;
          final document = ConsentDocument.fromJson(
            Map<String, dynamic>.from(data['document'] as Map),
          );
          final time = DateTime.fromMillisecondsSinceEpoch(
            (data['receivedAt'] as int) * 1000,
          ).toLocal();
          return ListView(
            children: [
              Text(document.title, style: AppTheme.sectionTitle),
              const SizedBox(height: 12),
              Text(
                'Version ${document.version}\nSigned: $time\nStatus: ${data['status']}',
                style: AppTheme.bodyMuted,
              ),
              const SizedBox(height: 16),
              AppCard(child: Text(document.text, style: AppTheme.body)),
              const SizedBox(height: 16),
              const Text(
                'Withdrawing consent stops future uploads. Contact the study team about data already submitted.',
                style: AppTheme.body,
              ),
              if (error.value != null) Text(error.value!, style: AppTheme.body),
              CupertinoButton(
                onPressed: busy.value || data['status'] == 'withdrawn'
                    ? null
                    : () async {
                        final confirmed = await showCupertinoDialog<bool>(
                          context: context,
                          builder: (context) => CupertinoAlertDialog(
                            title: const Text('Withdraw consent?'),
                            content: const Text(
                              'This stops future uploads and signs you out. Your signed consent record is retained.',
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
                        if (confirmed != true) return;
                        busy.value = true;
                        try {
                          await Api().withdrawConsent();
                          await ref.read(authProvider.notifier).logout();
                        } catch (_) {
                          if (context.mounted) {
                            error.value =
                                'Withdrawal could not be confirmed. Please try again or contact the study team.';
                          }
                        } finally {
                          if (context.mounted) busy.value = false;
                        }
                      },
                child: const Text('Withdraw consent'),
              ),
            ],
          );
        },
      ),
    );
  }
}
