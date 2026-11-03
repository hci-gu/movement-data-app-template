import 'package:flutter_test/flutter_test.dart';
import 'package:health/health.dart';
import 'package:research_steps_template/steps/movement.dart';
import 'package:research_steps_template/steps/summary.dart';

HealthDataPoint point(
  HealthDataType type,
  double value, {
  DateTime? from,
  DateTime? to,
}) => HealthDataPoint(
  NumericHealthValue(value),
  type,
  type == HealthDataType.STEPS ? HealthDataUnit.COUNT : HealthDataUnit.METER,
  from ?? DateTime.utc(2025, 1, 1),
  to ?? DateTime.utc(2025, 1, 1, 1),
  PlatformType.IOS,
  'device',
  'source',
  'iPhone',
);
void main() {
  test('movement preview never adds other quantities to step totals', () {
    final summary = StepImportSummary.fromHealthData([
      point(HealthDataType.STEPS, 100),
      point(HealthDataType.WALKING_STEP_LENGTH, .7),
      point(HealthDataType.DISTANCE_WALKING_RUNNING, 2000),
    ]);
    expect(summary.totalSteps, 100);
    expect(summary.daysWithData, 1);
  });
  test(
    'native filter rejects nonprotocol types and samples crossing the window',
    () {
      final window = MovementWindow(
        DateTime.utc(2025, 1, 1),
        DateTime.utc(2026),
      );
      expect(window.contains(point(HealthDataType.STEPS, 100)), true);
      expect(
        window.contains(point(HealthDataType.WALKING_STEADINESS, .9)),
        false,
      );
      expect(
        window.contains(
          point(HealthDataType.STEPS, 100, from: DateTime.utc(2024, 12, 31)),
        ),
        false,
      );
      expect(
        window.contains(
          point(HealthDataType.STEPS, 100, to: DateTime.utc(2026, 1, 2)),
        ),
        false,
      );
    },
  );
}
