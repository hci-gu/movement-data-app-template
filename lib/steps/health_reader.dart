import 'dart:io';
import 'package:health/health.dart';
import 'package:permission_handler/permission_handler.dart';

String get healthSourceName =>
    Platform.isIOS ? 'Apple Health' : 'your health app';

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
  NativeHealthReader([HealthFactory? health])
    : health = health ?? HealthFactory();
  @override
  Future<HealthRead> read() async {
    if (Platform.isAndroid) await Permission.activityRecognition.request();
    final authorized = await health.requestAuthorization(
      [HealthDataType.STEPS],
      permissions: [HealthDataAccess.READ],
    );
    if (!authorized) return const HealthRead(false, []);
    final now = DateTime.now();
    final samples = await health.getHealthDataFromTypes(
      DateTime(now.year - 5, 1, 1),
      now,
      [HealthDataType.STEPS],
    );
    final steps = samples.where((p) => p.type == HealthDataType.STEPS).toList()
      ..sort((a, b) => a.dateFrom.compareTo(b.dateFrom));
    return HealthRead(true, steps);
  }
}
