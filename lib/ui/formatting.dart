String formatDate(DateTime? value) {
  if (value == null) {
    return 'Not available';
  }

  final month = value.month.toString().padLeft(2, '0');
  final day = value.day.toString().padLeft(2, '0');
  return '${value.year}-$month-$day';
}

String formatDateTime(DateTime? value) {
  if (value == null) {
    return 'Not available';
  }

  final hour = value.hour.toString().padLeft(2, '0');
  final minute = value.minute.toString().padLeft(2, '0');
  return '${formatDate(value)} $hour:$minute';
}

String formatDateRange(DateTime? start, DateTime? end) {
  if (start == null || end == null) {
    return 'No date coverage available';
  }

  return '${formatDate(start)} to ${formatDate(end)}';
}

String formatInteger(num value) {
  final rounded = value.round().toString();
  final buffer = StringBuffer();

  for (var index = 0; index < rounded.length; index++) {
    final reverseIndex = rounded.length - index;
    buffer.write(rounded[index]);
    if (reverseIndex > 1 && reverseIndex % 3 == 1) {
      buffer.write(' ');
    }
  }

  return buffer.toString();
}

DateTime normalizeDate(DateTime value) {
  return DateTime(value.year, value.month, value.day);
}
