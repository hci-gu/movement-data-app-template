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

class BankIdOrder {
  final String id, status, hintCode, purpose, mode;
  final String? launchUrl, nonce, qrData, signatureId;
  final int? qrSecondsRemaining;
  final bool pickedUp;
  const BankIdOrder({
    required this.id,
    required this.status,
    required this.hintCode,
    required this.purpose,
    required this.mode,
    this.launchUrl,
    this.nonce,
    this.qrData,
    this.signatureId,
    this.qrSecondsRemaining,
    this.pickedUp = false,
  });
  factory BankIdOrder.fromJson(Map<String, dynamic> json) => BankIdOrder(
    id: json['id'] as String,
    status: json['status'] as String,
    hintCode: json['hintCode'] as String? ?? '',
    purpose: json['purpose'] as String,
    mode: json['mode'] as String,
    launchUrl: json['launchUrl'] as String?,
    nonce: json['nonce'] as String?,
    qrData: json['qrData'] as String?,
    signatureId: json['signatureId'] as String?,
    qrSecondsRemaining: json['qrSecondsRemaining'] as int?,
    pickedUp: json['pickedUp'] as bool? ?? false,
  );
  bool get pending => const [
    'creating',
    'pending',
    'collecting',
    'collected',
    'cancelling',
  ].contains(status);
  bool get accepted => status == 'accepted';
  bool get canExtendQR =>
      pending && mode == 'qr' && !pickedUp && qrSecondsRemaining == 0;
}

// Stored in Keychain before network calls, so interrupted requests can be retried
// with the same secret and idempotency key after a cold start.
class BankIdFlow {
  final String id,
      clientSecret,
      kind,
      invitationCode,
      requestKey,
      mode,
      orderId,
      nonce;
  final int expiresAt;
  final ConsentDocument? document;
  const BankIdFlow({
    required this.id,
    required this.clientSecret,
    required this.kind,
    required this.expiresAt,
    this.invitationCode = '',
    this.requestKey = '',
    this.mode = 'sameDevice',
    this.orderId = '',
    this.nonce = '',
    this.document,
  });
  String get authorization => '$id.$clientSecret';
  bool expired(DateTime now) => now.millisecondsSinceEpoch ~/ 1000 >= expiresAt;
  BankIdFlow copyWith({
    String? id,
    int? expiresAt,
    String? requestKey,
    String? mode,
    String? orderId,
    String? nonce,
    ConsentDocument? document,
    bool clearDocument = false,
  }) => BankIdFlow(
    id: id ?? this.id,
    clientSecret: clientSecret,
    kind: kind,
    expiresAt: expiresAt ?? this.expiresAt,
    invitationCode: invitationCode,
    requestKey: requestKey ?? this.requestKey,
    mode: mode ?? this.mode,
    orderId: orderId ?? this.orderId,
    nonce: nonce ?? this.nonce,
    document: clearDocument ? null : document ?? this.document,
  );
  factory BankIdFlow.fromJson(Map<String, dynamic> json) => BankIdFlow(
    id: json['id'] as String,
    clientSecret: json['clientSecret'] as String,
    kind: json['kind'] as String,
    expiresAt: json['expiresAt'] as int,
    invitationCode: json['invitationCode'] as String? ?? '',
    requestKey: json['requestKey'] as String? ?? '',
    mode: json['mode'] as String? ?? 'sameDevice',
    orderId: json['orderId'] as String? ?? '',
    nonce: json['nonce'] as String? ?? '',
    document: json['document'] == null
        ? null
        : ConsentDocument.fromJson(
            Map<String, dynamic>.from(json['document'] as Map),
          ),
  );
  Map<String, dynamic> toJson() => {
    'id': id,
    'clientSecret': clientSecret,
    'kind': kind,
    'expiresAt': expiresAt,
    'invitationCode': invitationCode,
    'requestKey': requestKey,
    'mode': mode,
    'orderId': orderId,
    'nonce': nonce,
    'document': document?.toJson(),
  };
}

String bankIdMessage(BankIdOrder order) {
  if (order.accepted) {
    return order.purpose == 'sign'
        ? 'Your consent has been signed and saved.'
        : 'Your identity has been confirmed.';
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
    'wrongSigner': 'The signer does not match this study invitation.',
    'identityConflict':
        'This identity is already linked to another participant. Contact the study team.',
    'participantNotFound':
        'No participant account was found. Use your study invitation to enroll.',
    'riskRejected':
        'This request could not be accepted. Contact the study team if it happens again.',
    'sessionExpired': 'Your enrollment session expired. Start again.',
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
