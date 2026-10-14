import 'package:flutter/cupertino.dart';

class QuestionnaireTextField extends StatefulWidget {
  final Object? value;
  final String label, placeholder;
  final bool readOnly, integer;
  final ValueChanged<Object?> onChanged;
  const QuestionnaireTextField({
    super.key,
    required this.value,
    required this.label,
    required this.placeholder,
    required this.onChanged,
    this.readOnly = false,
    this.integer = false,
  });
  @override
  State<QuestionnaireTextField> createState() => _QuestionnaireTextFieldState();
}

class _QuestionnaireTextFieldState extends State<QuestionnaireTextField> {
  late final TextEditingController controller = TextEditingController(
    text: widget.value?.toString() ?? '',
  );
  @override
  void didUpdateWidget(covariant QuestionnaireTextField oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.value != widget.value &&
        widget.value?.toString() != controller.text) {
      final text = widget.value?.toString() ?? '';
      controller.value = TextEditingValue(
        text: text,
        selection: TextSelection.collapsed(offset: text.length),
      );
    }
  }

  @override
  void dispose() {
    controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Semantics(
    label: widget.label,
    child: CupertinoTextField(
      controller: controller,
      readOnly: widget.readOnly,
      placeholder: widget.readOnly && widget.value == null
          ? 'Ej besvarad'
          : widget.placeholder,
      keyboardType: widget.integer
          ? const TextInputType.numberWithOptions(signed: true)
          : TextInputType.multiline,
      minLines: 1,
      maxLines: widget.integer ? 1 : null,
      clearButtonMode: widget.readOnly
          ? OverlayVisibilityMode.never
          : OverlayVisibilityMode.editing,
      padding: const EdgeInsets.all(12),
      onChanged: (raw) => widget.onChanged(
        raw.isEmpty
            ? null
            : widget.integer
            ? int.tryParse(raw) ?? raw
            : raw,
      ),
    ),
  );
}
