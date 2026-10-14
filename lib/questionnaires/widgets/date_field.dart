import 'package:flutter/cupertino.dart';
import '../models.dart';

class DateQuestionField extends StatelessWidget {
  final QuestionDefinition question;
  final Object? value;
  final bool readOnly;
  final ValueChanged<Object?> onChanged;
  const DateQuestionField({
    super.key,
    required this.question,
    required this.value,
    required this.onChanged,
    this.readOnly = false,
  });
  String _format(DateTime d) =>
      '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}${question.dateSettings.monthOnly ? '' : '-${d.day.toString().padLeft(2, '0')}'}';
  Future<void> _pick(BuildContext context) async {
    final now = DateTime.now();
    var selected = value is String
        ? DateTime.tryParse(
                '${value!}${question.dateSettings.monthOnly ? '-01' : ''}',
              ) ??
              now
        : now;
    selected = DateTime(
      selected.year,
      selected.month,
      question.dateSettings.monthOnly ? 1 : selected.day,
    );
    final maximum = question.dateSettings.allowFuture
        ? DateTime(9999, 12, 31)
        : DateTime(now.year, now.month, now.day, 23, 59);
    if (selected.isAfter(maximum)) {
      selected = DateTime(
        now.year,
        now.month,
        question.dateSettings.monthOnly ? 1 : now.day,
      );
    }
    final result = await showCupertinoModalPopup<DateTime>(
      context: context,
      builder: (sheetContext) => Container(
        color: CupertinoColors.systemBackground.resolveFrom(sheetContext),
        child: SafeArea(
          top: false,
          child: SizedBox(
            height: 340,
            child: Column(
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    CupertinoButton(
                      onPressed: () => Navigator.pop(sheetContext),
                      child: const Text('Avbryt'),
                    ),
                    CupertinoButton(
                      onPressed: () => Navigator.pop(sheetContext, selected),
                      child: const Text('Välj'),
                    ),
                  ],
                ),
                Expanded(
                  child: CupertinoDatePicker(
                    mode: question.dateSettings.monthOnly
                        ? CupertinoDatePickerMode.monthYear
                        : CupertinoDatePickerMode.date,
                    initialDateTime: selected,
                    minimumYear: 1,
                    maximumYear: 9999,
                    minimumDate: DateTime(1),
                    maximumDate: maximum,
                    onDateTimeChanged: (date) => selected = date,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
    if (result != null && context.mounted) onChanged(_format(result));
  }

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      Semantics(
        label: question.text,
        child: CupertinoButton(
          alignment: Alignment.centerLeft,
          padding: const EdgeInsets.symmetric(vertical: 12),
          onPressed: readOnly ? null : () => _pick(context),
          child: Text(
            value?.toString() ??
                (readOnly
                    ? 'Ej besvarad'
                    : question.placeholder.isNotEmpty
                    ? question.placeholder
                    : 'Välj datum'),
          ),
        ),
      ),
      if (value != null && !readOnly)
        CupertinoButton(
          onPressed: () => onChanged(null),
          child: const Text('Rensa datum'),
        ),
    ],
  );
}
