import 'package:flutter/cupertino.dart';
import 'package:html/dom.dart' as dom;
import 'package:html/parser.dart' as html;
import 'package:research_steps_template/theme.dart';
import '../models.dart';

// Render text and basic emphasis only. No scripts, images, embedded content,
// remote resources, or executable links are supported by questionnaire copy.
class QuestionnaireRichText extends StatelessWidget {
  final String source;
  const QuestionnaireRichText(this.source, {super.key});
  @override
  Widget build(BuildContext context) {
    return Text.rich(
      TextSpan(children: _nodes(html.parseFragment(source).nodes)),
      style: AppTheme.body,
    );
  }

  List<InlineSpan> _nodes(List<dom.Node> nodes) => [
    for (final node in nodes) ...render(node),
  ];
  List<InlineSpan> render(dom.Node node) {
    if (node is dom.Text) return [TextSpan(text: node.data)];
    if (node is! dom.Element ||
        [
          'script',
          'style',
          'iframe',
          'img',
          'object',
          'svg',
        ].contains(node.localName)) {
      return [];
    }
    final tag = node.localName;
    if (tag == 'br') return [const TextSpan(text: '\n')];
    return [
      if (tag == 'li') const TextSpan(text: '• '),
      TextSpan(
        style: TextStyle(
          fontWeight: ['b', 'strong'].contains(tag) ? FontWeight.w600 : null,
          fontStyle: ['i', 'em'].contains(tag) ? FontStyle.italic : null,
        ),
        children: _nodes(node.nodes),
      ),
      if (['p', 'div', 'li', 'ul', 'ol'].contains(tag))
        const TextSpan(text: '\n'),
    ];
  }
}

class QuestionCard extends StatelessWidget {
  final QuestionDefinition question;
  final String? error;
  final Widget child;
  const QuestionCard({
    super.key,
    required this.question,
    required this.child,
    this.error,
  });
  @override
  Widget build(BuildContext context) => AppCard(
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (question.introduction.isNotEmpty) ...[
          QuestionnaireRichText(question.introduction),
          const SizedBox(height: 8),
        ],
        Text(question.text, style: AppTheme.cardTitle),
        const SizedBox(height: 12),
        child,
        if (error != null) ...[
          const SizedBox(height: 8),
          Semantics(
            liveRegion: true,
            child: Text(
              error!,
              style: const TextStyle(color: CupertinoColors.systemRed),
            ),
          ),
        ],
      ],
    ),
  );
}
