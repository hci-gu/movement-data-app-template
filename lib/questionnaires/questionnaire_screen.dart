import 'package:flutter/cupertino.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/theme.dart';
import 'controller.dart';
import 'rules.dart';
import 'widgets/question_card.dart';
import 'widgets/question_field.dart';

class QuestionnaireScreen extends ConsumerStatefulWidget {
  final String questionnaireId;
  const QuestionnaireScreen({super.key, required this.questionnaireId});
  @override
  ConsumerState<QuestionnaireScreen> createState() =>
      _QuestionnaireScreenState();
}

class _QuestionnaireScreenState extends ConsumerState<QuestionnaireScreen> {
  final fields = <String, GlobalKey>{};
  bool allowPop = false, asking = false;
  Future<void> leave(QuestionnaireState state) async {
    if (asking || state.submitting) return;
    asking = true;
    final discard =
        !state.dirty ||
        state.response != null ||
        await showCupertinoDialog<bool>(
              context: context,
              builder: (context) => CupertinoAlertDialog(
                title: const Text('Lämna formuläret?'),
                content: const Text('Dina oskickade svar kommer att tas bort.'),
                actions: [
                  CupertinoDialogAction(
                    onPressed: () => Navigator.pop(context, false),
                    child: const Text('Stanna'),
                  ),
                  CupertinoDialogAction(
                    isDestructiveAction: true,
                    onPressed: () => Navigator.pop(context, true),
                    child: const Text('Lämna'),
                  ),
                ],
              ),
            ) ==
            true;
    asking = false;
    if (!discard || !mounted) return;
    setState(() => allowPop = true);
    await WidgetsBinding.instance.endOfFrame;
    if (mounted) {
      if (context.canPop()) {
        context.pop();
      } else {
        context.goNamed('questionnaires');
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final provider = questionnaireControllerProvider(widget.questionnaireId);
    final state = ref.watch(provider);
    final controller = ref.read(provider.notifier);
    final participant = ref.watch(participantContextProvider);
    final definition = state.definition;
    final visible = definition == null || participant == null
        ? []
        : visibleQuestions(definition, state.answers, participant);
    return PopScope(
      canPop: allowPop || (!state.dirty && !state.submitting),
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) leave(state);
      },
      child: AppScaffold(
        title: 'Formulär',
        withHorizontalPadding: false,
        leading: CupertinoButton(
          padding: EdgeInsets.zero,
          onPressed: state.submitting ? null : () => leave(state),
          child: const Icon(CupertinoIcons.back, semanticLabel: 'Tillbaka'),
        ),
        child: state.loading
            ? const Center(child: CupertinoActivityIndicator())
            : definition == null
            ? Center(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(state.error ?? 'Formuläret kunde inte läsas.'),
                      CupertinoButton(
                        onPressed: () => controller.load(),
                        child: const Text('Försök igen'),
                      ),
                    ],
                  ),
                ),
              )
            : SingleChildScrollView(
                keyboardDismissBehavior:
                    ScrollViewKeyboardDismissBehavior.onDrag,
                padding: const EdgeInsets.all(AppTheme.basePadding),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(definition.summary.name, style: AppTheme.sectionTitle),
                    const SizedBox(height: 12),
                    if (definition.summary.description.isNotEmpty)
                      QuestionnaireRichText(definition.summary.description),
                    Text(
                      state.response != null
                          ? 'Dina svar är inskickade. Tack för ditt deltagande.'
                          : 'Du kan hoppa över frågor. Skicka in när du är klar.',
                      style: AppTheme.bodyMuted,
                    ),
                    const SizedBox(height: 20),
                    for (final q in visible)
                      Padding(
                        key: fields.putIfAbsent(q.id, GlobalKey.new),
                        padding: const EdgeInsets.only(bottom: 16),
                        child: QuestionCard(
                          question: q,
                          error: state.errors[q.id],
                          child: QuestionField(
                            key: ValueKey(q.id),
                            question: q,
                            value: state.answers[q.id],
                            readOnly:
                                state.response != null || state.submitting,
                            onChanged: (value) => ref
                                .read(provider.notifier)
                                .setAnswer(q.id, value),
                          ),
                        ),
                      ),
                    if (state.error != null)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 12),
                        child: Semantics(
                          liveRegion: true,
                          child: Text(
                            state.error!,
                            style: const TextStyle(
                              color: CupertinoColors.systemRed,
                            ),
                          ),
                        ),
                      ),
                    if (state.response == null)
                      CupertinoButton.filled(
                        onPressed: state.submitting
                            ? null
                            : () async {
                                FocusScope.of(context).unfocus();
                                await controller.submit();
                                if (!mounted ||
                                    ref.read(provider.notifier) != controller) {
                                  return;
                                }
                                final errors = ref.read(provider).errors;
                                if (errors.isNotEmpty) {
                                  await WidgetsBinding.instance.endOfFrame;
                                  final target =
                                      fields[errors.keys.first]?.currentContext;
                                  if (target != null && target.mounted) {
                                    await Scrollable.ensureVisible(
                                      target,
                                      duration: const Duration(
                                        milliseconds: 250,
                                      ),
                                      alignment: 0.1,
                                    );
                                  }
                                }
                              },
                        child: state.submitting
                            ? const CupertinoActivityIndicator()
                            : const Text('Skicka in svar'),
                      ),
                    if (state.response != null)
                      CupertinoButton(
                        onPressed: () => leave(state),
                        child: const Text('Tillbaka till formulär'),
                      ),
                  ],
                ),
              ),
      ),
    );
  }
}
