import importlib.util
from pathlib import Path
import struct
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("report", Path(__file__).with_name("build-app-report.py"))
report = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(report)


class CoverageTests(unittest.TestCase):
    def test_missing_or_duplicate_cases_fail(self):
        for cases in [[], [{"id": "a", "images": []}, {"id": "a", "images": []}]]:
            with self.assertRaises(ValueError):
                report.validate_manifest({"requiredCases": ["a"], "cases": cases}, Path("."))

    def test_missing_invalid_and_escaping_images_fail(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            manifest = {"requiredCases": ["a"], "cases": [{"id": "a", "images": ["image.png"]}]}
            with self.assertRaises(FileNotFoundError):
                report.validate_manifest(manifest, root)
            (root / "image.png").write_bytes(b"not a screenshot")
            with self.assertRaises(ValueError):
                report.validate_manifest(manifest, root)
            manifest["cases"][0]["images"] = ["../image.png"]
            with self.assertRaises(ValueError):
                report.validate_manifest(manifest, root)

    def test_report_escapes_content_and_embeds_images(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            png = b"\x89PNG\r\n\x1a\n" + b"\0" * 8 + struct.pack(">II", 780, 1688) + b"\0" * 16
            (root / "image.png").write_bytes(png)
            manifest = {"appName": "<script>bad()</script>", "sourceKind": "study", "questionCount": 1,
                        "requiredCases": ["a"], "cases": [{"id": "a", "title": '<unsafe "title">', "group": "Group", "path": "/home", "images": ["image.png"]}]}
            report.validate_manifest(manifest, root)
            result = report.render_report(manifest, {"checks": [], "generatedAt": "now", "revision": "test", "font": "Roboto", "definitionDigest": "abc"}, root)
            self.assertNotIn("<script>bad()</script>", result)
            self.assertIn("&lt;script&gt;bad()&lt;/script&gt;", result)
            self.assertIn("data:image/png;base64,", result)
            self.assertNotIn("@@", result)


if __name__ == "__main__":
    unittest.main()
