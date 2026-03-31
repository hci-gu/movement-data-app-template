import 'package:flutter/cupertino.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:research_steps_template/app_config.dart';
import 'package:research_steps_template/theme.dart';

class ConsentModal extends HookWidget {
  const ConsentModal({super.key});

  @override
  Widget build(BuildContext context) {
    final consentAccepted = useState(false);

    return CupertinoAlertDialog(
      title: const Text('Study Consent'),
      content: Column(
        children: [
          Text(
            'Review and replace this placeholder consent text before inviting participants.',
            style: AppTheme.bodyMuted,
            textAlign: TextAlign.left,
          ),
          const SizedBox(height: 12),
          for (final statement in AppConfig.consentStatements) ...[
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Padding(
                  padding: EdgeInsets.only(top: 4),
                  child: Icon(
                    CupertinoIcons.check_mark_circled_solid,
                    size: 16,
                    color: AppTheme.ocean,
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    statement,
                    style: AppTheme.body,
                    textAlign: TextAlign.left,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 10),
          ],
          const SizedBox(height: 8),
          CupertinoSwitch(
            value: consentAccepted.value,
            onChanged: (value) {
              consentAccepted.value = value;
            },
          ),
        ],
      ),
      actions: [
        CupertinoDialogAction(
          isDestructiveAction: true,
          onPressed: () {
            Navigator.of(context).pop(false);
          },
          child: const Text('Cancel'),
        ),
        CupertinoDialogAction(
          isDefaultAction: true,
          onPressed: consentAccepted.value
              ? () {
                  Navigator.of(context).pop(true);
                }
              : null,
          child: const Text('Continue'),
        ),
      ],
    );
  }
}
