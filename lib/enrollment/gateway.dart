import 'package:dio/dio.dart';
import 'bankid/models.dart';

class GuardianRequest {
  final String personalNumber, link;
  final bool signed;
  const GuardianRequest(this.personalNumber, this.link, this.signed);
  String get identitySuffix => personalNumber.length > 4
      ? personalNumber.substring(personalNumber.length - 4)
      : personalNumber;
  factory GuardianRequest.fromJson(Map<String, dynamic> json) =>
      GuardianRequest(
        json['personalNumber'] as String? ?? '',
        json['link'] as String? ?? '',
        json['signed'] == true,
      );
}

class GuardianStatus {
  final int count;
  final bool eligible;
  final List<GuardianRequest> requests;
  const GuardianStatus({
    this.count = 0,
    this.eligible = false,
    this.requests = const [],
  });
  factory GuardianStatus.fromJson(Map<String, dynamic> json) => GuardianStatus(
    count: json['guardianCount'] as int? ?? 0,
    eligible: json['eligible'] == true,
    requests: ((json['requests'] as List?) ?? [])
        .map(
          (r) => GuardianRequest.fromJson(Map<String, dynamic>.from(r as Map)),
        )
        .toList(),
  );
}

class ConsentReceipt {
  final ConsentDocument document;
  final DateTime receivedAt;
  final String status;
  const ConsentReceipt(this.document, this.receivedAt, this.status);
  factory ConsentReceipt.fromJson(Map<String, dynamic> json) => ConsentReceipt(
    ConsentDocument.fromJson(
      Map<String, dynamic>.from(json['document'] as Map),
    ),
    DateTime.fromMillisecondsSinceEpoch((json['receivedAt'] as int) * 1000),
    json['status'] as String,
  );
}

abstract class EnrollmentGateway {
  Future<GuardianStatus> guardians();
  Future<GuardianStatus> createGuardian(String personalNumber, int count);
  Future<ConsentReceipt> receipt();
  Future<void> withdraw();
}

class HttpEnrollmentGateway implements EnrollmentGateway {
  final Dio client;
  final String token;
  HttpEnrollmentGateway(this.client, this.token);
  Options get options => Options(headers: {'Authorization': 'Bearer $token'});
  Map<String, dynamic> _json(Response<dynamic> r) =>
      Map<String, dynamic>.from(r.data as Map);
  @override
  Future<GuardianStatus> guardians() async => GuardianStatus.fromJson(
    _json(await client.get('/api/guardians', options: options)),
  );
  @override
  Future<GuardianStatus> createGuardian(
    String personalNumber,
    int count,
  ) async => GuardianStatus.fromJson(
    _json(
      await client.post(
        '/api/guardians',
        options: options,
        data: {'personalNumber': personalNumber, 'guardianCount': count},
      ),
    ),
  );
  @override
  Future<ConsentReceipt> receipt() async => ConsentReceipt.fromJson(
    _json(await client.get('/api/consent/receipt', options: options)),
  );
  @override
  Future<void> withdraw() async {
    await client.post('/api/consent/withdraw', options: options);
  }
}
