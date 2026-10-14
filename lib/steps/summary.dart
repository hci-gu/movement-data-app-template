import 'package:health/health.dart';
import '../ui/formatting.dart';

class DailyStepTotal {
  final DateTime date;
  final int steps;

  const DailyStepTotal({required this.date, required this.steps});
}

class StepImportSummary {
  final int rawSampleCount;
  final List<DailyStepTotal> dailyTotals;
  final Set<String> sources;

  const StepImportSummary({
    required this.rawSampleCount,
    required this.dailyTotals,
    required this.sources,
  });

  factory StepImportSummary.empty() {
    return const StepImportSummary(
      rawSampleCount: 0,
      dailyTotals: [],
      sources: {},
    );
  }

  factory StepImportSummary.fromHealthData(List<HealthDataPoint> data) {
    final totalsByDay = <DateTime, int>{};
    final sources = <String>{};

    for (final entry in data) {
      final json = entry.value.toJson();
      final rawValue = json['numericValue'];
      final numericValue = switch (rawValue) {
        num value => value.toDouble(),
        String value => double.tryParse(value) ?? 0,
        _ => 0.0,
      };

      final day = normalizeDate(entry.dateFrom);
      totalsByDay.update(
        day,
        (current) => current + numericValue.round(),
        ifAbsent: () => numericValue.round(),
      );

      if (entry.sourceName.isNotEmpty) {
        sources.add(entry.sourceName);
      } else if (entry.sourceId.isNotEmpty) {
        sources.add(entry.sourceId);
      } else if (entry.deviceId.isNotEmpty) {
        sources.add(entry.deviceId);
      }
    }

    final dailyTotals =
        totalsByDay.entries
            .map((item) => DailyStepTotal(date: item.key, steps: item.value))
            .toList()
          ..sort((left, right) => left.date.compareTo(right.date));

    return StepImportSummary(
      rawSampleCount: data.length,
      dailyTotals: dailyTotals,
      sources: sources,
    );
  }

  bool get hasData => rawSampleCount > 0 && dailyTotals.isNotEmpty;

  int get daysWithData => dailyTotals.length;

  DateTime? get earliestDate =>
      dailyTotals.isEmpty ? null : dailyTotals.first.date;

  DateTime? get latestDate =>
      dailyTotals.isEmpty ? null : dailyTotals.last.date;

  int get totalSteps => dailyTotals.fold(0, (sum, item) => sum + item.steps);

  double get averageDailySteps {
    if (dailyTotals.isEmpty) {
      return 0;
    }

    return totalSteps / dailyTotals.length;
  }

  List<DailyStepTotal> recentTotals([int limit = 14]) {
    if (dailyTotals.length <= limit) {
      return dailyTotals;
    }

    return dailyTotals.sublist(dailyTotals.length - limit);
  }
}
