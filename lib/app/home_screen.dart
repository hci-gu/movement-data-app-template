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
      title: 'Din checklista',
      noBackButton: true,
      trailing: const ParticipantMenu(),
      child: ListView(
        padding: const EdgeInsets.symmetric(vertical: 24),
        children: [
          const Text(
            'Börja med frågorna om din behandling. Granska sedan din rörelsedata innan du skickar den.',
            style: AppTheme.body,
          ),
          const SizedBox(height: 24),
          if (!state.eligible)
            AppCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  const Text(
                    'Vårdnadshavares samtycke',
                    style: AppTheme.cardTitle,
                  ),
                  const SizedBox(height: 12),
                  const Text(
                    'Om du är under 15 år behöver samtliga vårdnadshavare samtycka innan du kan fortsätta. Du kan tacka nej även om de tackar ja.',
                    style: AppTheme.body,
                  ),
                  const SizedBox(height: 16),
                  CupertinoButton.filled(
                    key: const ValueKey('show-guardians'),
                    onPressed: () async {
                      await showAppSheet(
                        context,
                        const ParticipantSheet(child: GuardiansSheet()),
                      );
                      if (context.mounted) {
                        await ref.read(sessionProvider.notifier).refresh();
                      }
                    },
                    child: const Text('Bjud in vårdnadshavare'),
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
                    Expanded(child: Text('Hämtar formulär…')),
                  ],
                ),
              ),
              error: (_, _) => AppCard(
                child: Column(
                  children: [
                    const Text('Formulären kunde inte hämtas. Försök igen.'),
                    const SizedBox(height: 16),
                    CupertinoButton(
                      onPressed: () =>
                          ref.invalidate(questionnaireListProvider),
                      child: const Text('Hämta formulär igen'),
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
                                  'Formuläret kan inte visas i appen. Kontakta forskargruppen.',
                                ),
                              )
                            else ...[
                              const SizedBox(height: 16),
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
                                      ? 'Visa sparade svar'
                                      : 'Svara på frågor',
                                ),
                              ),
                            ],
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
            child: const Text('Avsluta'),
          ),
          const SizedBox(height: 12),
          Text(
            state.canFinish
                ? 'Alla uppgifter är klara. Du kan avsluta eller läsa dina svar.'
                : 'Du kan avsluta när frågorna och rörelsedatan är inskickade.',
            style: AppTheme.bodyMuted,
          ),
        ],
      ),
    );
  }
}
