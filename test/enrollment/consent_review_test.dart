import 'dart:async';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:research_steps_template/enrollment/consent_review_controller.dart';
import 'package:research_steps_template/enrollment/bankid/models.dart';
import '../bankid/fakes.dart' show document;

class ReviewAPI implements ConsentReviewGateway {
  final ids = <String>[];
  Object? error;
  Completer<void>? pending;
  @override
  Future<ConsentDocument> current() async => document;
  @override
  Future<void> accept(ConsentDocument doc, String requestId) async {
    ids.add(requestId);
    if (error != null) throw error!;
    await pending?.future;
  }
}

void main() {
  test(
    'explicit acceptance required; ambiguous retries reuse request ID',
    () async {
      final api = ReviewAPI();
      var accepted = 0;
      final c = ConsentReviewController(
        gateway: api,
        isCurrent: () => true,
        onAccepted: () async {
          accepted++;
        },
      );
      addTearDown(c.dispose);
      await c.load();
      await c.accept();
      expect(api.ids, isEmpty);
      c.setReviewed(true);
      api.error = StateError('lost response');
      await c.accept();
      expect(c.state.error, isNotNull);
      expect(accepted, 0);
      api.error = null;
      await c.accept();
      expect(api.ids.toSet(), hasLength(1));
      expect(accepted, 1);
    },
  );
  test(
    'changed consent clears review; late acceptance cannot affect new session',
    () async {
      final api = ReviewAPI();
      var current = true, accepted = false;
      final c = ConsentReviewController(
        gateway: api,
        isCurrent: () => current,
        onAccepted: () async {
          accepted = true;
        },
      );
      addTearDown(c.dispose);
      await c.load();
      c.setReviewed(true);
      final request = RequestOptions(path: '/api/consent/accept');
      api.error = DioException(
        requestOptions: request,
        response: Response(requestOptions: request, statusCode: 409),
      );
      await c.accept();
      expect(c.state.reviewed, false);
      expect(c.state.document, document);
      api.error = null;
      api.pending = Completer<void>();
      c.setReviewed(true);
      final saving = c.accept();
      current = false;
      api.pending!.complete();
      await saving;
      expect(accepted, false);
    },
  );
}
