import 'package:flutter/cupertino.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:permission_handler/permission_handler.dart';
import '../ui/theme.dart';
import '../ui/formatting.dart';
import '../ui/sheet.dart';
import '../session/participant_sheet.dart';
import 'controller.dart';
import 'health_reader.dart';
import 'step_preview_chart.dart';

final openHealthSettingsProvider = Provider<Future<bool> Function()>(
  (ref) => openAppSettings,
);

final healthSourceNameProvider = Provider<String>((ref) => healthSourceName);

class StepUploadCard extends ConsumerWidget {
  const StepUploadCard({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(stepsProvider);
    final controller = ref.read(stepsProvider.notifier);
    return AppCard(
      backgroundColor: state.completed ? AppTheme.foam : CupertinoColors.white,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Icon(
                state.completed
                    ? CupertinoIcons.check_mark_circled_solid
                    : CupertinoIcons.circle,
                color: AppTheme.ocean,
              ),
              const SizedBox(width: 12),
              const Expanded(
                child: Text('Share step data', style: AppTheme.cardTitle),
              ),
            ],
          ),
          const SizedBox(height: 12),
          if (state.completed)
            Text(
              'Uploaded ${formatDateTime(state.completedAt)}',
              style: AppTheme.body,
            )
          else if (state.busy)
            Row(
              children: [
                const CupertinoActivityIndicator(),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(switch (state.phase) {
                    StepsPhase.reading => 'Reading step history…',
                    StepsPhase.uploading =>
                      'Uploading ${(state.progress * 100).round()}%…',
                    _ => 'Checking upload status…',
                  }, style: AppTheme.body),
                ),
              ],
            )
          else ...[
            Text(switch (state.phase) {
              StepsPhase.denied =>
                'Access was not granted. Allow access in your health settings, then try again.',
              StepsPhase.empty =>
                'No step data was found. Check your health app and try again.',
              StepsPhase.ready =>
                '${formatInteger(state.summary.rawSampleCount)} samples across ${state.summary.daysWithData} days are ready to share.',
              _ =>
                state.error ??
                    'Read your step history from ${ref.watch(healthSourceNameProvider)}, review it, then upload when you are ready.',
            }, style: AppTheme.body),
            const SizedBox(height: 12),
            if (state.samples.isNotEmpty)
              CupertinoButton.filled(
                onPressed: controller.upload,
                child: const Text('Upload step data'),
              )
            else
              CupertinoButton.filled(
                onPressed: controller.read,
                child: Text(
                  state.phase == StepsPhase.idle
                      ? 'Read step history'
                      : 'Try again',
                ),
              ),
            if (state.phase == StepsPhase.denied)
              CupertinoButton(
                onPressed: ref.watch(openHealthSettingsProvider),
                child: const Text('Open settings'),
              ),
          ],
          if (state.samples.isNotEmpty && !state.busy)
            CupertinoButton(
              key: const ValueKey('show-step-details'),
              onPressed: () => showAppSheet(
                context,
                ParticipantSheet(child: _StepDetails(state)),
              ),
              child: const Text('View step details'),
            ),
        ],
      ),
    );
  }
}

class _StepDetails extends StatelessWidget {
  final StepsState state;
  const _StepDetails(this.state);
  @override
  Widget build(BuildContext context) {
    final summary = state.summary;
    return AppScaffold(
      title: 'Step details',
      leading: CupertinoButton(
        padding: EdgeInsets.zero,
        onPressed: () => Navigator.pop(context),
        child: const Text('Close'),
      ),
      child: ListView(
        padding: const EdgeInsets.symmetric(vertical: 20),
        children: [
          Text(
            formatDateRange(summary.earliestDate, summary.latestDate),
            style: AppTheme.cardTitle,
          ),
          const SizedBox(height: 16),
          Text(
            '${formatInteger(summary.rawSampleCount)} samples\n${summary.daysWithData} days with data\n${formatInteger(summary.averageDailySteps)} average daily steps',
            style: AppTheme.body,
          ),
          const SizedBox(height: 24),
          StepPreviewChart(data: summary.recentTotals()),
          const SizedBox(height: 24),
          const Text('Sources', style: AppTheme.cardTitle),
          for (final source in summary.sources)
            Text(source, style: AppTheme.body),
        ],
      ),
    );
  }
}
