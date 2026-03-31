import 'package:flutter/cupertino.dart';
import 'package:research_steps_template/app_config.dart';
import 'package:research_steps_template/theme.dart';

class AboutScreen extends StatelessWidget {
  const AboutScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return AppScaffold(
      title: 'Study Information',
      withHorizontalPadding: false,
      child: ListView(
        padding: const EdgeInsets.all(AppTheme.basePadding),
        children: [
          AppCard(
            backgroundColor: AppTheme.sand,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: const [
                Text('Replace Before Production', style: AppTheme.sectionTitle),
                SizedBox(height: 12),
                Text(
                  'This screen is intentionally generic. It exists so a new study can start from a usable structure without carrying over prior study-specific claims.',
                  style: AppTheme.body,
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          for (final section in AppConfig.studySections) ...[
            AppCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(section.title, style: AppTheme.cardTitle),
                  const SizedBox(height: 12),
                  for (final paragraph in section.paragraphs) ...[
                    Text(paragraph, style: AppTheme.body),
                    const SizedBox(height: 10),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 16),
          ],
        ],
      ),
    );
  }
}
