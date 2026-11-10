class Participant {
  final String id, gender;
  final bool guardianRequired, consentRequired;
  const Participant({
    required this.id,
    this.gender = 'unknown',
    this.guardianRequired = false,
    this.consentRequired = false,
  });
  bool get canParticipate => !guardianRequired && !consentRequired;
  factory Participant.fromJson(Map<String, dynamic> json) {
    final id = json['id'];
    if (id is! String || id.isEmpty) {
      throw const FormatException('Missing participant');
    }
    return Participant(
      id: id,
      gender: json['gender'] as String? ?? 'unknown',
      guardianRequired: json['guardianRequired'] == true,
      consentRequired: json['consentRequired'] == true,
    );
  }
  Map<String, dynamic> toJson() => {
    'id': id,
    'gender': gender,
    'guardianRequired': guardianRequired,
    'consentRequired': consentRequired,
  };
  Participant requiringGuardian() => Participant(
    id: id,
    gender: gender,
    consentRequired: consentRequired,
    guardianRequired: true,
  );
}

class Session {
  final String token;
  final DateTime expiresAt;
  final Participant participant;
  const Session({
    required this.token,
    required this.expiresAt,
    required this.participant,
  });
  factory Session.fromJson(Map<String, dynamic> json) {
    final token = json['token'];
    final expiry = json['expiresAt'];
    if (token is! String ||
        token.isEmpty ||
        expiry is! int ||
        json['consentRequired'] == true) {
      throw const FormatException('Invalid session grant');
    }
    return Session(
      token: token,
      expiresAt: DateTime.fromMillisecondsSinceEpoch(expiry * 1000),
      participant: Participant.fromJson(
        Map<String, dynamic>.from(json['record'] as Map),
      ),
    );
  }
  Session withParticipant(Participant value) =>
      Session(token: token, expiresAt: expiresAt, participant: value);
  Map<String, dynamic> toJson() => {
    'token': token,
    'expiresAt': expiresAt.millisecondsSinceEpoch ~/ 1000,
    'record': participant.toJson(),
  };
}
