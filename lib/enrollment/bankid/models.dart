class ConsentDocument {
  final String id, version, title, text, documentHash;
  const ConsentDocument({
    required this.id,
    required this.version,
    required this.title,
    required this.text,
    required this.documentHash,
  });
  factory ConsentDocument.fromJson(Map<String, dynamic> json) =>
      ConsentDocument(
        id: json['id'] as String,
        version: json['version'] as String,
        title: json['title'] as String,
        text: json['text'] as String,
        documentHash: json['documentHash'] as String,
      );
  Map<String, dynamic> toJson() => {
    'id': id,
    'version': version,
    'title': title,
    'text': text,
    'documentHash': documentHash,
  };
}

class BankIdAttempt {
  final String id, secret, status, hintCode, purpose, mode;
  final String? launchUrl, nonce, qrData, signatureId;
  final int expiresAt;
  final int? qrSecondsRemaining;
  final bool pickedUp, consentRequired;
  final ConsentDocument? document;
  final Map<String, dynamic>? grant;
  const BankIdAttempt({
    required this.id,
    required this.secret,
    this.status = 'pending',
    this.hintCode = '',
    this.purpose = 'sign',
    this.mode = 'sameDevice',
    required this.expiresAt,
    this.launchUrl,
    this.nonce,
    this.qrData,
    this.signatureId,
    this.qrSecondsRemaining,
    this.pickedUp = false,
    this.consentRequired = false,
    this.document,
    this.grant,
  });
  String get authorization => '$id.$secret';
  bool get pending => status == 'pending';
  bool get accepted => status == 'accepted';
  bool get canExtendQR =>
      pending && mode == 'qr' && !pickedUp && qrSecondsRemaining == 0;
  bool expired(DateTime now) => now.millisecondsSinceEpoch ~/ 1000 >= expiresAt;
  factory BankIdAttempt.fromJson(Map<String, dynamic> json, String secret) =>
      BankIdAttempt(
        id: json['id'] as String,
        secret: secret,
        expiresAt: json['expiresAt'] as int,
        status: json['status'] as String? ?? 'pending',
        hintCode: json['hintCode'] as String? ?? '',
        purpose: json['purpose'] as String? ?? 'sign',
        mode: json['mode'] as String? ?? 'sameDevice',
        launchUrl: json['launchUrl'] as String?,
        nonce: json['nonce'] as String?,
        qrData: json['qrData'] as String?,
        signatureId: json['signatureId'] as String?,
        qrSecondsRemaining: json['qrSecondsRemaining'] as int?,
        pickedUp: json['pickedUp'] == true,
        consentRequired: json['consentRequired'] == true,
        document: json['document'] == null
            ? null
            : ConsentDocument.fromJson(
                Map<String, dynamic>.from(json['document'] as Map),
              ),
        grant: json['grant'] == null
            ? null
            : Map<String, dynamic>.from(json['grant'] as Map),
      );
  // Only the credential/reference survives an app restart. Fetch all results from the server.
  Map<String, dynamic> toStorage() => {
    'id': id,
    'secret': secret,
    'expiresAt': expiresAt,
  };
}

String bankIdMessage(BankIdAttempt order) {
  if (order.accepted) {
    return 'Ditt samtycke har signerats och sparats.';
  }
  const messages = {
    'ageIneligible':
        'Studien omfattar personer mellan 10 och 30 år. Kontakta forskargruppen om du har frågor.',
    'outstandingTransaction': 'Öppna BankID.',
    'noClient': 'Öppna BankID för att fortsätta.',
    'started':
        'Söker efter ditt BankID. Kontrollera att du har ett giltigt BankID.',
    'userSign': 'Läs och bekräfta förfrågan i BankID.',
    'userMrtd': 'Följ instruktionerna i BankID för att bekräfta din identitet.',
    'processing': 'Din förfrågan behandlas. Vänta en stund.',
    'expiredTransaction': 'Förfrågan har gått ut. Starta en ny signering.',
    'certificateErr':
        'Ditt BankID kunde inte användas. Kontrollera giltigheten i BankID.',
    'userCancel': 'Du avbröt signeringen.',
    'cancelled': 'Signeringen avbröts.',
    'startFailed':
        'BankID kunde inte startas. Kontrollera att appen är installerad och försök igen.',
    'notSupportedByUserApp': 'Uppdatera BankID och försök igen.',
    'alreadyInProgress': 'En annan BankID-förfrågan pågår. Avsluta den först.',
    'consentChanged':
        'Samtycket har ändrats. Läs den nya versionen innan du signerar.',
    'riskRejected':
        'Förfrågan kunde inte godkännas. Kontakta forskargruppen om det händer igen.',
    'sessionExpired': 'Signeringen har gått ut. Börja om.',
    'unknownResult':
        'Resultatet kunde inte bekräftas. Ingen åtkomst beviljades. Starta en ny signering.',
    'invalidEvidence':
        'Signeringen kunde inte verifieras. Starta en ny förfrågan.',
    'temporarilyUnavailable':
        'BankID är tillfälligt otillgängligt. Vänta en stund.',
    'certificateUnavailable':
        'Tjänsten är tillfälligt otillgänglig. Försök senare.',
  };
  return messages[order.hintCode] ??
      (order.pending
          ? 'Din förfrågan behandlas. Vänta en stund.'
          : 'Förfrågan kunde inte slutföras. Försök igen.');
}
