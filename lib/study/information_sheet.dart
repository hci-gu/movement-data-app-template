import 'package:flutter/cupertino.dart';
import 'package:url_launcher/link.dart';
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
        const _EmailLink(StudyContent.supportEmail),
        const SizedBox(height: 12),
        const Text('Privacy contact', style: AppTheme.cardTitle),
        const _EmailLink(StudyContent.privacyContact),
      ],
    ),
  );
}

class _EmailLink extends StatelessWidget {
  final String address;
  const _EmailLink(this.address);

  @override
  Widget build(BuildContext context) => Link(
    uri: Uri(scheme: 'mailto', path: address),
    builder: (context, followLink) => CupertinoButton(
      padding: const EdgeInsets.symmetric(vertical: 8),
      alignment: Alignment.centerLeft,
      onPressed: followLink,
      child: Text(
        address,
        style: AppTheme.body.copyWith(
          color: AppTheme.ocean,
          decoration: TextDecoration.underline,
        ),
      ),
    ),
  );
}
