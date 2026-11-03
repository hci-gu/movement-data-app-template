import 'package:flutter/cupertino.dart';
import 'package:go_router/go_router.dart';
import '../ui/theme.dart';
import 'participant_actions.dart';

class DoneScreen extends StatelessWidget {
  const DoneScreen({super.key});
  @override
  Widget build(BuildContext context) => AppScaffold(
    title: 'Klart',
    noBackButton: true,
    trailing: const ParticipantMenu(),
    child: AppScrollBody(
      centerContent: true,
      children: [
        const Icon(
          CupertinoIcons.check_mark_circled_solid,
          size: 72,
          color: AppTheme.ocean,
        ),
        const SizedBox(height: 24),
        const Text(
          'Allt är inskickat',
          textAlign: TextAlign.center,
          style: AppTheme.heroTitle,
        ),
        const SizedBox(height: 12),
        const Text(
          'Dina svar och din rörelsedata har tagits emot. Du kan läsa dina svar och ditt samtycke via checklistan.',
          textAlign: TextAlign.center,
          style: AppTheme.body,
        ),
        const SizedBox(height: 24),
        CupertinoButton.filled(
          onPressed: () => context.goNamed('home'),
          child: const Text('Visa checklistan'),
        ),
      ],
    ),
  );
}
