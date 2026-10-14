import 'package:flutter/cupertino.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/theme.dart';
import 'controller.dart';
import 'gateway.dart';

class QuestionnaireListScreen extends ConsumerWidget {
  final bool inTab;
  const QuestionnaireListScreen({super.key, this.inTab = false});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final list = ref.watch(questionnaireListProvider);
    return AppScaffold(
      title: 'Formulär',
      noBackButton: inTab,
      withHorizontalPadding: false,
      trailing: CupertinoButton(
        padding: EdgeInsets.zero,
        onPressed: () => ref.invalidate(questionnaireListProvider),
        child: const Icon(
          CupertinoIcons.refresh,
          semanticLabel: 'Uppdatera formulär',
        ),
      ),
      child: list.when(
        loading: () => const Center(child: CupertinoActivityIndicator()),
        error: (error, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(questionnaireError(error)),
                CupertinoButton(
                  onPressed: () => ref.invalidate(questionnaireListProvider),
                  child: const Text('Försök igen'),
                ),
              ],
            ),
          ),
        ),
        data: (items) => items.isEmpty
            ? const Center(child: Text('Det finns inga formulär att visa.'))
            : ListView(
                padding: const EdgeInsets.all(AppTheme.basePadding),
                children: [
                  for (final entry in items)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 16),
                      child: AppCard(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            Text(entry.summary.name, style: AppTheme.cardTitle),
                            const SizedBox(height: 8),
                            Text(
                              entry.completed
                                  ? 'Inskickat'
                                  : entry.summary.occurrence == 'once'
                                  ? 'Ej besvarat'
                                  : 'Återkommande formulär stöds inte ännu.',
                              style: AppTheme.bodyMuted,
                            ),
                            CupertinoButton(
                              onPressed: entry.summary.occurrence != 'once'
                                  ? null
                                  : () => context.pushNamed(
                                      'questionnaire',
                                      pathParameters: {'id': entry.summary.id},
                                    ),
                              child: Text(
                                entry.completed
                                    ? 'Visa svar'
                                    : 'Öppna formulär',
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                ],
              ),
      ),
    );
  }
}
