import 'package:flutter/cupertino.dart';
import 'demo/data.dart';
import 'demo/gallery.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(DemoGallery(study: DemoStudy.generated()));
}
