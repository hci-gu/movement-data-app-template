import 'package:health/health.dart';

class StudySection {
  final String title;
  final List<String> paragraphs;

  const StudySection({required this.title, required this.paragraphs});
}

class AppConfig {
  static const appName = 'Research Steps Template';
  static const appDescription =
      'Reusable iOS research app for collecting Apple Health step data and uploading it to an API.';
  static const organizationName = 'Your Research Organisation';
  static const studyTitle = 'Template Apple Health Step Study';
  static const supportEmail = 'research@example.org';
  static const privacyContact = 'privacy@example.org';
  static const bankidReturnUrl = String.fromEnvironment(
    'BANKID_RETURN_URL',
    defaultValue: 'researchsteps://bankid/return',
  );
  static const participantIdLabel = 'Participant ID';
  static const participantIdHint = 'Example: STUDY-001';
  static const participantIdHelp =
      'Use an alphanumeric study ID. Replace this rule if your protocol requires another identifier format.';
  static const requestedDataLabel = 'Apple Health step count';
  static const summaryLookbackDays = 14;
  static const apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'https://research-steps-api.example.org',
  );

  static final participantIdPattern = RegExp(r'^[A-Z0-9][A-Z0-9_-]{3,31}$');

  static List<HealthDataType> get requestedTypes => [HealthDataType.STEPS];

  static List<HealthDataAccess> get requestedPermissions => [
    HealthDataAccess.READ,
  ];

  static DateTime get importStartDate =>
      DateTime(DateTime.now().year - 5, 1, 1);

  static String normalizeParticipantId(String value) {
    return value.trim().toUpperCase();
  }

  static bool isValidParticipantId(String value) {
    return participantIdPattern.hasMatch(normalizeParticipantId(value));
  }

  static const onboardingHighlights = [
    'Review study information and update the placeholder copy before production use.',
    'Use your study invitation and sign the consent with BankID.',
    'Request read access to Apple Health step data only.',
    'Preview the extracted dataset, then upload it to your study API.',
  ];

  static const postUploadChecklist = [
    'Confirm the API base URL and authentication model for your environment.',
    'Replace the placeholder consent, privacy, and contact information with study-approved text.',
    'Review the requested HealthKit data types before submission to the App Store or TestFlight.',
  ];

  static const studySections = [
    StudySection(
      title: 'Template Notice',
      paragraphs: [
        'This repository is a generic template. Replace every placeholder in the study description, consent copy, support contacts, deployment manifests, and app metadata before using it with participants.',
      ],
    ),
    StudySection(
      title: 'Study Purpose',
      paragraphs: [
        'Use this screen to explain the purpose of your research study, who is inviting the participant, and why Apple Health step data is relevant to your protocol.',
        'The current template is designed for retrospective extraction of step-count data from Apple Health on iPhone devices.',
      ],
    ),
    StudySection(
      title: 'What Data Is Collected',
      paragraphs: [
        'By default the app requests read-only access to Apple Health step count data and prepares it for upload to your backend.',
        'The backend stores each upload in compressed chunks and records upload metadata so later analysis can happen in a secure research environment.',
      ],
    ),
    StudySection(
      title: 'Participant Identifier',
      paragraphs: [
        'Your study invitation links your BankID identity to a study participant identifier. Step uploads use that study identifier.',
        'The backend retains encrypted identity and signing evidence, including your personal identity number. Your study consent explains who can access this information and how long it is retained.',
      ],
    ),
    StudySection(
      title: 'Privacy And Governance',
      paragraphs: [
        'Replace this section with the approved legal basis, storage duration, data protection details, and withdrawal procedure for your study.',
        'You should also list the responsible organisation, principal investigator, study sponsor if applicable, and support contacts.',
      ],
    ),
    StudySection(
      title: 'Support Contacts',
      paragraphs: [
        'Research support: research@example.org',
        'Privacy contact: privacy@example.org',
      ],
    ),
  ];
}
