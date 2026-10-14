import 'dart:async';
import 'dart:math' as math;
import 'package:flutter/cupertino.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../ui/theme.dart';
import 'controller.dart';
import 'models.dart';
import 'rules.dart';
import 'widgets/question_card.dart';
import 'widgets/question_field.dart';

class QuestionnaireScreen extends ConsumerWidget {
  final String questionnaireId;
  const QuestionnaireScreen({super.key, required this.questionnaireId});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final provider = questionnaireControllerProvider(questionnaireId);
    ref.watch(provider);
    // A different session gets a fresh page controller as well as fresh answers.
    return _QuestionnairePages(
      key: ObjectKey(ref.read(provider.notifier)),
      questionnaireId: questionnaireId,
    );
  }
}

class _QuestionnairePages extends ConsumerStatefulWidget {
  final String questionnaireId;
  const _QuestionnairePages({super.key, required this.questionnaireId});
  @override
  ConsumerState<_QuestionnairePages> createState() =>
      _QuestionnairePagesState();
}

class _QuestionnairePagesState extends ConsumerState<_QuestionnairePages>
    with WidgetsBindingObserver {
  final pages = PageController();
  List<String> ids = [];
  int index = 0;
  bool allowPop = false, asking = false, reconciling = false;
  String? errorTarget;
  Timer? autoAdvance;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  void cancelAutoAdvance() => autoAdvance?.cancel();

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed) cancelAutoAdvance();
  }

  void answer(QuestionDefinition question, Object? value) {
    cancelAutoAdvance();
    final provider = questionnaireControllerProvider(widget.questionnaireId);
    final controller = ref.read(provider.notifier);
    controller.setAnswer(question.id, value);
    if (isUnanswered(value) ||
        validateValue(question, value, controller.now()) != null) {
      return;
    }
    autoAdvance = Timer(const Duration(milliseconds: 500), () {
      if (!mounted ||
          asking ||
          reconciling ||
          ModalRoute.of(context)?.isCurrent == false ||
          ids.isEmpty ||
          ids[index] != question.id) {
        return;
      }
      final state = ref.read(provider);
      if (state.loading ||
          state.submitting ||
          state.response != null ||
          state.answers[question.id] != value ||
          state.errors.containsKey(question.id)) {
        return;
      }
      // Resolve the next page after any branch changes. The last answer never
      // submits the response; move ignores a target beyond the final page.
      unawaited(move(index + 1));
    });
  }

  @override
  void dispose() {
    cancelAutoAdvance();
    WidgetsBinding.instance.removeObserver(this);
    pages.dispose();
    super.dispose();
  }

  Future<void> home() async {
    cancelAutoAdvance();
    setState(() => allowPop = true);
    await WidgetsBinding.instance.endOfFrame;
    if (!mounted) return;
    final router = GoRouter.maybeOf(context);
    if (router != null) {
      router.goNamed('home');
    } else if (Navigator.of(context).canPop()) {
      Navigator.pop(context);
    }
  }

  Future<void> leave(QuestionnaireState state) async {
    cancelAutoAdvance();
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
    if (discard && mounted) await home();
  }

  void reconcile(List<QuestionDefinition> visible) {
    final next = visible.map((q) => q.id).toList();
    if (next.length == ids.length &&
        List.generate(next.length, (i) => next[i] == ids[i]).every((v) => v)) {
      return;
    }
    final current = ids.isEmpty ? null : ids[index.clamp(0, ids.length - 1)];
    var target = current == null ? 0 : next.indexOf(current);
    if (target < 0) {
      cancelAutoAdvance();
      target = 0;
      for (var i = math.min(index - 1, ids.length - 1); i >= 0; i--) {
        final preceding = next.indexOf(ids[i]);
        if (preceding >= 0) {
          target = preceding;
          break;
        }
      }
    }
    ids = next;
    index = target;
    reconciling = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      if (pages.hasClients && ids.isNotEmpty) pages.jumpToPage(index);
      reconciling = false;
    });
  }

  Future<void> move(int target) async {
    cancelAutoAdvance();
    if (!mounted ||
        !pages.hasClients ||
        target < 0 ||
        target >= ids.length ||
        ref
            .read(questionnaireControllerProvider(widget.questionnaireId))
            .submitting) {
      return;
    }
    FocusManager.instance.primaryFocus?.unfocus();
    if (MediaQuery.of(context).disableAnimations) {
      pages.jumpToPage(target);
    } else {
      await pages.animateToPage(
        target,
        duration: const Duration(milliseconds: 280),
        curve: Curves.easeOutCubic,
      );
    }
  }

  Future<void> submit() async {
    cancelAutoAdvance();
    FocusManager.instance.primaryFocus?.unfocus();
    final provider = questionnaireControllerProvider(widget.questionnaireId);
    final controller = ref.read(provider.notifier);
    final saved = await controller.submit();
    if (!mounted || ref.read(provider.notifier) != controller) return;
    if (saved) {
      await home();
      return;
    }
    final errors = ref.read(provider).errors;
    final target = ids.indexWhere(errors.containsKey);
    if (target >= 0) {
      setState(() => errorTarget = ids[target]);
      await move(target);
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
        ? <QuestionDefinition>[]
        : visibleQuestions(definition, state.answers, participant);
    reconcile(visible);
    final readOnly = state.response != null;
    final content = PopScope(
      canPop: allowPop || (!state.dirty && !state.submitting),
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) leave(state);
      },
      child: AppScaffold(
        title: definition?.summary.name ?? 'Formulär',
        withHorizontalPadding: false,
        leading: CupertinoButton(
          padding: EdgeInsets.zero,
          onPressed: state.submitting ? null : () => leave(state),
          child: const Icon(
            CupertinoIcons.xmark,
            semanticLabel: 'Till checklistan',
          ),
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
                        onPressed: controller.load,
                        child: const Text('Försök igen'),
                      ),
                    ],
                  ),
                ),
              )
            : Column(
                children: [
                  Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 8,
                    ),
                    child: Semantics(
                      liveRegion: true,
                      excludeSemantics: true,
                      label: visible.isEmpty
                          ? 'Inga frågor att visa'
                          : 'Fråga ${index + 1} av ${visible.length}: ${visible[index].text}',
                      child: Text(
                        visible.isEmpty
                            ? 'Inga frågor att visa'
                            : 'Fråga ${index + 1} av ${visible.length}',
                        style: AppTheme.bodyMuted,
                      ),
                    ),
                  ),
                  Expanded(
                    child: visible.isEmpty
                        ? Center(
                            child: Padding(
                              padding: const EdgeInsets.all(24),
                              child: Text(
                                readOnly
                                    ? 'Ditt tomma svar är sparat. Det finns inga frågor att visa.'
                                    : 'Det finns inga frågor för dig i detta formulär. Du kan ändå skicka in ett tomt svar.',
                                textAlign: TextAlign.center,
                              ),
                            ),
                          )
                        : AbsorbPointer(
                            absorbing: state.submitting,
                            child: PageView.builder(
                              controller: pages,
                              scrollDirection: Axis.vertical,
                              physics: state.submitting
                                  ? const NeverScrollableScrollPhysics()
                                  : const ClampingScrollPhysics(),
                              itemCount: visible.length,
                              findChildIndexCallback: (key) =>
                                  key is ValueKey<String> &&
                                      ids.contains(key.value)
                                  ? ids.indexOf(key.value)
                                  : null,
                              onPageChanged: (value) {
                                if (reconciling) return;
                                cancelAutoAdvance();
                                FocusManager.instance.primaryFocus?.unfocus();
                                setState(() => index = value);
                              },
                              itemBuilder: (context, i) {
                                final q = visible[i];
                                return _QuestionPage(
                                  key: ValueKey(q.id),
                                  active: i == index,
                                  onBoundary: (direction) =>
                                      move(i + direction),
                                  revealError:
                                      errorTarget == q.id &&
                                      state.errors.containsKey(q.id),
                                  description: i == 0
                                      ? definition.summary.description
                                      : '',
                                  readOnly: readOnly,
                                  question: q,
                                  value: state.answers[q.id],
                                  error: state.errors[q.id],
                                  onChanged: (value) => answer(q, value),
                                );
                              },
                            ),
                          ),
                  ),
                  if (state.error != null)
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 16),
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
                  Padding(
                    padding: const EdgeInsets.all(12),
                    child: Row(
                      children: [
                        Expanded(
                          child: CupertinoButton(
                            onPressed: !state.submitting && index > 0
                                ? () => move(index - 1)
                                : null,
                            child: const Text('Föregående'),
                          ),
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: CupertinoButton.filled(
                            onPressed: state.submitting
                                ? null
                                : index < visible.length - 1
                                ? () => move(index + 1)
                                : readOnly
                                ? home
                                : submit,
                            child: state.submitting
                                ? const CupertinoActivityIndicator()
                                : Text(
                                    index < visible.length - 1
                                        ? 'Nästa'
                                        : readOnly
                                        ? 'Till checklistan'
                                        : 'Skicka in svar',
                                  ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
      ),
    );
    return Listener(onPointerDown: (_) => cancelAutoAdvance(), child: content);
  }
}

/// The inner scroll owns long content. Overscroll at its boundary deliberately
/// turns one page. Short content is centered in the available page height.
class _QuestionPage extends StatefulWidget {
  final QuestionDefinition question;
  final Object? value;
  final String description;
  final String? error;
  final bool active, readOnly, revealError;
  final ValueChanged<Object?> onChanged;
  final ValueChanged<int> onBoundary;
  const _QuestionPage({
    super.key,
    required this.question,
    required this.value,
    required this.description,
    required this.error,
    required this.active,
    required this.readOnly,
    required this.revealError,
    required this.onChanged,
    required this.onBoundary,
  });
  @override
  State<_QuestionPage> createState() => _QuestionPageState();
}

class _QuestionPageState extends State<_QuestionPage> {
  final scroll = ScrollController();
  final inputKey = GlobalKey();
  double overscroll = 0;
  bool turned = false;
  @override
  void initState() {
    super.initState();
    if (widget.active && widget.revealError) revealInput();
  }

  void revealInput() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final target = inputKey.currentContext;
      if (mounted && target != null) {
        Scrollable.ensureVisible(target, alignment: 0.5);
      }
    });
  }

  @override
  void dispose() {
    scroll.dispose();
    super.dispose();
  }

  @override
  void didUpdateWidget(covariant _QuestionPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.active &&
        widget.revealError &&
        (!oldWidget.active ||
            !oldWidget.revealError ||
            oldWidget.error != widget.error)) {
      revealInput();
    }
  }

  @override
  Widget build(BuildContext context) => ExcludeSemantics(
    excluding: !widget.active,
    child: NotificationListener<ScrollNotification>(
      onNotification: (event) {
        if (event.depth != 0) return false;
        if (event is ScrollStartNotification) {
          overscroll = 0;
          turned = false;
        }
        if (event is OverscrollNotification &&
            event.dragDetails != null &&
            widget.active &&
            !turned) {
          overscroll += event.overscroll;
          if (overscroll.abs() >= 65) {
            turned = true;
            widget.onBoundary(overscroll > 0 ? 1 : -1);
          }
        }
        return false;
      },
      child: LayoutBuilder(
        builder: (context, constraints) => SingleChildScrollView(
          controller: scroll,
          physics: const ClampingScrollPhysics(
            parent: AlwaysScrollableScrollPhysics(),
          ),
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          padding: const EdgeInsets.all(16),
          child: ConstrainedBox(
            constraints: BoxConstraints(
              minHeight: math.max(0, constraints.maxHeight - 32),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (widget.description.isNotEmpty) ...[
                  QuestionnaireRichText(widget.description),
                  const SizedBox(height: 16),
                ],
                Text(
                  widget.readOnly
                      ? 'Dina sparade svar'
                      : 'Du kan hoppa över frågor.',
                  style: AppTheme.bodyMuted,
                ),
                const SizedBox(height: 16),
                QuestionCard(
                  question: widget.question,
                  error: widget.error,
                  child: KeyedSubtree(
                    key: inputKey,
                    child: QuestionField(
                      question: widget.question,
                      value: widget.value,
                      readOnly: widget.readOnly,
                      onChanged: widget.onChanged,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    ),
  );
}
