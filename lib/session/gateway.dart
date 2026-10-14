import 'package:dio/dio.dart';
import 'models.dart';

abstract class SessionGateway {
  Future<Participant> current(String token);
  Future<void> logout(String token);
}

class HttpSessionGateway implements SessionGateway {
  final Dio client;
  HttpSessionGateway(this.client);
  Options _options(String token) =>
      Options(headers: {'Authorization': 'Bearer $token'});
  @override
  Future<Participant> current(String token) async => Participant.fromJson(
    Map<String, dynamic>.from(
      (await client.get('/api/me', options: _options(token))).data as Map,
    ),
  );
  @override
  Future<void> logout(String token) async {
    await client.post('/api/logout', options: _options(token));
  }
}
