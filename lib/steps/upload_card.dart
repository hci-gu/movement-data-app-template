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
import 'movement.dart';
import 'package:health/health.dart';
import '../questionnaires/controller.dart';
import '../questionnaires/scoliosis.dart';

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
    final forms = ref.watch(questionnaireListProvider);
    final treatmentReady =
        forms.asData?.value.any(
          (entry) =>
              entry.summary.id == scoliosisQuestionnaireId && entry.completed,
        ) ==
        true;
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
                child: Text('Dela rörelsedata', style: AppTheme.cardTitle),
              ),
            ],
          ),
          const SizedBox(height: 12),
          if (!state.completed &&
              state.phase == StepsPhase.idle &&
              !treatmentReady)
            const Padding(
              padding: EdgeInsets.only(bottom: 12),
              child: Text(
                'Svara först på frågorna om behandling och behandlingsdatum. De bestämmer vilken period som hämtas.',
                style: AppTheme.body,
              ),
            ),
          if (state.completed)
            Text(
              'Inskickad ${formatDateTime(state.completedAt)}',
              style: AppTheme.body,
            )
          else if (state.busy)
            Row(
              children: [
                const CupertinoActivityIndicator(),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(switch (state.phase) {
                    StepsPhase.reading => 'Hämtar rörelsedata…',
                    StepsPhase.uploading =>
                      'Skickar ${(state.progress * 100).round()} %…',
                    _ => 'Kontrollerar inskickad data…',
                  }, style: AppTheme.body),
                ),
              ],
            )
          else ...[
            Text(switch (state.phase) {
              StepsPhase.denied =>
                'Åtkomst kunde inte ges. Kontrollera behörigheterna i Apple Hälsa och försök igen.',
              StepsPhase.empty =>
                'Inga mätningar hittades för studieperioden. Kontrollera data och behörigheter i Apple Hälsa och försök igen.',
              StepsPhase.ready =>
                '${formatInteger(state.samples.length)} mätningar är klara att granska och skicka.',
              _ =>
                state.error ??
                    'Hämta dina sex rörelsemått från ${ref.watch(healthSourceNameProvider)}. Du kan granska mätningarna innan du skickar dem.',
            }, style: AppTheme.body),
            const SizedBox(height: 16),
            if (state.samples.isNotEmpty)
              CupertinoButton.filled(
                onPressed: controller.upload,
                child: const Text('Skicka rörelsedata'),
              )
            else
              CupertinoButton.filled(
                onPressed: controller.read,
                child: Text(
                  state.phase == StepsPhase.idle
                      ? 'Hämta rörelsedata'
                      : 'Försök igen',
                ),
              ),
            if (state.phase == StepsPhase.denied)
              CupertinoButton(
                onPressed: ref.watch(openHealthSettingsProvider),
                child: const Text('Öppna inställningar'),
              ),
          ],
          if (state.samples.isNotEmpty && !state.busy)
            CupertinoButton(
              key: const ValueKey('show-step-details'),
              onPressed: () => showAppSheet(
                context,
                ParticipantSheet(child: _StepDetails(state)),
              ),
              child: const Text('Granska rörelsedata'),
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
    final dates = state.samples.expand((p) => [p.dateFrom, p.dateTo]).toList()
      ..sort();
    return AppScaffold(
      title: 'Din rörelsedata',
      leading: CupertinoButton(
        padding: EdgeInsets.zero,
        onPressed: () => Navigator.pop(context),
        child: const Text('Stäng'),
      ),
      child: ListView(
        padding: const EdgeInsets.symmetric(vertical: 20),
        children: [
          Text(
            formatDateRange(
              dates.isEmpty ? null : dates.first,
              dates.isEmpty ? null : dates.last,
            ),
            style: AppTheme.cardTitle,
          ),
          const SizedBox(height: 16),
          Text(
            '${formatInteger(state.samples.length)} mätningar från Apple Hälsa. Endast mätningar inom studieperioden skickas.',
            style: AppTheme.body,
          ),
          const SizedBox(height: 24),
          if (summary.dailyTotals.isNotEmpty) ...[
            const Text('Steg per dag', style: AppTheme.cardTitle),
            const SizedBox(height: 12),
            StepPreviewChart(data: summary.recentTotals()),
            Text(
              '${formatInteger(summary.averageDailySteps)} steg per dag i genomsnitt över ${summary.daysWithData} dagar med stegdata.',
              style: AppTheme.bodyMuted,
            ),
          ],
          const SizedBox(height: 24),
          for (final type in movementTypes) ...[
            _MeasureSummary(
              type: type,
              samples: state.samples.where((p) => p.type == type).toList(),
            ),
            const SizedBox(height: 16),
          ],
          const SizedBox(height: 24),
          const Text('Datakällor', style: AppTheme.cardTitle),
          for (final source
              in state.samples
                  .map((p) => p.sourceName)
                  .where((s) => s.isNotEmpty)
                  .toSet())
            Text(source, style: AppTheme.body),
        ],
      ),
    );
  }
}

class _MeasureSummary extends StatelessWidget {
  final HealthDataType type;
  final List<HealthDataPoint> samples;
  const _MeasureSummary({required this.type, required this.samples});
  @override
  Widget build(BuildContext context) {
    final values =
        samples
            .map((p) => (p.value as NumericHealthValue).numericValue.toDouble())
            .toList()
          ..sort();
    final unit = switch (type) {
      HealthDataType.STEPS => 'steg',
      HealthDataType.WALKING_SPEED => 'm/s',
      HealthDataType.WALKING_ASYMMETRY_PERCENTAGE ||
      HealthDataType.WALKING_DOUBLE_SUPPORT_PERCENTAGE => '%',
      _ => 'm',
    };
    // HealthKit percentages are fractions, whereas meters and counts retain
    // their native units. The uploaded raw values are unchanged.
    final factor = unit == '%' ? 100 : 1;
    String display(double value) => (value * factor)
        .toStringAsFixed(type == HealthDataType.STEPS ? 0 : 2)
        .replaceAll('.', ',');
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(movementLabel(type), style: AppTheme.cardTitle),
          const SizedBox(height: 8),
          Text(
            values.isEmpty
                ? 'Inga tillgängliga mätningar.'
                : '${formatInteger(values.length)} mätningar\n${display(values.first)}–${display(values.last)} $unit per mätning\nMedelvärde: ${display(values.reduce((a, b) => a + b) / values.length)} $unit',
            style: AppTheme.body,
          ),
        ],
      ),
    );
  }
}
