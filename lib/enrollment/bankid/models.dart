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
  final String? launchUrl, nonce;
  final int expiresAt;
  final int? qrSecondsRemaining;
  final Map<String, dynamic>? grant;
  const BankIdAttempt({
    required this.id,
    required this.secret,
    this.status = 'pending',
    this.hintCode = '',
    this.purpose = 'participant',
    this.mode = 'sameDevice',
    required this.expiresAt,
    this.launchUrl,
    this.nonce,
    this.qrSecondsRemaining,
    this.grant,
  });
  String get authorization => '$id.$secret';
  bool get pending => status == 'pending';
  bool get accepted => status == 'accepted';
  bool get canExtendQR => pending && mode == 'qr' && qrSecondsRemaining == 0;
  bool expired(DateTime now) => now.millisecondsSinceEpoch ~/ 1000 >= expiresAt;
  factory BankIdAttempt.fromJson(Map<String, dynamic> json, String secret) =>
      BankIdAttempt(
        id: json['id'] as String,
        secret: secret,
        expiresAt: json['expiresAt'] as int,
        status: json['status'] as String? ?? 'pending',
        hintCode: json['hintCode'] as String? ?? '',
        purpose: json['purpose'] as String? ?? 'participant',
        mode: json['mode'] as String? ?? 'sameDevice',
        launchUrl: json['launchUrl'] as String?,
        nonce: json['nonce'] as String?,
        qrSecondsRemaining: json['qrSecondsRemaining'] as int?,
        grant: json['grant'] == null
            ? null
            : Map<String, dynamic>.from(json['grant'] as Map),
      );
  // Only the credential/reference survives an app restart. Fetch all results from the server.
  Map<String, dynamic> toStorage() => {
    'id': id,
    'secret': secret,
    'expiresAt': expiresAt + 600,
  };
}

String bankIdMessage(BankIdAttempt order) {
  if (order.accepted) {
    return 'Din identitet har bekräftats. Fortsätt till studien.';
  }
  const messages = {
    'outstandingTransaction': 'Öppna BankID.',
    'noClient': 'Öppna BankID för att fortsätta.',
    'started':
        'Söker efter ditt BankID. Kontrollera att du har ett giltigt BankID i appen.',
    'userSign': 'Granska förfrågan och bekräfta i BankID.',
    'userMrtd': 'Följ instruktionerna i BankID för att bekräfta din identitet.',
    'processing': 'Din förfrågan behandlas. Vänta en stund.',
    'expiredTransaction': 'Förfrågan har gått ut. Starta en ny BankID-förfrågan.',
    'certificateErr':
        'Ditt BankID kunde inte användas. Kontrollera att det är giltigt.',
    'userCancel':
        'Appen har avbrutit förfrågan. Avbryt även i BankID om det fortfarande är öppet.',
    'cancelled': 'Appen har avbrutit förfrågan.',
    'ageIneligible': 'Studien vänder sig till personer mellan 10 och 30 år.',
    'invalidIdentity':
        'Din identitet kunde inte bekräftas. Starta en ny förfrågan.',
    'startFailed':
        'BankID kunde inte startas. Kontrollera att appen är installerad och försök igen.',
    'notSupportedByUserApp': 'Uppdatera BankID och försök igen.',
    'alreadyInProgress':
        'En annan BankID-förfrågan pågår. Avsluta den innan du försöker igen.',
    'consentChanged':
        'Samtycket har ändrats. Läs den nya versionen innan du bekräftar.',
    'riskRejected':
        'Förfrågan kunde inte godkännas. Kontakta forskargruppen om det händer igen.',
    'sessionExpired': 'Din BankID-förfrågan har gått ut. Börja om.',
    'unknownResult':
        'Resultatet kunde inte bekräftas. Ingen åtkomst beviljades. Starta en ny förfrågan.',
    'invalidEvidence':
        'Identifieringen kunde inte verifieras. Starta en ny förfrågan.',
    'temporarilyUnavailable': 'BankID är tillfälligt otillgängligt. Försök igen senare.',
    'certificateUnavailable':
        'Tjänsten är tillfälligt otillgänglig. Försök igen senare.',
  };
  return messages[order.hintCode] ??
      (order.pending
          ? 'Din förfrågan behandlas. Vänta en stund.'
          : 'Förfrågan kunde inte slutföras. Försök igen.');
}
