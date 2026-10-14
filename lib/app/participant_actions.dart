import 'package:flutter/cupertino.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../session/controller.dart';
import '../session/participant_sheet.dart';
import '../study/information_sheet.dart';
import '../ui/sheet.dart';

class ParticipantMenu extends ConsumerWidget {
  const ParticipantMenu({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) => CupertinoButton(
    padding: EdgeInsets.zero,
    child: const Icon(
      CupertinoIcons.ellipsis_circle,
      semanticLabel: 'Study and account',
    ),
    onPressed: () async {
      final action = await showCupertinoModalPopup<String>(
        context: context,
        builder: (context) => ParticipantSheet(
          child: CupertinoActionSheet(
            actions: [
              for (final item in const [
                ('information', 'Study information'),
                ('consent', 'Signed consent'),
                ('logout', 'Sign out'),
              ])
                CupertinoActionSheetAction(
                  onPressed: () => Navigator.pop(context, item.$1),
                  child: Text(item.$2),
                ),
            ],
            cancelButton: CupertinoActionSheetAction(
              onPressed: () => Navigator.pop(context),
              child: const Text('Cancel'),
            ),
          ),
        ),
      );
      if (!context.mounted || ref.read(sessionProvider) == null) return;
      switch (action) {
        case 'information':
          await showAppSheet(
            context,
            const ParticipantSheet(child: StudyInformationSheet()),
          );
        case 'consent':
          context.pushNamed('consent-receipt');
        case 'logout':
          await ref.read(sessionProvider.notifier).logout();
      }
    },
  );
}
