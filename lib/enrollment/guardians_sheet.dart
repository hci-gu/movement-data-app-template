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
    final chosenCount = useState(2);
    final copied = useState<int?>(null);
    final count = state.status.count == 0
        ? chosenCount.value
        : state.status.count;
    return AppScaffold(
      title: 'Vårdnadshavares samtycke',
      leading: CupertinoButton(
        padding: EdgeInsets.zero,
        onPressed: () => Navigator.pop(context),
        child: const Text('Stäng'),
      ),
      child: ListView(
        padding: const EdgeInsets.symmetric(vertical: 20),
        children: [
          const Text(
            'Be dina vårdnadshavare om samtycke',
            style: AppTheme.sectionTitle,
          ),
          const SizedBox(height: 12),
          const Text(
            'Samtliga vårdnadshavare behöver samtycka innan du kan fortsätta. Skapa en länk för varje vårdnadshavare och dela den med personen.',
            style: AppTheme.body,
          ),
          const SizedBox(height: 20),
          const Text('Antal vårdnadshavare', style: AppTheme.body),
          const SizedBox(height: 12),
          CupertinoSlidingSegmentedControl<int>(
            groupValue: count,
            children: const {
              1: Text('1', semanticsLabel: 'En vårdnadshavare'),
              2: Text('2', semanticsLabel: 'Två vårdnadshavare'),
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
                'Antalet kan inte ändras när den första länken har skapats.',
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
                    Text('Vårdnadshavare ${i + 1}', style: AppTheme.cardTitle),
                    const SizedBox(height: 12),
                    if (i < state.status.requests.length) ...[
                      Text(
                        'Personnummer som slutar på ${state.status.requests[i].identitySuffix}',
                        style: AppTheme.bodyMuted,
                      ),
                      Text(
                        state.status.requests[i].approved
                            ? 'Samtycke bekräftat efter identifiering'
                            : 'Väntar på samtycke',
                        style: AppTheme.body,
                      ),
                      if (!state.status.requests[i].approved) ...[
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
                                ? 'Länken har kopierats'
                                : 'Kopiera samtyckeslänk',
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
                        child: const Text('Skapa samtyckeslänk'),
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
                'Samtycke har bekräftats. Du kan återgå till checklistan.',
                style: AppTheme.body,
              ),
            ),
          const SizedBox(height: 16),
          CupertinoButton(
            onPressed: state.busy ? null : controller.refresh,
            child: state.busy
                ? const CupertinoActivityIndicator()
                : const Text('Uppdatera status'),
          ),
        ],
      ),
    );
  }
}
