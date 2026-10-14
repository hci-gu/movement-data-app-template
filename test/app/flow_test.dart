import 'package:flutter/cupertino.dart';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:research_steps_template/ui/theme.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/app/home_state.dart';
import 'package:research_steps_template/app/router.dart';
import 'package:research_steps_template/enrollment/bankid/controller.dart';
import 'package:research_steps_template/enrollment/bankid/models.dart';
import 'package:research_steps_template/session/controller.dart';
import 'package:research_steps_template/questionnaires/controller.dart';
import 'package:research_steps_template/questionnaires/models.dart';
import 'package:research_steps_template/steps/controller.dart';
import 'package:research_steps_template/steps/health_reader.dart';
import '../session/session_test.dart'
    show MemorySessionStore, SessionApi, grant;
import '../steps/steps_test.dart' show Reader, Uploader, Marker;
import '../bankid/fakes.dart' as bankid;
import '../questionnaires/fixtures.dart';

class Forms extends FakeQuestionnaireGateway {
  int lists = 0;
  bool empty = false, fail = false;
  @override
  Future<List<QuestionnaireSummary>> list() async {
    lists++;
    if (fail) throw Exception('discovery failed');
    if (empty) return [];
    return super.list();
  }
}

class Harness {
  final previewKey = GlobalKey();
  final sessions = MemorySessionStore(), sessionApi = SessionApi();
  final attempts = bankid.MemoryStore();
  late final signingApi = bankid.Gateway(attempts);
  final forms = Forms();
  final reader = Reader(), uploader = Uploader(), marker = Marker();
  late ProviderContainer container;
  Harness() {
    forms.definition = const QuestionnaireDefinition(
      QuestionnaireSummary(
        id: 'testfrm00000001',
        name: 'Example questionnaire',
      ),
      [
        QuestionDefinition(
          id: 'text',
          text: 'Your answer',
          type: QuestionType.text,
        ),
      ],
    );
    container = ProviderContainer(
      overrides: [
        sessionStoreProvider.overrideWithValue(sessions),
        sessionGatewayProvider.overrideWithValue(sessionApi),
        bankIdProvider.overrideWith((ref) {
          ref.watch(sessionProvider.select((s) => s?.token));
          return BankIdController(
            gateway: signingApi,
            store: attempts,
            launch: (_) async => true,
            onGrant: (value) =>
                ref.read(sessionProvider.notifier).accept(value),
          );
        }),
        questionnaireGatewayProvider.overrideWithValue(forms),
        stepsProvider.overrideWith((ref) {
          final token = ref.watch(sessionProvider.select((s) => s?.token));
          final c = StepsController(
            reader: reader,
            gateway: uploader,
            store: marker,
            isCurrent: () =>
                token != null && ref.read(sessionProvider)?.token == token,
          );
          c.restore();
          return c;
        }),
      ],
    );
  }
  Future<void> mount(WidgetTester tester) async {
    addTearDown(container.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: RepaintBoundary(
          key: previewKey,
          child: Consumer(
            builder: (context, ref, _) => CupertinoApp.router(
              debugShowCheckedModeBanner: false,
              theme: AppTheme.cupertinoTheme,
              routerConfig: ref.watch(routerProvider),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  Future<void> close(WidgetTester tester) async {
    await tester.pumpWidget(const SizedBox());
    container.dispose();
  }

  Future<void> preview(WidgetTester tester, String name) async {
    if (!const bool.fromEnvironment('SAVE_REVIEW_PREVIEWS')) return;
    await tester.runAsync(() async {
      final boundary =
          previewKey.currentContext!.findRenderObject()!
              as RenderRepaintBoundary;
      final image = await boundary.toImage();
      final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
      await Directory('build/review').create(recursive: true);
      await File(
        'build/review/$name.png',
      ).writeAsBytes(bytes!.buffer.asUint8List());
      image.dispose();
    });
  }
}

Future<void> tap(WidgetTester tester, String label) async {
  await tester.ensureVisible(find.text(label));
  await tester.tap(find.text(label));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() async {
    if (!const bool.fromEnvironment('SAVE_REVIEW_PREVIEWS')) return;
    final file = File(const String.fromEnvironment('REVIEW_FONT_PATH'));
    final bytes = await file.readAsBytes();
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
  });
  test(
    'Finish requires known discovery and every task; empty differs from unknown and unsupported',
    () {
      const a = QuestionnaireListEntry(
        QuestionnaireSummary(id: 'a', name: 'A'),
        true,
      );
      const b = QuestionnaireListEntry(
        QuestionnaireSummary(id: 'b', name: 'B'),
        false,
      );
      const unsupported = QuestionnaireListEntry(
        QuestionnaireSummary(id: 'c', name: 'C', occurrence: 'daily'),
        true,
      );
      for (final data in [
        const AsyncLoading<List<QuestionnaireListEntry>>(),
        AsyncError<List<QuestionnaireListEntry>>(
          Exception('offline'),
          StackTrace.current,
        ),
        const AsyncData([a, b]),
        const AsyncData([unsupported]),
      ]) {
        expect(
          HomeState(
            eligible: true,
            questionnaires: data,
            stepsCompleted: true,
          ).canFinish,
          false,
        );
      }
      for (final entries in [
        <QuestionnaireListEntry>[],
        [a],
        [a, a],
      ]) {
        expect(
          HomeState(
            eligible: true,
            questionnaires: AsyncData(entries),
            stepsCompleted: true,
          ).canFinish,
          true,
        );
        expect(
          HomeState(
            eligible: true,
            questionnaires: AsyncData(entries),
            stepsCompleted: false,
          ).canFinish,
          false,
        );
        expect(
          HomeState(
            eligible: false,
            questionnaires: AsyncData(entries),
            stepsCompleted: true,
          ).canFinish,
          false,
        );
      }
    },
  );
  test(
    'redirects protect startup, pending attempts, guardian tasks, Done, and allow review',
    () {
      String? redirect(
        String path, {
        bool ready = true,
        bool signedIn = true,
        bool pending = false,
        bool eligible = true,
        bool complete = false,
      }) => participantRedirect(
        path: path,
        ready: ready,
        signedIn: signedIn,
        pendingBankId: pending,
        eligible: eligible,
        canFinish: complete,
      );
      expect(redirect('/home', ready: false), '/loading');
      expect(redirect('/loading', signedIn: false), '/introduction');
      expect(
        redirect('/loading', signedIn: false, pending: true),
        '/introduction/login',
      );
      expect(redirect('/questionnaires/q', signedIn: false), '/introduction');
      expect(redirect('/loading', complete: true), '/home');
      expect(redirect('/questionnaires/q', eligible: false), '/home');
      expect(redirect('/done'), '/home');
      expect(redirect('/done', complete: true), isNull);
      expect(redirect('/home', complete: true), isNull);
      expect(redirect('/questionnaires/q', complete: true), isNull);
    },
  );
  for (final stepsFirst in [false, true]) {
    testWidgets(
      'introduction, signing, both task orders, Done and review (steps first: $stepsFirst)',
      (tester) async {
        tester.view.physicalSize = const Size(430, 932);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final h = Harness();
        await h.mount(tester);
        await h.preview(tester, 'introduction');
        expect(find.text('Continue with BankID'), findsOneWidget);
        await tap(tester, 'Continue with BankID');
        expect(find.text(bankid.document.text), findsOneWidget);
        await tester.tap(find.byType(CupertinoSwitch));
        await tester.pumpAndSettle();
        h.signingApi.result = BankIdAttempt(
          id: 'accepted',
          secret: 'secret',
          expiresAt: 4102444800,
          status: 'accepted',
          document: bankid.document,
          grant: grant(),
        );
        await tap(tester, 'Sign with BankID');
        expect(find.text('Your checklist'), findsOneWidget);
        await h.preview(tester, 'home');
        expect(h.reader.reads, 0);
        Future<void> upload() async {
          await tap(tester, 'Read step history');
          await tap(tester, 'Upload step data');
        }

        Future<void> questionnaire() async {
          await tap(tester, 'Open questionnaire');
          await h.preview(tester, 'questionnaire');
          await tester.enterText(
            find.byType(CupertinoTextField),
            'Saved answer',
          );
          await tap(tester, 'Skicka in svar');
          expect(find.text('Your checklist'), findsOneWidget);
        }

        if (stepsFirst) {
          await upload();
        } else {
          await questionnaire();
        }
        expect(h.container.read(homeStateProvider).canFinish, false);
        if (stepsFirst) {
          await questionnaire();
        } else {
          await upload();
        }
        expect(h.container.read(homeStateProvider).canFinish, true);
        await tap(tester, 'Finish');
        expect(find.text('Thank you'), findsOneWidget);
        await h.preview(tester, 'done');
        await tap(tester, 'Review checklist');
        await tap(tester, 'Review saved answers');
        expect(
          tester
              .widget<CupertinoTextField>(find.byType(CupertinoTextField))
              .readOnly,
          true,
        );
        await tap(tester, 'Till checklistan');
        expect(
          find.text('Your checklist'),
          findsOneWidget,
          reason:
              '${h.container.read(routerProvider).routeInformationProvider.value.uri}: ${tester.widgetList<Text>(find.byType(Text)).map((w) => w.data).toList()}',
        );
        await h.close(tester);
      },
    );
  }
  testWidgets(
    'restored guardian participant lands home without protected reads or upload access',
    (tester) async {
      final h = Harness();
      h.sessions.value = grant(guardian: true);
      h.sessionApi.participant = h.sessionApi.participant.requiringGuardian();
      await h.mount(tester);
      expect(find.text('Guardian approval'), findsOneWidget);
      expect(h.forms.lists, 0);
      expect(h.reader.reads, 0);
      h.container.read(routerProvider).go('/questionnaires/testfrm00000001');
      await tester.pumpAndSettle();
      expect(find.text('Your checklist'), findsOneWidget);
      expect(h.forms.lists, 0);
      await h.close(tester);
    },
  );
  testWidgets(
    'restored BankID attempt resumes enrollment; late return never skips verification',
    (tester) async {
      final h = Harness();
      h.attempts.value = bankid.pending.toStorage();
      await h.mount(tester);
      expect(find.text('Continue with BankID'), findsOneWidget);
      expect(h.container.read(sessionProvider), isNull);
      expect(h.signingApi.statuses, greaterThan(0));
      await h.close(tester);
    },
  );
  testWidgets(
    'empty discovery permits upload-only completion; errors never unlock Finish',
    (tester) async {
      final h = Harness();
      h.sessions.value = grant();
      h.forms.empty = true;
      h.marker.value = DateTime(2026);
      await h.mount(tester);
      expect(h.container.read(homeStateProvider).canFinish, true);
      h.forms.fail = true;
      h.container.invalidate(questionnaireListProvider);
      await tester.pumpAndSettle();
      expect(find.text('Retry questionnaires'), findsOneWidget);
      expect(h.container.read(homeStateProvider).canFinish, false);
      h.container.read(routerProvider).goNamed('done');
      await tester.pumpAndSettle();
      expect(find.text('Your checklist'), findsOneWidget);
      h.forms.fail = false;
      await tap(tester, 'Retry questionnaires');
      expect(h.container.read(homeStateProvider).canFinish, true);
      await h.close(tester);
    },
  );
  testWidgets(
    'denied, empty and failed uploads leave questionnaires accessible',
    (tester) async {
      final h = Harness();
      h.sessions.value = grant();
      await h.mount(tester);
      final steps = h.container.read(stepsProvider.notifier);
      h.reader.result = const HealthRead(false, []);
      await steps.read();
      await tester.pumpAndSettle();
      expect(h.container.read(homeStateProvider).canFinish, false);
      await tap(tester, 'Open questionnaire');
      await tester.tap(find.byIcon(CupertinoIcons.xmark));
      await tester.pumpAndSettle();
      h.reader.result = const HealthRead(true, []);
      await steps.read();
      await tester.pumpAndSettle();
      expect(h.container.read(homeStateProvider).canFinish, false);
      await tap(tester, 'Open questionnaire');
      await tester.tap(find.byIcon(CupertinoIcons.xmark));
      await tester.pumpAndSettle();
      h.reader.result = Reader().result;
      h.uploader.failure = Exception('partial');
      await steps.read();
      await steps.upload();
      await tester.pumpAndSettle();
      expect(h.container.read(homeStateProvider).canFinish, false);
      await tap(tester, 'Open questionnaire');
      expect(find.text('Your answer'), findsOneWidget);
      await h.close(tester);
    },
  );
}
