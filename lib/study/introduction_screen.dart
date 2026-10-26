import 'package:flutter/cupertino.dart';
import 'package:go_router/go_router.dart';
import '../ui/theme.dart';
import '../ui/sheet.dart';
import 'content.dart';
import 'information_sheet.dart';

class IntroductionScreen extends StatelessWidget {
  const IntroductionScreen({super.key});
  @override
  Widget build(BuildContext context) => AppScaffold(
    title: StudyContent.appName,
    noBackButton: true,
    child: ListView(
      padding: const EdgeInsets.symmetric(vertical: 32),
      children: [
        const Center(
          child: SizedBox(
            width: 112,
            height: 112,
            child: Icon(
              CupertinoIcons.heart_circle,
              size: 72,
              color: AppTheme.ocean,
            ),
          ),
        ),
        const SizedBox(height: 24),
        const Text(StudyContent.title, style: AppTheme.heroTitle),
        const SizedBox(height: 16),
        const Text(StudyContent.introduction, style: AppTheme.body),
        const SizedBox(height: 24),
        const AppCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Taking part', style: AppTheme.cardTitle),
              SizedBox(height: 12),
              Text(
                '1. Read the consent and sign with BankID.\n2. Complete the tasks on your checklist.\n3. Finish when everything is submitted.',
                style: AppTheme.body,
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),
        CupertinoButton.filled(
          onPressed: () => context.goNamed('login'),
          child: const Text('Continue with BankID'),
        ),
        CupertinoButton(
          key: const ValueKey('show-information'),
          onPressed: () => showAppSheet(context, const StudyInformationSheet()),
          child: const Text('Study information and contact'),
        ),
      ],
    ),
  );
}
