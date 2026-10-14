import 'package:flutter/cupertino.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'controller.dart';

/// Dismiss participant details when the session that opened them ends.
class ParticipantSheet extends ConsumerWidget {
  final Widget child;
  const ParticipantSheet({super.key, required this.child});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.listen(sessionProvider.select((s) => s?.token), (previous, next) {
      if (previous == next) return;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (context.mounted && ModalRoute.of(context)?.isCurrent == true) {
          Navigator.pop(context);
        }
      });
    });
    return ref.watch(sessionProvider) == null ? const SizedBox.shrink() : child;
  }
}
