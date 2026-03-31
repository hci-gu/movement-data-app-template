import 'package:flutter/cupertino.dart';

class AppTheme {
  static const Color ink = Color(0xFF183247);
  static const Color ocean = Color(0xFF1C6E8C);
  static const Color foam = Color(0xFFEAF6FA);
  static const Color sand = Color(0xFFF6EFE4);
  static const Color accent = Color(0xFFF28C48);
  static const Color border = Color(0x1F183247);

  static const double basePadding = 16;

  static const TextStyle heroTitle = TextStyle(
    fontSize: 34,
    fontWeight: FontWeight.w700,
    color: ink,
  );

  static const TextStyle sectionTitle = TextStyle(
    fontSize: 24,
    fontWeight: FontWeight.w700,
    color: ink,
  );

  static const TextStyle cardTitle = TextStyle(
    fontSize: 17,
    fontWeight: FontWeight.w700,
    color: ink,
  );

  static const TextStyle body = TextStyle(
    fontSize: 15,
    height: 1.35,
    color: ink,
  );

  static const TextStyle bodyMuted = TextStyle(
    fontSize: 14,
    height: 1.35,
    color: CupertinoColors.systemGrey,
  );

  static CupertinoThemeData get cupertinoTheme => const CupertinoThemeData(
    primaryColor: ocean,
    scaffoldBackgroundColor: Color(0xFFF7FAFC),
    barBackgroundColor: Color(0xFFF7FAFC),
    textTheme: CupertinoTextThemeData(primaryColor: ink),
  );
}

class AppScaffold extends StatelessWidget {
  final Widget child;
  final String? title;
  final bool noBackButton;
  final bool withHorizontalPadding;

  const AppScaffold({
    super.key,
    required this.child,
    this.title,
    this.noBackButton = false,
    this.withHorizontalPadding = true,
  });

  @override
  Widget build(BuildContext context) {
    final paddedChild = withHorizontalPadding
        ? Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: AppTheme.basePadding,
            ),
            child: child,
          )
        : child;

    return CupertinoPageScaffold(
      navigationBar: CupertinoNavigationBar(
        middle: title == null ? null : Text(title!, style: AppTheme.cardTitle),
        leading: noBackButton ? const SizedBox.shrink() : null,
      ),
      child: DecoratedBox(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [Color(0xFFF4FBFD), Color(0xFFF8F6F2)],
          ),
        ),
        child: SafeArea(child: paddedChild),
      ),
    );
  }
}

class AppCard extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry padding;
  final Color backgroundColor;

  const AppCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(AppTheme.basePadding),
    this.backgroundColor = CupertinoColors.white,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: padding,
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: AppTheme.border),
        boxShadow: const [
          BoxShadow(
            color: Color(0x12000000),
            blurRadius: 16,
            offset: Offset(0, 8),
          ),
        ],
      ),
      child: child,
    );
  }
}
