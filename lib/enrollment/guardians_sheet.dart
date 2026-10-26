import 'package:flutter/cupertino.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../ui/theme.dart';
import 'guardians_controller.dart';

class GuardiansSheet extends HookConsumerWidget {
  const GuardiansSheet({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(guardiansProvider);
    final controller = ref.read(guardiansProvider.notifier);
    final first = useTextEditingController(),
        second = useTextEditingController();
    final chosenCount = useState(1);
    final copied = useState<int?>(null);
    final count = state.status.count == 0
        ? chosenCount.value
        : state.status.count;
    return AppScaffold(
      title: 'Guardian permission',
      leading: CupertinoButton(
        padding: EdgeInsets.zero,
        onPressed: () => Navigator.pop(context),
        child: const Text('Close'),
      ),
      child: ListView(
        padding: const EdgeInsets.symmetric(vertical: 20),
        children: [
          const Text(
            'Ask a legal guardian to sign',
            style: AppTheme.sectionTitle,
          ),
          const SizedBox(height: 12),
          const Text(
            'Guardian signatures are needed before you can complete the study tasks. Generate a link for each guardian and share it with that person.',
            style: AppTheme.body,
          ),
          const SizedBox(height: 20),
          const Text('Number of guardians', style: AppTheme.body),
          const SizedBox(height: 12),
          CupertinoSlidingSegmentedControl<int>(
            groupValue: count,
            children: const {
              1: Text('1', semanticsLabel: 'One guardian'),
              2: Text('2', semanticsLabel: 'Two guardians'),
            },
            onValueChanged: (value) {
              if (!state.busy && state.status.count == 0 && value != null) {
                chosenCount.value = value;
              }
            },
          ),
          if (state.status.count > 0)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 12),
              child: Text(
                'The number is fixed after the first link is created.',
                style: AppTheme.bodyMuted,
              ),
            ),
          for (var i = 0; i < count; i++)
            Padding(
              padding: const EdgeInsets.only(top: 16),
              child: AppCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text('Guardian ${i + 1}', style: AppTheme.cardTitle),
                    const SizedBox(height: 12),
                    if (i < state.status.requests.length) ...[
                      Text(
                        'Personal number ending ${state.status.requests[i].identitySuffix}',
                        style: AppTheme.bodyMuted,
                      ),
                      Text(
                        state.status.requests[i].signed
                            ? 'Signed and verified'
                            : 'Waiting for signature',
                        style: AppTheme.body,
                      ),
                      if (!state.status.requests[i].signed) ...[
                        const SizedBox(height: 16),
                        CupertinoButton(
                          onPressed: () async {
                            await Clipboard.setData(
                              ClipboardData(
                                text: state.status.requests[i].link,
                              ),
                            );
                            if (context.mounted) copied.value = i;
                          },
                          child: Text(
                            copied.value == i
                                ? 'Link copied'
                                : 'Copy signing link',
                          ),
                        ),
                      ],
                    ] else ...[
                      CupertinoTextField(
                        controller: i == 0 ? first : second,
                        placeholder: 'YYYYMMDDXXXX',
                        keyboardType: TextInputType.number,
                        maxLength: 12,
                        enabled: !state.busy,
                      ),
                      const SizedBox(height: 16),
                      CupertinoButton(
                        onPressed:
                            state.busy || i > state.status.requests.length
                            ? null
                            : () => controller.create(
                                i == 0 ? first.text : second.text,
                                count,
                              ),
                        child: const Text('Generate signing link'),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          if (state.error != null)
            Padding(
              padding: const EdgeInsets.only(top: 16),
              child: Text(state.error!, style: AppTheme.body),
            ),
          if (state.status.eligible)
            const Padding(
              padding: EdgeInsets.only(top: 16),
              child: Text(
                'Permission confirmed. You can return to your checklist.',
                style: AppTheme.body,
              ),
            ),
          const SizedBox(height: 16),
          CupertinoButton(
            onPressed: state.busy ? null : controller.refresh,
            child: state.busy
                ? const CupertinoActivityIndicator()
                : const Text('Refresh signature status'),
          ),
        ],
      ),
    );
  }
}
