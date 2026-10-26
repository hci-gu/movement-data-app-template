import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:flutter/cupertino.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:research_steps_template/app/router.dart';
import 'package:research_steps_template/demo/cases.dart';
import 'package:research_steps_template/demo/data.dart';
import 'package:research_steps_template/demo/gallery.dart';
import 'package:research_steps_template/demo/services.dart';
import 'package:research_steps_template/enrollment/bankid/models.dart';
import 'package:research_steps_template/questionnaires/widgets/question_card.dart';
import 'package:research_steps_template/study/content.dart';

const capture = bool.fromEnvironment('REPORT_CAPTURE');
const output = String.fromEnvironment(
  'REPORT_OUTPUT',
  defaultValue: 'build/app-report/captures',
);

Iterable<(String, GoRoute)> screenRoutes(
  List<RouteBase> routes, [
  String parent = '',
]) sync* {
  for (final route in routes) {
    if (route is GoRoute) {
      final path = route.path.startsWith('/')
          ? route.path
          : '$parent/${route.path}';
      if (route.builder != null || route.pageBuilder != null) {
        yield (path, route);
      }
      yield* screenRoutes(route.routes, path);
    } else {
      yield* screenRoutes(route.routes, parent);
    }
  }
}

List<ScrollableState> scrolls(WidgetTester tester) => tester
    .stateList<ScrollableState>(find.byType(Scrollable))
    .where(
      (s) =>
          s.position.axis == Axis.vertical &&
          s.position is! PageMetrics &&
          s.context.findAncestorWidgetOfExactType<CupertinoDatePicker>() ==
              null &&
          ModalRoute.of(s.context)?.isCurrent == true &&
          (s.context.findRenderObject() as RenderBox)
                  .localToGlobal(Offset.zero)
                  .dy <
              844 &&
          (s.context.findRenderObject() as RenderBox)
                      .localToGlobal(Offset.zero)
                      .dy +
                  (s.context.findRenderObject() as RenderBox).size.height >
              0,
    )
    .toList();

Future<void> captureSections(
  WidgetTester tester,
  String id,
  Future<void> Function() snapshot,
) async {
  await snapshot();
  // Keep phone dimensions and overlap rather than stretching long content.
  for (final scroll in scrolls(tester)) {
    var count = 0;
    while (scroll.mounted &&
        scroll.position.pixels < scroll.position.maxScrollExtent - 1) {
      if (++count > 50) fail('Long content exceeded 50 sections: $id');
      final p = scroll.position;
      p.jumpTo(
        (p.pixels + p.viewportDimension * .8).clamp(0, p.maxScrollExtent),
      );
      await tester.pump(const Duration(milliseconds: 100));
      // Lazy lists refine their estimated extent when the last items mount.
      // Clamp again so the final capture never catches an overscroll bounce.
      for (var i = 0; i < 5 && p.outOfRange; i++) {
        p.jumpTo(p.pixels.clamp(p.minScrollExtent, p.maxScrollExtent));
        await tester.pump(const Duration(milliseconds: 100));
      }
      expect(p.outOfRange, false, reason: 'Unsettled scroll position: $id');
      expect(tester.takeException(), isNull);
      await snapshot();
    }
  }
}

