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
        Center(
          child: SizedBox(
            width: 112,
            height: 112,
            child: ClipRRect(
              borderRadius: BorderRadius.circular(26),
              child: Image.asset(
                'assets/icon/app-icon-1024.png',
                fit: BoxFit.contain,
                semanticLabel: 'Skolios och rörelse',
              ),
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
              Text('Så deltar du', style: AppTheme.cardTitle),
              SizedBox(height: 12),
              Text(
                '1. Läs informationen och samtycket.\n2. Svara på frågor om din behandling.\n3. Granska och dela din rörelsedata.',
                style: AppTheme.body,
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),
        CupertinoButton.filled(
          onPressed: () => context.goNamed('login'),
          child: const Text('Läs samtycke och fortsätt'),
        ),
        CupertinoButton(
          key: const ValueKey('show-information'),
          onPressed: () => showAppSheet(context, const StudyInformationSheet()),
          child: const Text('Studieinformation och kontakt'),
        ),
      ],
    ),
  );
}
