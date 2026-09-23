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
    return 'Your consent has been signed and saved.';
  }
  const messages = {
    'outstandingTransaction': 'Open your BankID app.',
    'noClient': 'Open your BankID app to continue.',
    'started':
        'Searching for your BankID. Check that you have a valid BankID in the app.',
    'userSign': 'Review the request and confirm in your BankID app.',
    'userMrtd': 'Follow the instructions in BankID to confirm your identity.',
    'processing': 'Your request is being processed. Please wait.',
    'expiredTransaction': 'The request expired. Start a new BankID request.',
    'certificateErr':
        'Your BankID could not be used. Check its validity in the BankID app.',
    'userCancel': 'You cancelled the BankID request.',
    'cancelled': 'The BankID request was cancelled.',
    'startFailed':
        'BankID could not start. Check that it is installed and try again.',
    'notSupportedByUserApp': 'Update the BankID app and try again.',
    'alreadyInProgress':
        'Another BankID request is in progress. Finish it before trying again.',
    'consentChanged':
        'The consent has changed. Read the new version before signing.',
    'riskRejected':
        'This request could not be accepted. Contact the study team if it happens again.',
    'sessionExpired': 'Your BankID request expired. Start again.',
    'unknownResult':
        'We could not confirm the result. No access was granted. Start a new request.',
    'invalidEvidence':
        'The signing evidence could not be confirmed. Start a new request.',
    'temporarilyUnavailable': 'BankID is temporarily unavailable. Please wait.',
    'certificateUnavailable':
        'The service is temporarily unavailable. Please try later.',
  };
  return messages[order.hintCode] ??
      (order.pending
          ? 'Your request is being processed. Please wait.'
          : 'The request could not be completed. Please try again.');
}
