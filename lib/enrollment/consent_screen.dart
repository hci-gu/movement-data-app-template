import 'package:flutter/cupertino.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../session/controller.dart';
import '../ui/theme.dart';
import 'consent_review_controller.dart';

class ConsentScreen extends ConsumerWidget {
  const ConsentScreen({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(consentReviewProvider);
    final controller = ref.read(consentReviewProvider.notifier);
    final document = state.document;
    return AppScaffold(
      title: 'Study consent',
      leading: CupertinoButton(
        padding: EdgeInsets.zero,
        onPressed: state.saving
            ? null
            : ref.read(sessionProvider.notifier).logout,
        child: const Text('Leave'),
      ),
      child: AppScrollBody(
        centerContent: document == null,
        children: [
          if (state.loading) const Center(child: CupertinoActivityIndicator()),
          if (state.error != null) ...[
            Text(
              state.error!,
              style: AppTheme.body,
              textAlign: document == null ? TextAlign.center : TextAlign.start,
            ),
            const SizedBox(height: 16),
            if (document == null)
              CupertinoButton(
                onPressed: controller.load,
                child: const Text('Retry'),
              ),
          ],
          if (document != null) ...[
            const Text(
              'Your identity has been verified with BankID. Read the information below before deciding whether to participate.',
              style: AppTheme.body,
            ),
            const SizedBox(height: 24),
            Text(document.title, style: AppTheme.sectionTitle),
            const SizedBox(height: 8),
            Text('Version ${document.version}', style: AppTheme.bodyMuted),
            const SizedBox(height: 16),
            AppCard(child: Text(document.text, style: AppTheme.body)),
            const SizedBox(height: 16),
            Row(
              children: [
                CupertinoSwitch(
                  value: state.reviewed,
                  onChanged: state.saving ? null : controller.setReviewed,
                ),
                const SizedBox(width: 12),
                const Expanded(
                  child: Text(
                    'I have read the information and consent to participate.',
                    style: AppTheme.body,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 20),
            CupertinoButton.filled(
              onPressed: state.saving || !state.reviewed
                  ? null
                  : controller.accept,
              child: state.saving
                  ? const CupertinoActivityIndicator()
                  : const Text('Accept consent and continue'),
            ),
            const SizedBox(height: 12),
            CupertinoButton(
              onPressed: state.saving
                  ? null
                  : ref.read(sessionProvider.notifier).logout,
              child: const Text('Leave without consenting'),
            ),
          ],
        ],
      ),
    );
  }
}
