import 'package:flutter/cupertino.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/app_config.dart';
import 'package:research_steps_template/screens/about.dart';
import 'package:research_steps_template/state/auth.dart';
import 'package:research_steps_template/state/health.dart';
import 'package:research_steps_template/storage.dart';
import 'package:research_steps_template/theme.dart';
import 'package:research_steps_template/utils.dart';
import 'package:research_steps_template/widgets/step_preview_chart.dart';

class SummaryScreen extends StatelessWidget {
  const SummaryScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return CupertinoTabScaffold(
      tabBar: CupertinoTabBar(
        items: [
          BottomNavigationBarItem(
            icon: Icon(CupertinoIcons.chart_bar_square),
            label: 'Summary',
          ),
          BottomNavigationBarItem(
            icon: Icon(CupertinoIcons.doc_text),
            label: 'Study',
          ),
        ],
      ),
      tabBuilder: (context, index) {
        switch (index) {
          case 0:
            return const UploadSummaryHome();
          case 1:
            return const AboutScreen();
          default:
            return const SizedBox.shrink();
        }
      },
    );
  }
}

class UploadSummaryHome extends HookConsumerWidget {
  const UploadSummaryHome({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final initializeFuture = useMemoized(
      () => HealthManager().initialize(),
      const [],
    );

    return AppScaffold(
      title: 'Upload Summary',
      noBackButton: true,
      withHorizontalPadding: false,
      child: FutureBuilder<void>(
        future: initializeFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting &&
              !HealthManager().summary.hasData) {
            return const Center(child: CupertinoActivityIndicator());
          }

          final auth = ref.watch(authProvider);
          final participantId =
              auth?.record.getStringValue('username') ?? 'Unknown';
          final summary = HealthManager().summary;
          final lastUploadAt = Storage().getLastUploadAt();

          return ListView(
            padding: const EdgeInsets.all(AppTheme.basePadding),
            children: [
              AppCard(
                backgroundColor: AppTheme.foam,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text('Upload complete', style: AppTheme.sectionTitle),
                    const SizedBox(height: 12),
                    const Text(
                      'The participant dataset is ready for later research analysis through your configured backend.',
                      style: AppTheme.body,
                    ),
                    const SizedBox(height: 16),
                    _SummaryLine(
                      label: AppConfig.participantIdLabel,
                      value: participantId,
                    ),
                    _SummaryLine(
                      label: 'Last upload',
                      value: formatDateTime(lastUploadAt),
                    ),
                    _SummaryLine(
                      label: 'Coverage',
                      value: formatDateRange(
                        summary.earliestDate,
                        summary.latestDate,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              AppCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      'Imported step history',
                      style: AppTheme.cardTitle,
                    ),
                    const SizedBox(height: 12),
                    _SummaryLine(
                      label: 'Raw samples',
                      value: formatInteger(summary.rawSampleCount),
                    ),
                    _SummaryLine(
                      label: 'Days with data',
                      value: formatInteger(summary.daysWithData),
                    ),
                    _SummaryLine(
                      label: 'Average daily steps',
                      value: formatInteger(summary.averageDailySteps),
                    ),
                    const SizedBox(height: 16),
                    StepPreviewChart(data: summary.recentTotals()),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              AppCard(
                backgroundColor: AppTheme.sand,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text('Template checklist', style: AppTheme.cardTitle),
                    const SizedBox(height: 12),
                    for (final item in AppConfig.postUploadChecklist) ...[
                      Text(item, style: AppTheme.body),
                      const SizedBox(height: 10),
                    ],
                  ],
                ),
              ),
              const SizedBox(height: 16),
              CupertinoButton.filled(
                onPressed: () {
                  context.goNamed('upload');
                },
                child: const Text('Upload Again'),
              ),
              const SizedBox(height: 8),
              CupertinoButton(
                onPressed: () => context.pushNamed('consentReceipt'),
                child: const Text('View signed consent'),
              ),
              CupertinoButton(
                onPressed: () async {
                  await ref.read(authProvider.notifier).logout();
                  ref.read(dataUploadedProvider.notifier).state = false;
                  HealthManager().reset();
                  if (context.mounted) {
                    context.goNamed('introduction');
                  }
                },
                child: const Text('Sign Out'),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _SummaryLine extends StatelessWidget {
  final String label;
  final String value;

  const _SummaryLine({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(child: Text(label, style: AppTheme.bodyMuted)),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              value,
              style: AppTheme.body,
              textAlign: TextAlign.right,
            ),
          ),
        ],
      ),
    );
  }
}
