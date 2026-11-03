class Environment {
  static const apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://127.0.0.1:8091',
  );
  static const bankIdReturnUrl = String.fromEnvironment(
    'BANKID_RETURN_URL',
    defaultValue: 'researchsteps://bankid/return',
  );
}
