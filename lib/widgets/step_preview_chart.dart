import 'package:flutter/cupertino.dart';
import 'package:research_steps_template/state/health.dart';
import 'package:research_steps_template/theme.dart';

class StepPreviewChart extends StatelessWidget {
  final List<DailyStepTotal> data;

  const StepPreviewChart({super.key, required this.data});

  @override
  Widget build(BuildContext context) {
    if (data.isEmpty) {
      return const SizedBox.shrink();
    }

    final maxValue = data
        .map((entry) => entry.steps)
        .reduce((left, right) => left > right ? left : right);

    return SizedBox(
      height: 120,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          for (final entry in data)
            Expanded(
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 2),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    Expanded(
                      child: Align(
                        alignment: Alignment.bottomCenter,
                        child: Container(
                          decoration: BoxDecoration(
                            color: entry == data.last
                                ? AppTheme.accent
                                : AppTheme.ocean,
                            borderRadius: BorderRadius.circular(999),
                          ),
                          height: maxValue == 0
                              ? 4
                              : 12 + (84 * (entry.steps / maxValue)),
                        ),
                      ),
                    ),
                    const SizedBox(height: 6),
                    Text(
                      '${entry.date.month}/${entry.date.day}',
                      style: const TextStyle(
                        fontSize: 10,
                        color: CupertinoColors.systemGrey,
                      ),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}
