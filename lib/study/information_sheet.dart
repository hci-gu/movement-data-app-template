import 'package:flutter/cupertino.dart';
import '../ui/theme.dart';
import 'content.dart';

class StudyInformationSheet extends StatelessWidget {
  const StudyInformationSheet({super.key});
  @override
  Widget build(BuildContext context) => AppScaffold(
    title: 'Study information',
    leading: CupertinoButton(
      padding: EdgeInsets.zero,
      onPressed: () => Navigator.pop(context),
      child: const Text('Close'),
    ),
    child: ListView(
      padding: const EdgeInsets.symmetric(vertical: 20),
      children: [
        for (final section in StudyContent.information) ...[
          Text(section.$1, style: AppTheme.sectionTitle),
          const SizedBox(height: 12),
          Text(section.$2, style: AppTheme.body),
          const SizedBox(height: 24),
        ],
        const Text('Support', style: AppTheme.cardTitle),
        const Text(StudyContent.supportEmail, style: AppTheme.body),
        const SizedBox(height: 12),
        const Text('Privacy contact', style: AppTheme.cardTitle),
        const Text(StudyContent.privacyContact, style: AppTheme.body),
      ],
    ),
  );
}
