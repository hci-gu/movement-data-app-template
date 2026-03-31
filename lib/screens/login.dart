import 'package:flutter/cupertino.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:research_steps_template/app_config.dart';
import 'package:research_steps_template/state/auth.dart';
import 'package:research_steps_template/theme.dart';
import 'package:research_steps_template/widgets/consent_modal.dart';
import 'package:research_steps_template/widgets/participant_id_input.dart';

class LoginScreen extends HookConsumerWidget {
  const LoginScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final controller = useTextEditingController();
    final canContinue = useState(false);

    useEffect(() {
      void updateState() {
        canContinue.value = AppConfig.isValidParticipantId(controller.text);
      }

      controller.addListener(updateState);
      return () => controller.removeListener(updateState);
    }, [controller]);

    return AppScaffold(
      title: AppConfig.participantIdLabel,
      child: Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.symmetric(vertical: 24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              AppCard(
                backgroundColor: AppTheme.foam,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: const [
                    Text('Enroll participant', style: AppTheme.sectionTitle),
                    SizedBox(height: 12),
                    Text(
                      'This template creates or reuses a study participant account using a generic identifier. Replace this flow if your study needs invitation codes, SSO, or a different enrollment model.',
                      style: AppTheme.body,
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              AppCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [ParticipantIdInput(controller: controller)],
                ),
              ),
              const SizedBox(height: 16),
              SizedBox(
                width: double.infinity,
                child: CupertinoButton.filled(
                  onPressed: canContinue.value
                      ? () async {
                          final consentAccepted =
                              await showCupertinoModalPopup<bool>(
                                context: context,
                                builder: (context) => const ConsentModal(),
                              ) ??
                              false;

                          if (!consentAccepted || !context.mounted) {
                            return;
                          }

                          final participantId =
                              AppConfig.normalizeParticipantId(controller.text);

                          try {
                            await ref
                                .read(authProvider.notifier)
                                .signup(
                                  participantId,
                                  consentAccepted: consentAccepted,
                                );
                          } catch (_) {
                            if (!context.mounted) {
                              return;
                            }

                            await showCupertinoDialog<void>(
                              context: context,
                              builder: (context) => CupertinoAlertDialog(
                                title: const Text('Enrollment failed'),
                                content: const Text(
                                  'The participant account could not be created or restored. Check the API base URL and backend availability.',
                                ),
                                actions: [
                                  CupertinoDialogAction(
                                    isDefaultAction: true,
                                    onPressed: () {
                                      Navigator.of(context).pop();
                                    },
                                    child: const Text('OK'),
                                  ),
                                ],
                              ),
                            );
                          }
                        }
                      : null,
                  child: const Text('Continue'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
