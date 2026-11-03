import 'package:health/health.dart';

const movementTypes = [
  HealthDataType.STEPS,
  HealthDataType.WALKING_STEP_LENGTH,
  HealthDataType.DISTANCE_WALKING_RUNNING,
  HealthDataType.WALKING_SPEED,
  HealthDataType.WALKING_ASYMMETRY_PERCENTAGE,
  HealthDataType.WALKING_DOUBLE_SUPPORT_PERCENTAGE,
];

String movementLabel(HealthDataType type) => switch (type) {
  HealthDataType.STEPS => 'Antal steg',
  HealthDataType.WALKING_STEP_LENGTH => 'Steglängd',
  HealthDataType.DISTANCE_WALKING_RUNNING => 'Gång- och löpsträcka',
  HealthDataType.WALKING_SPEED => 'Gånghastighet',
  HealthDataType.WALKING_ASYMMETRY_PERCENTAGE => 'Asymmetri vid gång',
  HealthDataType.WALKING_DOUBLE_SUPPORT_PERCENTAGE =>
    'Tid med båda fötterna på marken',
  _ => type.name,
};

class MovementWindow {
  final DateTime from, to;
  const MovementWindow(this.from, this.to);
  factory MovementWindow.fromJson(Map<String, dynamic> json) => MovementWindow(
    DateTime.parse(json['from'] as String),
    DateTime.parse(json['to'] as String),
  );
  bool contains(HealthDataPoint point) =>
      movementTypes.contains(point.type) &&
      !point.dateFrom.isBefore(from) &&
      !point.dateTo.isAfter(to) &&
      !point.dateTo.isBefore(point.dateFrom);
}
