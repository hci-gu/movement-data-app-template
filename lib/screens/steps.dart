import 'package:flutter/cupertino.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:research_steps_template/app_config.dart';
import 'package:research_steps_template/state/auth.dart';
import 'package:research_steps_template/state/health.dart';
import 'package:research_steps_template/storage.dart';
import 'package:research_steps_template/theme.dart';
import 'package:research_steps_template/utils.dart';
import 'package:research_steps_template/widgets/step_preview_chart.dart';

class UploadStepsScreen extends HookConsumerWidget {
  const UploadStepsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isUploading = useState(false);
    final refreshKey = useState(0);
    final initializeFuture = useMemoized(
      () => HealthManager().initialize(forceRefresh: refreshKey.value > 0),
      [refreshKey.value],
    );

    return AppScaffold(
      title: 'Connect Apple Health',
      noBackButton: true,
      withHorizontalPadding: false,
      trailing: CupertinoButton(
        padding: EdgeInsets.zero,
        onPressed: () => context.pushNamed('consentReceipt'),
        child: const Icon(
          CupertinoIcons.doc_text,
          semanticLabel: 'Signed consent and withdrawal',
        ),
      ),
      child: FutureBuilder<void>(
        future: HealthManager().ongoingUpload ?? initializeFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting ||
              isUploading.value) {
            return const _CenteredStatus(
              title: 'Preparing upload',
              message:
                  'Reading Apple Health step data and preparing the dataset for transfer.',
              loading: true,
            );
          }

          final manager = HealthManager();
          final summary = manager.summary;

          if (!manager.isAuthorized) {
            return _PermissionRequest(
              authorizationFailed: manager.authorizationFailed,
              onRetry: () {
                manager.reset();
                refreshKey.value++;
              },
            );
          }

          if (!summary.hasData) {
            return _NoDataState(
              onRetry: () {
                manager.reset();
                refreshKey.value++;
              },
            );
          }

          final userId = ref.read(authProvider)?.record.id;

          return ListView(
            padding: const EdgeInsets.all(AppTheme.basePadding),
            children: [
              AppCard(
                backgroundColor: AppTheme.foam,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text('Dataset preview', style: AppTheme.sectionTitle),
                    const SizedBox(height: 12),
                    Text(
                      'The template currently requests ${AppConfig.requestedDataLabel.toLowerCase()} from ${formatDate(AppConfig.importStartDate)} until today.',
                      style: AppTheme.body,
                    ),
                    const SizedBox(height: 16),
                    _MetricGrid(summary: summary),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              AppCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      'Recent daily totals',
                      style: AppTheme.cardTitle,
                    ),
                    const SizedBox(height: 8),
                    const Text(
                      'Use this quick view to confirm the imported Apple Health history looks plausible before uploading.',
                      style: AppTheme.bodyMuted,
                    ),
                    const SizedBox(height: 16),
                    StepPreviewChart(data: summary.recentTotals()),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              if (summary.sources.isNotEmpty)
                AppCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text('Detected sources', style: AppTheme.cardTitle),
                      const SizedBox(height: 12),
                      for (final source in summary.sources) ...[
                        Text(source, style: AppTheme.body),
                        const SizedBox(height: 6),
                      ],
                    ],
                  ),
                ),
              if (summary.sources.isNotEmpty) const SizedBox(height: 16),
              CupertinoButton.filled(
                onPressed: userId == null
                    ? null
                    : () async {
                        isUploading.value = true;
                        final success = await manager.uploadLatestData(userId);

                        if (success) {
                          ref.read(dataUploadedProvider.notifier).state = true;
                          await Storage().setHasUploadedData(true);
                          await Storage().storeLastUploadAt(DateTime.now());
                          if (context.mounted) {
                            context.goNamed('summary');
                          }
                        } else if (context.mounted) {
                          await _showErrorDialog(
                            context,
                            'Upload failed',
                            'The dataset could not be uploaded. Check your API configuration and try again.',
                          );
                        }

                        if (context.mounted) {
                          isUploading.value = false;
                        }
                      },
                child: const Text('Upload Step Data'),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _PermissionRequest extends StatelessWidget {
  final bool authorizationFailed;
  final VoidCallback onRetry;

  const _PermissionRequest({
    required this.authorizationFailed,
    required this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(AppTheme.basePadding),
      children: [
        AppCard(
          backgroundColor: AppTheme.sand,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                authorizationFailed
                    ? 'Access not granted'
                    : 'Authorize Apple Health',
                style: AppTheme.sectionTitle,
              ),
              const SizedBox(height: 12),
              Text(
                authorizationFailed
                    ? 'Apple Health access was denied. Re-run authorization or open the system settings if the dialog no longer appears.'
                    : 'Grant read-only access to Apple Health step count data so the template can prepare a research upload.',
                style: AppTheme.body,
              ),
              const SizedBox(height: 16),
              CupertinoButton.filled(
                onPressed: onRetry,
                child: const Text('Request Access'),
              ),
              const SizedBox(height: 8),
              CupertinoButton(
                onPressed: () {
                  openAppSettings();
                },
                child: const Text('Open Settings'),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _NoDataState extends StatelessWidget {
  final VoidCallback onRetry;

  const _NoDataState({required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(AppTheme.basePadding),
      children: [
        AppCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text('No step data found', style: AppTheme.sectionTitle),
              const SizedBox(height: 12),
              const Text(
                'Apple Health access is enabled, but the device did not return any step samples for the configured import window.',
                style: AppTheme.body,
              ),
              const SizedBox(height: 12),
              const Text(
                'If this is unexpected, verify that the device contains step history in Apple Health and that the requested data type is enabled.',
                style: AppTheme.bodyMuted,
              ),
              const SizedBox(height: 16),
              CupertinoButton.filled(
                onPressed: onRetry,
                child: const Text('Refresh Data'),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _CenteredStatus extends StatelessWidget {
  final String title;
  final String message;
  final bool loading;

  const _CenteredStatus({
    required this.title,
    required this.message,
    this.loading = false,
  });

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppTheme.basePadding * 2),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            if (loading) const CupertinoActivityIndicator(),
            if (loading) const SizedBox(height: 16),
            Text(
              title,
              style: AppTheme.sectionTitle,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 12),
            Text(message, style: AppTheme.body, textAlign: TextAlign.center),
          ],
        ),
      ),
    );
  }
}

class _MetricGrid extends StatelessWidget {
  final StepImportSummary summary;

  const _MetricGrid({required this.summary});

  @override
  Widget build(BuildContext context) {
    final entries = [
      ('Coverage', formatDateRange(summary.earliestDate, summary.latestDate)),
      ('Days with data', formatInteger(summary.daysWithData)),
      ('Raw samples', formatInteger(summary.rawSampleCount)),
      ('Average daily steps', formatInteger(summary.averageDailySteps)),
    ];

    return Wrap(
      spacing: 12,
      runSpacing: 12,
      children: [
        for (final entry in entries)
          SizedBox(
            width: 150,
            child: AppCard(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(entry.$1, style: AppTheme.bodyMuted),
                  const SizedBox(height: 6),
                  Text(entry.$2, style: AppTheme.cardTitle),
                ],
              ),
            ),
          ),
      ],
    );
  }
}

Future<void> _showErrorDialog(
  BuildContext context,
  String title,
  String message,
) async {
  await showCupertinoDialog<void>(
    context: context,
    builder: (context) => CupertinoAlertDialog(
      title: Text(title),
      content: Text(message),
      actions: [
        CupertinoDialogAction(
          isDefaultAction: true,
          onPressed: () {
            Navigator.of(context).pop();
          },
          child: const Text('OK'),
        ),
      ],
    ),
  );
}
