import 'package:flutter/cupertino.dart';
import 'package:go_router/go_router.dart';
import 'package:research_steps_template/app_config.dart';
import 'package:research_steps_template/screens/about.dart';
import 'package:research_steps_template/theme.dart';

class IntroductionScreen extends StatelessWidget {
  const IntroductionScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return CupertinoTabScaffold(
      tabBar: CupertinoTabBar(
        items: [
          BottomNavigationBarItem(
            icon: Icon(CupertinoIcons.house_alt),
            label: 'Home',
          ),
          BottomNavigationBarItem(
            icon: Icon(CupertinoIcons.doc_text),
            label: 'Study',
          ),
        ],
      ),
      tabBuilder: (context, index) {
        switch (index) {
          case 0:
            return AppScaffold(
              withHorizontalPadding: false,
              child: ListView(
                padding: const EdgeInsets.all(AppTheme.basePadding),
                children: [
                  AppCard(
                    backgroundColor: AppTheme.foam,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Container(
                          width: 56,
                          height: 56,
                          decoration: const BoxDecoration(
                            color: AppTheme.ocean,
                            shape: BoxShape.circle,
                          ),
                          child: const Icon(
                            CupertinoIcons.waveform_path_ecg,
                            color: CupertinoColors.white,
                            size: 28,
                          ),
                        ),
                        const SizedBox(height: 20),
                        const Text(
                          AppConfig.appName,
                          style: AppTheme.heroTitle,
                        ),
                        const SizedBox(height: 12),
                        const Text(
                          AppConfig.appDescription,
                          style: AppTheme.body,
                        ),
                        const SizedBox(height: 20),
                        CupertinoButton.filled(
                          onPressed: () {
                            context.goNamed('login');
                          },
                          child: const Text('Start Enrollment'),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 16),
                  const AppCard(
                    backgroundColor: AppTheme.sand,
                    child: _HighlightList(),
                  ),
                  const SizedBox(height: 16),
                  AppCard(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: const [
                        Text('Before you deploy', style: AppTheme.cardTitle),
                        SizedBox(height: 8),
                        Text(
                          'Replace the placeholder study information, support contacts, deployment hostnames, and app metadata in this repository.',
                          style: AppTheme.body,
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            );
          case 1:
            return const AboutScreen();
          default:
            return const SizedBox.shrink();
        }
      },
    );
  }
}

class _HighlightList extends StatelessWidget {
  const _HighlightList();

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text('Template flow', style: AppTheme.cardTitle),
        const SizedBox(height: 12),
        for (final item in AppConfig.onboardingHighlights) ...[
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Padding(
                padding: EdgeInsets.only(top: 3),
                child: Icon(
                  CupertinoIcons.arrow_right_circle_fill,
                  size: 16,
                  color: AppTheme.ocean,
                ),
              ),
              const SizedBox(width: 8),
              Expanded(child: Text(item, style: AppTheme.body)),
            ],
          ),
          const SizedBox(height: 10),
        ],
      ],
    );
  }
}
