import 'package:flutter/cupertino.dart';
import '../models.dart';
import 'choice_field.dart';
import 'date_field.dart';
import 'integer_field.dart';
import 'text_field.dart';

class QuestionField extends StatelessWidget {
  final QuestionDefinition question;
  final Object? value;
  final bool readOnly;
  final ValueChanged<Object?> onChanged;
  const QuestionField({
    super.key,
    required this.question,
    required this.value,
    required this.onChanged,
    this.readOnly = false,
  });
  @override
  Widget build(BuildContext context) => switch (question.type) {
    QuestionType.text => QuestionnaireTextField(
      value: value,
      label: question.text,
      placeholder: question.placeholder,
      onChanged: onChanged,
      readOnly: readOnly,
    ),
    QuestionType.integer => IntegerQuestionField(
      question: question,
      value: value,
      onChanged: onChanged,
      readOnly: readOnly,
    ),
    QuestionType.singleChoice => ChoiceQuestionField(
      question: question,
      value: value,
      onChanged: onChanged,
      readOnly: readOnly,
    ),
    QuestionType.date => DateQuestionField(
      question: question,
      value: value,
      onChanged: onChanged,
      readOnly: readOnly,
    ),
  };
}
