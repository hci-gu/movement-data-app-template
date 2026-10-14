import 'package:flutter/cupertino.dart';
import 'package:research_steps_template/theme.dart';
import '../models.dart';
import 'text_field.dart';

class IntegerQuestionField extends StatelessWidget {
  final QuestionDefinition question;
  final Object? value;
  final bool readOnly;
  final ValueChanged<Object?> onChanged;
  const IntegerQuestionField({
    super.key,
    required this.question,
    required this.value,
    required this.onChanged,
    this.readOnly = false,
  });
  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      QuestionnaireTextField(
        value: value,
        label: question.text,
        placeholder: question.placeholder,
        onChanged: onChanged,
        readOnly: readOnly,
        integer: true,
      ),
      if (question.numberSettings.unit.isNotEmpty)
        Padding(
          padding: const EdgeInsets.only(top: 6),
          child: Text(question.numberSettings.unit, style: AppTheme.bodyMuted),
        ),
    ],
  );
}
