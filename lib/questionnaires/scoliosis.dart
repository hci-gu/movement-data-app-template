import '../ui/formatting.dart';

const scoliosisQuestionnaireId = 'scoliosis000001';
const treatmentQuestionId = 'scoliquestion02';
const surgeryDateQuestionId = 'scoliquestion03';
const braceDateQuestionId = 'scoliquestion04';

bool isTreatmentQuestion(String id) => const {
  treatmentQuestionId,
  surgeryDateQuestionId,
  braceDateQuestionId,
}.contains(id);

Map<String, String> validateTreatment(
  Map<String, Object?> answers,
  DateTime now,
) {
  final type = answers[treatmentQuestionId];
  if (!const ['surgery', 'full_time', 'night'].contains(type)) {
    return {
      treatmentQuestionId:
          'Välj din behandling för att bestämma perioden för rörelsedata.',
    };
  }
  final key = type == 'surgery' ? surgeryDateQuestionId : braceDateQuestionId;
  final value = answers[key];
  final date = value is String ? DateTime.tryParse(value) : null;
  if (date == null ||
      formatDate(date) != value ||
      date.year < 2020 ||
      date.year > 2027 ||
      date.isAfter(normalizeDate(now))) {
    return {key: 'Ange behandlingsdatum mellan 2020 och 2027, senast i dag.'};
  }
  return {};
}
