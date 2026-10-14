import 'package:flutter/cupertino.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../enrollment/guardians_sheet.dart';
import '../session/controller.dart';
import '../session/participant_sheet.dart';
import '../steps/upload_card.dart';
import '../questionnaires/controller.dart';
import '../ui/theme.dart';
import '../ui/sheet.dart';
import 'home_state.dart';
import 'participant_actions.dart';

class HomeScreen extends ConsumerWidget {
  const HomeScreen({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(homeStateProvider);
    return AppScaffold(
      title: 'Your checklist',
      noBackButton: true,
      trailing: const ParticipantMenu(),
      child: ListView(
        padding: const EdgeInsets.symmetric(vertical: 24),
        children: [
          const Text('One task at a time', style: AppTheme.sectionTitle),
          const SizedBox(height: 12),
          const Text(
            'Complete these tasks in whichever order works for you.',
            style: AppTheme.body,
          ),
          const SizedBox(height: 24),
          if (!state.eligible)
            AppCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  const Text('Guardian approval', style: AppTheme.cardTitle),
                  const SizedBox(height: 12),
                  const Text(
                    'Your guardian must approve participation before you can begin the tasks.',
                    style: AppTheme.body,
                  ),
                  CupertinoButton.filled(
                    onPressed: () async {
                      await showAppSheet(
                        context,
                        const ParticipantSheet(child: GuardiansSheet()),
                      );
                      if (context.mounted) {
                        await ref.read(sessionProvider.notifier).refresh();
                      }
                    },
                    child: const Text('Manage guardian approval'),
                  ),
                ],
              ),
            )
          else ...[
            state.questionnaires.when(
              skipLoadingOnRefresh: false,
              loading: () => const AppCard(
                child: Row(
                  children: [
                    CupertinoActivityIndicator(),
                    SizedBox(width: 12),
                    Text('Loading questionnaires…'),
                  ],
                ),
              ),
              error: (_, _) => AppCard(
                child: Column(
                  children: [
                    const Text(
                      'Questionnaires could not be loaded. Please try again.',
                    ),
                    CupertinoButton(
                      onPressed: () =>
                          ref.invalidate(questionnaireListProvider),
                      child: const Text('Retry questionnaires'),
                    ),
                  ],
                ),
              ),
              data: (entries) => Column(
                children: [
                  for (final entry in entries)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 16),
                      child: AppCard(
                        backgroundColor: entry.completed
                            ? AppTheme.foam
                            : CupertinoColors.white,
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            Row(
                              children: [
                                Icon(
                                  entry.completed
                                      ? CupertinoIcons.check_mark_circled_solid
                                      : CupertinoIcons.circle,
                                  color: AppTheme.ocean,
                                ),
                                const SizedBox(width: 12),
                                Expanded(
                                  child: Text(
                                    entry.summary.name,
                                    style: AppTheme.cardTitle,
                                  ),
                                ),
                              ],
                            ),
                            if (entry.summary.occurrence != 'once')
                              const Padding(
                                padding: EdgeInsets.only(top: 12),
                                child: Text(
                                  'This questionnaire is not supported by this app. Please contact the study team.',
                                ),
                              )
                            else
                              CupertinoButton(
                                onPressed: () async {
                                  await context.pushNamed(
                                    'questionnaire',
                                    pathParameters: {'id': entry.summary.id},
                                  );
                                  if (context.mounted) {
                                    ref.invalidate(questionnaireListProvider);
                                  }
                                },
                                child: Text(
                                  entry.completed
                                      ? 'Review saved answers'
                                      : 'Open questionnaire',
                                ),
                              ),
                          ],
                        ),
                      ),
                    ),
                ],
              ),
            ),
            const SizedBox(height: 16),
            const StepUploadCard(),
          ],
          const SizedBox(height: 28),
          CupertinoButton.filled(
            onPressed: state.canFinish
                ? () {
                    if (ref.read(homeStateProvider).canFinish) {
                      context.goNamed('done');
                    }
                  }
                : null,
            child: const Text('Finish'),
          ),
          const SizedBox(height: 12),
          Text(
            state.canFinish
                ? 'All tasks are complete. You can finish or review your answers.'
                : 'Finish becomes available when every task is complete.',
            style: AppTheme.bodyMuted,
          ),
        ],
      ),
    );
  }
}
