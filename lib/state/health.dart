import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:health/health.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:research_steps_template/api.dart';
import 'package:research_steps_template/app_config.dart';
import 'package:research_steps_template/utils.dart';

export 'package:health/health.dart';

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

  List<DailyStepTotal> recentTotals([
    int limit = AppConfig.summaryLookbackDays,
  ]) {
    if (dailyTotals.length <= limit) {
      return dailyTotals;
    }

    return dailyTotals.sublist(dailyTotals.length - limit);
  }
}

class HealthManager {
  List<HealthDataPoint> data = [];
  StepImportSummary summary = StepImportSummary.empty();
  HealthFactory health = HealthFactory();
  bool isAuthorized = false;
  bool triedToAuthorize = false;
  bool authorizationFailed = false;
  Future<void>? ongoingUpload;

  void reset() {
    data = [];
    summary = StepImportSummary.empty();
    isAuthorized = false;
    triedToAuthorize = false;
    authorizationFailed = false;
  }

  Future<void> initialize({bool forceRefresh = false}) async {
    if (!forceRefresh && data.isNotEmpty) {
      return;
    }

    if (forceRefresh) {
      data = [];
      summary = StepImportSummary.empty();
    }

    await authorize();
    if (!isAuthorized) {
      if (kDebugMode) {
        print('HealthManager: authorization was not granted.');
      }
      return;
    }

    final healthData = await health.getHealthDataFromTypes(
      AppConfig.importStartDate,
      DateTime.now(),
      AppConfig.requestedTypes,
    );

    data =
        healthData.where((entry) => entry.type == HealthDataType.STEPS).toList()
          ..sort((left, right) => left.dateFrom.compareTo(right.dateFrom));
    summary = StepImportSummary.fromHealthData(data);
  }

  Future<bool> uploadLatestData(String userId) async {
    try {
      ongoingUpload = Api().uploadData(userId, data);
      await ongoingUpload;
      ongoingUpload = null;
      return true;
    } catch (error) {
      if (kDebugMode) {
        print('Error uploading latest data: $error');
      }
      ongoingUpload = null;
      return false;
    }
  }

  Future<bool> authorize() async {
    if (Platform.isAndroid) {
      await Permission.activityRecognition.request();
    }

    isAuthorized = await health.requestAuthorization(
      AppConfig.requestedTypes,
      permissions: AppConfig.requestedPermissions,
    );

    triedToAuthorize = true;
    authorizationFailed = !isAuthorized;
    return isAuthorized;
  }

  static final HealthManager _instance = HealthManager._internal();

  factory HealthManager() {
    return _instance;
  }

  HealthManager._internal();
}