void main() {
  final study = DemoStudy.generated();
  final cases = previewCases(study);
  final results = <Map<String, Object?>>[];

  setUpAll(() async {
    if (!capture) return;
    final font = File(const String.fromEnvironment('REPORT_FONT'));
    if (!await font.exists()) {
      throw StateError('REPORT_FONT must point to a real TTF font.');
    }
    final bytes = await font.readAsBytes();
    for (final family in [
      'CupertinoSystemText',
      'CupertinoSystemDisplay',
      '.SF Pro Text',
      '.SF Pro Display',
      'Roboto',
    ]) {
      await ui.loadFontFromList(bytes, fontFamily: family);
    }
    final icons = await rootBundle.load(
      'packages/cupertino_icons/assets/CupertinoIcons.ttf',
    );
    await ui.loadFontFromList(
      icons.buffer.asUint8List(),
      fontFamily: 'packages/cupertino_icons/CupertinoIcons',
    );
    await Directory(output).create(recursive: true);
  });

  for (final preview in cases) {
    testWidgets('preview ${preview.id}', (tester) async {
      tester.view.physicalSize = const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
        debugDefaultTargetPlatformOverride = null;
      });
      final runtime = DemoRuntime(study, preview);
      final captureKey = GlobalKey();
      final previewKey = GlobalKey<DemoPreviewState>();
      try {
        await runtime.prepare();
        await tester.pumpWidget(
          DemoPreview(
            key: previewKey,
            runtime: runtime,
            captureKey: captureKey,
          ),
        );
        for (
          var i = 0;
          i < 100 && previewKey.currentState?.ready != true;
          i++
        ) {
          await tester.pump(const Duration(milliseconds: 20));
        }
        expect(
          previewKey.currentState?.ready,
          true,
          reason: 'Preview did not become ready',
        );
        expect(previewKey.currentState?.error, isNull);
        await tester.pump(const Duration(milliseconds: 400));
        // A modal's first frame can start an animation after async state loads.
        await tester.pump(const Duration(milliseconds: 400));
        expect(tester.takeException(), isNull);
        expect(
          runtime.blockedRequests,
          0,
          reason: 'A production gateway escaped demo overrides',
        );
        final router = runtime.container.read(routerProvider);
        expect(
          router.routeInformationProvider.value.uri.path,
          Uri.parse(preview.path).path,
        );
        if (preview.questionId != null &&
            !['question-loading', 'question-error'].contains(preview.state)) {
          expect(
            find.byWidgetPredicate(
              (w) => w is QuestionCard && w.question.id == preview.questionId,
            ),
            findsOneWidget,
          );
        }
        if (preview.id.startsWith('picker-')) {
          expect(find.byType(CupertinoDatePicker), findsOneWidget);
        }
        if (preview.id == 'withdraw' || preview.state == 'question-discard') {
          expect(find.byType(CupertinoAlertDialog), findsOneWidget);
        }

        final images = <String>[];
        Future<void> snapshot() async {
          if (!capture) return;
          final name = '${preview.id}-${images.length + 1}.png';
          await tester.runAsync(() async {
            final boundary =
                captureKey.currentContext!.findRenderObject()!
                    as RenderRepaintBoundary;
            final image = await boundary.toImage(pixelRatio: 2);
            final bytes = await image.toByteData(
              format: ui.ImageByteFormat.png,
            );
            await File(
              '$output/$name',
            ).writeAsBytes(bytes!.buffer.asUint8List());
            image.dispose();
          });
          images.add(name);
        }

        await captureSections(tester, preview.id, snapshot);
        expect(runtime.blockedRequests, 0);
        results.add({...preview.toJson(), 'images': images});
      } finally {
        await tester.pumpWidget(const SizedBox());
        runtime.dispose();
        debugDefaultTargetPlatformOverride = null;
      }
    });
  }

  testWidgets('long consent captures reach the final paragraph', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final longStudy = DemoStudy(
      ConsentDocument(
        id: 'long-consent',
        version: 'test',
        title: 'Long consent regression',
        text: List.generate(
          40,
          (i) =>
              'Paragraph $i. This is synthetic information for checking that long study text remains fully reviewable.',
        ).join('\n\n'),
        documentHash: 'test',
      ),
      const [],
      'study',
    );
    final runtime = DemoRuntime(
      longStudy,
      const PreviewCase(
        'long-consent',
        'Long consent',
        'Test',
        '/introduction/login',
        signedIn: false,
      ),
    );
    try {
      await runtime.prepare();
      await tester.pumpWidget(
        DemoPreview(runtime: runtime, captureKey: GlobalKey()),
      );
      await tester.pumpAndSettle();
      var count = 0;
      await captureSections(tester, 'long-consent', () async {
        count++;
      });
      expect(count, greaterThan(2));
      expect(find.text('Start over').hitTestable(), findsOneWidget);
      final positions = scrolls(tester).map((s) => s.position);
      expect(positions, isNotEmpty);
      for (final p in positions) {
        expect(p.pixels, closeTo(p.maxScrollExtent, 1));
      }
      expect(runtime.blockedRequests, 0);
    } finally {
      await tester.pumpWidget(const SizedBox());
      runtime.dispose();
    }
  });

  test('report inventory covers every route and exported question', () {
    final runtime = DemoRuntime(study, cases.first);
    try {
      final router = runtime.container.read(routerProvider);
      final covered = cases.map((c) => Uri.parse(c.path).path).toSet();
      // Include nested/shell routes; redirect-only aliases have no screen.
      for (final (path, _) in screenRoutes(router.configuration.routes)) {
        if (path == '/questionnaires/:id' && study.forms.isEmpty) continue;
        final pattern = RegExp(
          '^${path.split('/').map((part) => part.startsWith(':') ? '[^/]+' : RegExp.escape(part)).join('/')}\$',
        );
        expect(
          covered.any(pattern.hasMatch),
          true,
          reason: 'Add a preview for $path',
        );
      }
      for (final form in study.forms) {
        for (final q in form.questions) {
          for (final saved in [false, true]) {
            expect(
              cases.where(
                (c) =>
                    c.formId == form.summary.id &&
                    c.questionId == q.id &&
                    c.saved == saved,
              ),
              isNotEmpty,
            );
          }
        }
      }
      expect(results.length, cases.length);
      if (capture) {
        File('$output/manifest.json').writeAsStringSync(
          const JsonEncoder.withIndent('  ').convert({
            'appName': StudyContent.appName,
            'sourceKind': study.sourceKind,
            'viewport': {'width': 390, 'height': 844, 'pixelRatio': 2},
            'renderer':
                'Flutter widgets with iOS theme and bundled Flutter SDK font; synthetic identities and health data',
            'requiredCases': cases.map((c) => c.id).toList(),
            'questionCount': study.forms.fold<int>(
              0,
              (n, f) => n + f.questions.length,
            ),
            'cases': results,
          }),
        );
      }
    } finally {
      runtime.dispose();
    }
  });
}
