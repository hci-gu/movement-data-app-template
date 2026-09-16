import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart' show Colors;
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:research_steps_template/bankid/controller.dart';
import 'package:research_steps_template/bankid/models.dart';
import 'package:research_steps_template/theme.dart';

class LoginScreen extends HookConsumerWidget {
  const LoginScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final invitation = useTextEditingController();
    final state = ref.watch(bankIdProvider);
    final controller = ref.read(bankIdProvider.notifier);
    useEffect(() {
      controller.restore();
      return null;
    }, const []);
    final order = state.attempt;
    final document = state.document;
    final disabled = state.busy;

    return AppScaffold(
      title: document != null ? 'Study consent' : 'Join the study',
      child: ListView(
        children: [
          if (state.error != null) ...[
            AppCard(
              backgroundColor: AppTheme.sand,
              child: Semantics(
                liveRegion: true,
                child: Text(state.error!, style: AppTheme.body),
              ),
            ),
            const SizedBox(height: 16),
          ],
          if (state.busy)
            const Padding(
              padding: EdgeInsets.all(12),
              child: CupertinoActivityIndicator(),
            ),
          if (!state.begun) ...[
            const Text(
              'Enroll with your study invitation',
              style: AppTheme.sectionTitle,
            ),
            const SizedBox(height: 12),
            const Text(
              'Enter the invitation code from your study team. You will review the study consent and sign it using BankID.',
              style: AppTheme.body,
            ),
            const SizedBox(height: 20),
            CupertinoTextField(
              controller: invitation,
              placeholder: 'Invitation code',
              autocorrect: false,
              enableSuggestions: false,
              padding: const EdgeInsets.all(16),
            ),
            const SizedBox(height: 16),
            CupertinoButton.filled(
              onPressed: disabled
                  ? null
                  : () => controller.begin(invitationCode: invitation.text),
              child: const Text('Review study consent'),
            ),
            const SizedBox(height: 12),
            CupertinoButton(
              onPressed: disabled
                  ? null
                  : () => controller.begin(returning: true),
              child: const Text('Already enrolled? Sign in with BankID'),
            ),
          ] else if (order != null) ...[
            AppCard(
              backgroundColor: order.accepted ? AppTheme.foam : Colors.white,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    order.accepted
                        ? (order.purpose == 'sign'
                              ? 'Consent signed'
                              : 'Identity confirmed')
                        : 'Continue with BankID',
                    style: AppTheme.sectionTitle,
                  ),
                  const SizedBox(height: 12),
                  Semantics(
                    liveRegion: true,
                    child: Text(bankIdMessage(order), style: AppTheme.body),
                  ),
                  if (order.accepted && document != null) ...[
                    const SizedBox(height: 16),
                    Text(document.title, style: AppTheme.cardTitle),
                    Text(
                      'Consent version ${document.version}',
                      style: AppTheme.bodyMuted,
                    ),
                  ],
                  if (order.qrData != null) ...[
                    const SizedBox(height: 20),
                    const Text(
                      'Scan this code with BankID on your other device.',
                      style: AppTheme.body,
                    ),
                    const SizedBox(height: 12),
                    Center(
                      child: Semantics(
                        label:
                            'BankID QR code. Scan with the BankID app on your other device.',
                        child: ExcludeSemantics(
                          child: QrImageView(
                            data: order.qrData!,
                            size: 240,
                            backgroundColor: Colors.white,
                          ),
                        ),
                      ),
                    ),
                    ExcludeSemantics(
                      child: Center(
                        child: Text(
                          'Scan within ${order.qrSecondsRemaining} seconds',
                          style: AppTheme.bodyMuted,
                        ),
                      ),
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 16),
            if (order.accepted)
              CupertinoButton.filled(
                onPressed: disabled ? null : controller.finish,
                child: const Text('Continue to Apple Health'),
              ),
            if (order.pending && order.mode == 'sameDevice')
              CupertinoButton.filled(
                onPressed: disabled ? null : controller.openBankId,
                child: const Text('Open BankID'),
              ),
            if (order.canExtendQR)
              CupertinoButton.filled(
                onPressed: disabled ? null : controller.extendQR,
                child: const Text('Show a new QR code'),
              ),
            if (order.pending)
              CupertinoButton(
                onPressed: disabled ? null : controller.cancel,
                child: const Text('Cancel request'),
              ),
            if (!order.pending && !order.accepted)
              CupertinoButton.filled(
                onPressed: disabled
                    ? null
                    : () => order.purpose == 'sign'
                          ? controller.reviewAgain()
                          : controller.start(order.mode),
                child: Text(
                  order.purpose == 'sign'
                      ? 'Review consent and try again'
                      : 'Try again',
                ),
              ),
            if (state.error != null)
              CupertinoButton(
                onPressed: disabled ? null : controller.refresh,
                child: const Text('Check request status'),
              ),
          ] else if (document != null) ...[
            Text(document.title, style: AppTheme.sectionTitle),
            const SizedBox(height: 8),
            Text('Version ${document.version}', style: AppTheme.bodyMuted),
            const SizedBox(height: 16),
            AppCard(child: Text(document.text, style: AppTheme.body)),
            const SizedBox(height: 16),
            Row(
              children: [
                CupertinoSwitch(
                  value: state.reviewed,
                  onChanged: disabled ? null : controller.setReviewed,
                ),
                const SizedBox(width: 12),
                const Expanded(
                  child: Text(
                    'I have read the consent above and want to sign it.',
                    style: AppTheme.body,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 20),
            CupertinoButton.filled(
              onPressed: disabled || !state.reviewed
                  ? null
                  : () => controller.start('sameDevice'),
              child: const Text('Sign with BankID'),
            ),
            CupertinoButton(
              onPressed: disabled || !state.reviewed
                  ? null
                  : () => controller.start('qr'),
              child: const Text('Use BankID on another device'),
            ),
          ] else if (!state.returning) ...[
            CupertinoButton.filled(
              onPressed: disabled ? null : controller.reviewAgain,
              child: const Text('Load study consent'),
            ),
          ] else ...[
            const Text('Sign in with BankID', style: AppTheme.sectionTitle),
            const SizedBox(height: 12),
            const Text(
              'Use the BankID you used when enrolling in the study.',
              style: AppTheme.body,
            ),
            const SizedBox(height: 20),
            CupertinoButton.filled(
              onPressed: disabled ? null : () => controller.start('sameDevice'),
              child: const Text('Open BankID'),
            ),
            CupertinoButton(
              onPressed: disabled ? null : () => controller.start('qr'),
              child: const Text('Use BankID on another device'),
            ),
          ],
          if (state.begun && !state.busy && order?.accepted != true) ...[
            const SizedBox(height: 24),
            CupertinoButton(
              onPressed: controller.reset,
              child: const Text('Start over'),
            ),
          ],
          const SizedBox(height: 24),
        ],
      ),
    );
  }
}
