import 'package:flutter/cupertino.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/api.dart';
import 'package:research_steps_template/bankid/gateway.dart';
import 'package:research_steps_template/state/auth.dart';
import 'package:research_steps_template/theme.dart';

class GuardiansScreen extends HookConsumerWidget {
  const GuardiansScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final first = useTextEditingController();
    final second = useTextEditingController();
    final count = useState(1);
    final savedCount = useState(0);
    final requests = useState<List<Map<String, dynamic>>>([]);
    final busy = useState(false);
    final error = useState<String?>(null);
    final copied = useState<int?>(null);

    Future<void> apply(Map<String, dynamic> status) async {
      final configured = status['guardianCount'] as int? ?? 0;
      savedCount.value = configured;
      if (configured > 0) count.value = configured;
      requests.value = (status['requests'] as List? ?? [])
          .map((value) => Map<String, dynamic>.from(value as Map))
          .toList();
      if (status['eligible'] == true) {
        ref.read(guardianRequiredProvider.notifier).state = false;
      }
    }

    Future<void> refresh() async {
      if (busy.value) return;
      busy.value = true;
      error.value = null;
      try {
        await apply(await Api().guardianStatus());
      } catch (e) {
        error.value = bankIdError(e);
      } finally {
        busy.value = false;
      }
    }

    Future<void> create(int index) async {
      if (busy.value) return;
      final personalNumber = (index == 0 ? first.text : second.text).replaceAll(
        RegExp(r'[^0-9]'),
        '',
      );
      if (personalNumber.length != 12) {
        error.value = 'Enter a 12-digit personal number (YYYYMMDDXXXX).';
        return;
      }
      busy.value = true;
      error.value = null;
      try {
        await apply(
          await Api().createGuardianRequest(personalNumber, count.value),
        );
      } catch (e) {
        error.value = bankIdError(e);
      } finally {
        busy.value = false;
      }
    }

    useEffect(() {
      Future.microtask(refresh);
      return null;
    }, const []);

    return AppScaffold(
      title: 'Guardian permission',
      noBackButton: true,
      child: ListView(
        children: [
          const SizedBox(height: 16),
          const Text(
            'Ask a legal guardian to sign',
            style: AppTheme.sectionTitle,
          ),
          const SizedBox(height: 12),
          const Text(
            'Because you are under 18, a legal guardian must sign before you can connect Apple Health. Choose how many guardians need to sign, then share each link with that person.',
            style: AppTheme.body,
          ),
          const SizedBox(height: 20),
          AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('Number of guardians', style: AppTheme.cardTitle),
                const SizedBox(height: 12),
                CupertinoSlidingSegmentedControl<int>(
                  groupValue: count.value,
                  children: const {1: Text('One'), 2: Text('Two')},
                  onValueChanged: (value) {
                    if (savedCount.value == 0 && value != null) count.value = value;
                  },
                ),
                if (savedCount.value > 0) ...[
                  const SizedBox(height: 10),
                  const Text(
                    'The number is fixed after you create the first link.',
                    style: AppTheme.bodyMuted,
                  ),
                ],
              ],
            ),
          ),
          const SizedBox(height: 16),
          for (var index = 0; index < count.value; index++) ...[
            AppCard(
              backgroundColor:
                  index < requests.value.length &&
                      requests.value[index]['signed'] == true
                  ? AppTheme.foam
                  : CupertinoColors.white,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          'Guardian ${index + 1}',
                          style: AppTheme.cardTitle,
                        ),
                      ),
                      if (index < requests.value.length &&
                          requests.value[index]['signed'] == true)
                        const Icon(
                          CupertinoIcons.check_mark_circled_solid,
                          color: CupertinoColors.activeGreen,
                          semanticLabel: 'Signed',
                        ),
                    ],
                  ),
                  const SizedBox(height: 10),
                  if (index < requests.value.length) ...[
                    Text(
                      'Personal number ending ${((requests.value[index]['personalNumber'] as String?) ?? '').substring(8)}',
                      style: AppTheme.bodyMuted,
                    ),
                    const SizedBox(height: 8),
                    Text(
                      requests.value[index]['signed'] == true
                          ? 'Signed and verified'
                          : 'Waiting for signature',
                      style: AppTheme.body,
                    ),
                    const SizedBox(height: 12),
                    if (requests.value[index]['signed'] != true)
                      CupertinoButton.filled(
                        onPressed: () async {
                          await Clipboard.setData(
                            ClipboardData(
                              text: requests.value[index]['link'] as String,
                            ),
                          );
                          copied.value = index;
                        },
                        child: Text(
                          copied.value == index ? 'Link copied' : 'Copy link',
                        ),
                      ),
                  ] else ...[
                    CupertinoTextField(
                      controller: index == 0 ? first : second,
                      placeholder: 'YYYYMMDDXXXX',
                      keyboardType: TextInputType.number,
                      maxLength: 12,
                      autofillHints: const [],
                    ),
                    const SizedBox(height: 12),
                    CupertinoButton.filled(
                      onPressed: busy.value || index > requests.value.length
                          ? null
                          : () => create(index),
                      child: const Text('Generate signing link'),
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 16),
          ],
          if (error.value != null) ...[
            Text(error.value!, style: AppTheme.body),
            const SizedBox(height: 12),
          ],
          CupertinoButton(
            onPressed: busy.value ? null : refresh,
            child: busy.value
                ? const CupertinoActivityIndicator()
                : const Text('Refresh signature status'),
          ),
          const SizedBox(height: 24),
        ],
      ),
    );
  }
}
