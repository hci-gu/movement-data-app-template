import 'package:flutter/cupertino.dart';

Future<T?> showAppSheet<T>(BuildContext context, Widget child) =>
    showCupertinoModalPopup<T>(
      context: context,
      builder: (context) => SizedBox(
        height: MediaQuery.sizeOf(context).height * .92,
        child: child,
      ),
    );
