class Environment {
  static const apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://192.168.10.144:8090',
  );
  static const bankIdReturnUrl = String.fromEnvironment(
    'BANKID_RETURN_URL',
    defaultValue: 'researchsteps://bankid/return',
  );
}
