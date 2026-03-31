import 'package:flutter/cupertino.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:research_steps_template/app_config.dart';

class ParticipantIdFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(
    TextEditingValue oldValue,
    TextEditingValue newValue,
  ) {
    final sanitized = newValue.text.toUpperCase().replaceAll(
      RegExp(r'[^A-Z0-9_-]'),
      '',
    );

    return newValue.copyWith(
      text: sanitized,
      selection: TextSelection.collapsed(offset: sanitized.length),
    );
  }
}

class ParticipantIdInput extends HookWidget {
  final TextEditingController controller;

  const ParticipantIdInput({super.key, required this.controller});

  @override
  Widget build(BuildContext context) {
    final focusNode = useFocusNode();
    final errorMessage = useState<String?>(null);

    useEffect(() {
      void validate() {
        final value = AppConfig.normalizeParticipantId(controller.text);
        if (value.isEmpty || AppConfig.isValidParticipantId(value)) {
          errorMessage.value = null;
        }
      }

      controller.addListener(validate);
      return () => controller.removeListener(validate);
    }, [controller]);

    useEffect(() {
      void onFocusChange() {
        if (focusNode.hasFocus) {
          return;
        }

        final value = AppConfig.normalizeParticipantId(controller.text);
        if (value.isEmpty) {
          errorMessage.value = 'Required';
        } else if (!AppConfig.isValidParticipantId(value)) {
          errorMessage.value = 'Use 4-32 characters: A-Z, 0-9, "_" or "-"';
        } else {
          errorMessage.value = null;
        }
      }

      focusNode.addListener(onFocusChange);
      return () => focusNode.removeListener(onFocusChange);
    }, [focusNode]);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        CupertinoTextField(
          controller: controller,
          focusNode: focusNode,
          keyboardType: TextInputType.text,
          textCapitalization: TextCapitalization.characters,
          inputFormatters: [ParticipantIdFormatter()],
          prefix: const Padding(
            padding: EdgeInsets.only(left: 12),
            child: Text(
              AppConfig.participantIdLabel,
              style: TextStyle(
                fontWeight: FontWeight.w600,
                color: CupertinoColors.systemGrey,
              ),
            ),
          ),
          placeholder: AppConfig.participantIdHint,
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 16),
        ),
        const SizedBox(height: 8),
        Text(
          errorMessage.value ?? AppConfig.participantIdHelp,
          style: TextStyle(
            fontSize: 13,
            color: errorMessage.value == null
                ? CupertinoColors.systemGrey
                : CupertinoColors.destructiveRed,
          ),
        ),
      ],
    );
  }
}
