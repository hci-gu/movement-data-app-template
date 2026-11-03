import 'dart:io';
import 'package:health/health.dart';
import 'movement.dart';

String get healthSourceName => 'Apple Hälsa';

class HealthRead {
  final bool authorized;
  final List<HealthDataPoint> samples;
  const HealthRead(this.authorized, this.samples);
}

abstract class HealthReader {
  Future<HealthRead> read();
}

class NativeHealthReader implements HealthReader {
  final HealthFactory health;
  final Future<MovementWindow> Function() window;
  NativeHealthReader(this.window, [HealthFactory? health])
    : health = health ?? HealthFactory();
  @override
  Future<HealthRead> read() async {
    if (!Platform.isIOS) {
      throw UnsupportedError('Studien använder Apple Hälsa på iPhone.');
    }
    final period = await window();
    final authorized = await health.requestAuthorization(
      movementTypes,
      permissions: movementTypes.map((_) => HealthDataAccess.READ).toList(),
    );
    if (!authorized) return const HealthRead(false, []);
    final samples = await health.getHealthDataFromTypes(
      period.from,
      period.to,
      movementTypes,
    );
    final steps = samples.where(period.contains).toList()
      ..sort((a, b) => a.dateFrom.compareTo(b.dateFrom));
    return HealthRead(true, steps);
  }
}
