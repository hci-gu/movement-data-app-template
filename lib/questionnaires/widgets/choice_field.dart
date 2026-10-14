import 'package:flutter/cupertino.dart';
import '../models.dart';

class ChoiceQuestionField extends StatelessWidget {
  final QuestionDefinition question;
  final Object? value;
  final bool readOnly;
  final ValueChanged<Object?> onChanged;
  const ChoiceQuestionField({
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
      for (final option in question.options)
        Semantics(
          selected: value == option.value,
          inMutuallyExclusiveGroup: true,
          child: CupertinoButton(
            alignment: Alignment.centerLeft,
            padding: const EdgeInsets.symmetric(vertical: 12),
            onPressed: readOnly ? null : () => onChanged(option.value),
            child: Row(
              children: [
                Icon(
                  value == option.value
                      ? CupertinoIcons.checkmark_circle_fill
                      : CupertinoIcons.circle,
                ),
                const SizedBox(width: 12),
                Expanded(child: Text(option.label)),
              ],
            ),
          ),
        ),
      if (value != null && !readOnly)
        CupertinoButton(
          onPressed: () => onChanged(null),
          child: const Text('Rensa val'),
        ),
      if (value == null && readOnly) const Text('Ej besvarad'),
    ],
  );
}
