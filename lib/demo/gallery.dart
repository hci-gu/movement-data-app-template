import 'dart:async';
import 'package:flutter/cupertino.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../app/app.dart';
import '../app/router.dart';
import '../enrollment/guardians_sheet.dart';
import '../session/participant_sheet.dart';
import '../ui/sheet.dart';
import '../ui/theme.dart';
import 'cases.dart';
import 'data.dart';
import 'services.dart';

/// A real app viewport without native startup or the gallery's developer UI.
class DemoPreview extends StatefulWidget {
  final DemoRuntime runtime;
  final GlobalKey captureKey;
  const DemoPreview({
    super.key,
    required this.runtime,
    required this.captureKey,
  });
  @override
  State<DemoPreview> createState() => DemoPreviewState();
}

class DemoPreviewState extends State<DemoPreview> {
  bool ready = false;
  Object? error;
  final rootKey = GlobalKey();
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _open());
  }

  List<Element> elements() {
    final result = <Element>[];
    void visit(Element element) {
      result.add(element);
      element.visitChildElements(visit);
    }

    final root = rootKey.currentContext;
    if (root is Element) visit(root);
    return result;
  }

  Future<void> _frame() async {
    WidgetsBinding.instance.scheduleFrame();
    await WidgetsBinding.instance.endOfFrame;
  }

  Future<void> _open() async {
    try {
      // Give async gateway/controller loads a bounded number of frames.
      for (var i = 0; i < 4 && mounted; i++) {
        await _frame();
      }
      for (final action in widget.runtime.preview.actions) {
        if (!mounted) return;
        if (action == 'guardians-approved') {
          final context = widget.runtime.container
              .read(routerProvider)
              .routerDelegate
              .navigatorKey
              .currentContext!;
          if (!context.mounted) return;
          unawaited(
            showAppSheet(
              context,
              const ParticipantSheet(child: GuardiansSheet()),
            ),
          );
          await _frame();
          continue;
        }
        Element? target;
        for (var attempt = 0; attempt < 60 && mounted; attempt++) {
          target = elements()
              .where((e) => e.widget.key == ValueKey(action))
              .firstOrNull;
          if (target != null) break;
          // Lazily built controls below long consent/home content need scrolling.
          for (final element in elements()) {
            if (element is StatefulElement &&
                element.state is ScrollableState) {
              final position = (element.state as ScrollableState).position;
              if (position.axis == Axis.vertical &&
                  position.hasContentDimensions) {
                position.jumpTo(
                  (position.pixels + position.viewportDimension * .8).clamp(
                    0,
                    position.maxScrollExtent,
                  ),
                );
              }
            }
          }
          await _frame();
        }
        if (!mounted) return;
        if (target?.widget is! CupertinoButton) {
          throw StateError('Preview action not found: $action');
        }
        await Scrollable.ensureVisible(target!, duration: Duration.zero);
        final button = target.widget as CupertinoButton;
        if (button.onPressed == null) {
          throw StateError('Preview action is disabled: $action');
        }
        button.onPressed!();
        await _frame();
      }
      if (mounted) setState(() => ready = true);
    } catch (e) {
      if (mounted) {
        setState(() {
          error = e;
          ready = true;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) => UncontrolledProviderScope(
    container: widget.runtime.container,
    child: RepaintBoundary(
      key: widget.captureKey,
      child: error != null
          ? CupertinoPageScaffold(
              child: Center(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Text('Could not open preview: $error'),
                ),
              ),
            )
          : KeyedSubtree(key: rootKey, child: const AppView()),
    ),
  );
}

class DemoGallery extends StatelessWidget {
  final DemoStudy study;
  const DemoGallery({super.key, required this.study});
  @override
  Widget build(BuildContext context) => CupertinoApp(
    debugShowCheckedModeBanner: false,
    theme: AppTheme.cupertinoTheme,
    home: Builder(
      builder: (context) => AppScaffold(
        title: 'App preview gallery',
        child: ListView(
          children: [
            const SizedBox(height: 20),
            const Text('Explore every screen', style: AppTheme.heroTitle),
            const SizedBox(height: 12),
            Text(
              study.sourceKind == 'template'
                  ? 'Template demonstration · synthetic questionnaires and participants.'
                  : 'Study content · synthetic participants and health measurements.',
            ),
            const SizedBox(height: 8),
            const Text(
              'Select a view to open it directly. Changes stay in memory. BankID, health access and uploads are simulated.',
            ),
            const SizedBox(height: 20),
            for (final preview in previewCases(study))
              CupertinoButton(
                alignment: Alignment.centerLeft,
                onPressed: () => Navigator.push(
                  context,
                  CupertinoPageRoute<void>(
                    builder: (_) => _DemoPage(study, preview),
                  ),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(preview.group, style: AppTheme.bodyMuted),
                    Text(preview.title),
                  ],
                ),
              ),
          ],
        ),
      ),
    ),
  );
}

class _DemoPage extends StatefulWidget {
  final DemoStudy study;
  final PreviewCase preview;
  const _DemoPage(this.study, this.preview);
  @override
  State<_DemoPage> createState() => _DemoPageState();
}

class _DemoPageState extends State<_DemoPage> {
  late DemoRuntime runtime;
  late Future<void> preparation;
  final captureKey = GlobalKey();
  final previewKey = GlobalKey<DemoPreviewState>();
  @override
  void initState() {
    super.initState();
    runtime = DemoRuntime(widget.study, widget.preview);
    preparation = runtime.prepare();
  }

  @override
  void dispose() {
    runtime.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => CupertinoPageScaffold(
    child: SafeArea(
      child: Column(
        children: [
          Container(
            color: AppTheme.foam,
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Row(
              children: [
                CupertinoButton(
                  onPressed: () => Navigator.pop(context),
                  child: const Text('Gallery'),
                ),
                const Expanded(
                  child: Text(
                    'DEMO · sample data',
                    textAlign: TextAlign.center,
                    style: AppTheme.bodyMuted,
                  ),
                ),
              ],
            ),
          ),
          Expanded(
            child: FutureBuilder<void>(
              future: preparation,
              builder: (context, snapshot) {
                if (snapshot.hasError) {
                  return Center(
                    child: Text('Preview failed: ${snapshot.error}'),
                  );
                }
                if (snapshot.connectionState != ConnectionState.done) {
                  return const Center(child: CupertinoActivityIndicator());
                }
                return DemoPreview(
                  key: previewKey,
                  runtime: runtime,
                  captureKey: captureKey,
                );
              },
            ),
          ),
        ],
      ),
    ),
  );
}
