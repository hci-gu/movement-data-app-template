import 'package:flutter/cupertino.dart';
import 'package:go_router/go_router.dart';
import '../ui/theme.dart';
import 'participant_actions.dart';

class DoneScreen extends StatelessWidget {
  const DoneScreen({super.key});
  @override
  Widget build(BuildContext context) => AppScaffold(
    title: 'Complete',
    noBackButton: true,
    trailing: const ParticipantMenu(),
    child: Center(
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(
              CupertinoIcons.check_mark_circled_solid,
              size: 72,
              color: AppTheme.ocean,
            ),
            const SizedBox(height: 24),
            const Text('Thank you', style: AppTheme.heroTitle),
            const SizedBox(height: 12),
            const Text(
              'Your answers and step data have been received. Your checklist is complete.',
              textAlign: TextAlign.center,
              style: AppTheme.body,
            ),
            const SizedBox(height: 24),
            CupertinoButton.filled(
              onPressed: () => context.goNamed('home'),
              child: const Text('Review checklist'),
            ),
          ],
        ),
      ),
    ),
  );
}
