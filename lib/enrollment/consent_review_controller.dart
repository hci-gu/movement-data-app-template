import 'dart:async';
import 'dart:convert';
import 'dart:math';
import 'package:dio/dio.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../infrastructure/http_client.dart';
import '../session/controller.dart';
import 'bankid/models.dart';

abstract class ConsentReviewGateway {
  Future<ConsentDocument> current();
  Future<void> accept(ConsentDocument document, String requestId);
}

class HttpConsentReviewGateway implements ConsentReviewGateway {
  final Dio client;
  final String token;
  HttpConsentReviewGateway(this.client, this.token);
  Options get options => Options(headers: {'Authorization': 'Bearer $token'});
  @override
  Future<ConsentDocument> current() async {
    final response = await client.get('/api/consent/current', options: options);
    return ConsentDocument.fromJson(
      Map<String, dynamic>.from(response.data['document'] as Map),
    );
  }

  @override
  Future<void> accept(ConsentDocument document, String requestId) async {
    await client.post(
      '/api/consent/accept',
      options: options,
      data: {
        'consentTextId': document.id,
        'documentHash': document.documentHash,
        'acceptanceRequestId': requestId,
      },
    );
  }
}

class ConsentReviewState {
  final ConsentDocument? document;
  final bool loading, saving, reviewed;
  final String? error;
  const ConsentReviewState({
    this.document,
    this.loading = true,
    this.saving = false,
    this.reviewed = false,
    this.error,
  });
}

class ConsentReviewController extends StateNotifier<ConsentReviewState> {
  final ConsentReviewGateway gateway;
  final bool Function() isCurrent;
  final Future<void> Function() onAccepted;
  String? _requestId;
  ConsentReviewController({
    required this.gateway,
    required this.isCurrent,
    required this.onAccepted,
  }) : super(const ConsentReviewState());

  Future<void> load() async {
    if (!mounted || !isCurrent() || state.saving) return;
    state = const ConsentReviewState();
    _requestId = null;
    try {
      final document = await gateway.current();
      if (mounted && isCurrent()) {
        state = ConsentReviewState(document: document, loading: false);
      }
    } catch (_) {
      if (mounted && isCurrent()) {
        state = const ConsentReviewState(
          loading: false,
          error: 'Samtycket kunde inte hämtas. Försök igen.',
        );
      }
    }
  }

  void setReviewed(bool value) {
    if (state.document == null || state.saving) return;
    state = ConsentReviewState(
      document: state.document,
      loading: false,
      reviewed: value,
      error: state.error,
    );
  }

  Future<void> accept() async {
    final document = state.document;
    if (!mounted ||
        !isCurrent() ||
        document == null ||
        !state.reviewed ||
        state.saving) {
      return;
    }
    _requestId ??= base64UrlEncode(
      List.generate(32, (_) => Random.secure().nextInt(256)),
    ).replaceAll('=', '');
    state = ConsentReviewState(
      document: document,
      loading: false,
      saving: true,
      reviewed: true,
    );
    try {
      await gateway.accept(document, _requestId!);
      if (mounted && isCurrent()) await onAccepted();
      if (mounted && isCurrent()) {
        state = ConsentReviewState(
          document: document,
          loading: false,
          reviewed: true,
        );
      }
    } on DioException catch (error) {
      if (!mounted || !isCurrent()) return;
      if (error.response?.statusCode == 409) {
        state = const ConsentReviewState();
        await load();
        if (mounted && isCurrent()) {
          state = ConsentReviewState(
            document: state.document,
            loading: false,
            error:
                'Samtycket har ändrats. Läs det igen innan du bekräftar.',
          );
        }
      } else {
        _failed(document);
      }
    } catch (_) {
      if (mounted && isCurrent()) _failed(document);
    }
  }

  void _failed(ConsentDocument document) {
    state = ConsentReviewState(
      document: document,
      loading: false,
      reviewed: true,
      error: 'Samtycket kunde inte bekräftas. Försök igen.',
    );
  }
}

final consentReviewGatewayProvider = Provider<ConsentReviewGateway>((ref) {
  final token = ref.watch(sessionProvider.select((s) => s?.token)) ?? '';
  return HttpConsentReviewGateway(ref.watch(httpClientProvider), token);
});
final consentReviewProvider =
    StateNotifierProvider.autoDispose<
      ConsentReviewController,
      ConsentReviewState
    >((ref) {
      final token = ref.watch(sessionProvider.select((s) => s?.token));
      final controller = ConsentReviewController(
        gateway: ref.watch(consentReviewGatewayProvider),
        isCurrent: () =>
            token != null && ref.read(sessionProvider)?.token == token,
        onAccepted: () => ref.read(sessionProvider.notifier).refresh(),
      );
      if (token != null) unawaited(controller.load());
      return controller;
    });
