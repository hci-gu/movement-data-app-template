import importlib.util
from copy import deepcopy
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
                        "requiredCases": ["a"], "cases": [{"id": "a", "title": '<unsafe "title">', "group": "Group", "path": "/home", "images": ["image.png", "image.png"]}]}
            report.validate_manifest(manifest, root)
            result = report.render_report(manifest, {"checks": [], "generatedAt": "now", "revision": "test", "font": "Roboto", "definitionDigest": "abc"}, root)
            self.assertNotIn("<script>bad()</script>", result)
            self.assertIn("&lt;script&gt;bad()&lt;/script&gt;", result)
            self.assertIn("data:image/png;base64,", result)
            self.assertNotIn("@@", result)
            self.assertNotIn("/home", result)
            self.assertNotIn("<footer>", result)
            self.assertIn('data-search="Group', result)
            self.assertEqual(result.count('src="data:image/png;base64,'), 2)
            self.assertIn("section 2 of 2", result)


class StudyContentTests(unittest.TestCase):
    def setUp(self):
        self.source = {"sourceKind": "study", "consent": {"title": "Movement study", "text": "Approved study text."}, "questionnaires": []}
        self.manifest = {
            "studyContent": {"appName": "Movement study", "introduction": "We study movement after treatment.",
                             "supportEmail": "study@university.se", "privacyContact": "privacy@university.se"},
            "cases": [{"id": "intro", "participantText": ["Movement study"]}, {"id": "loading", "participantText": []}],
        }

    def test_final_content_and_input_hints_are_allowed(self):
        self.source["questionnaires"] = [{"text": "Email", "placeholder": "name@example.org"}]
        report.validate_study_source(self.source)
        report.validate_study_manifest(self.manifest)
        report.validate_copy("A draft response can be edited before submission. Please try again.", "runtime copy")

    def test_template_or_unfinished_source_is_rejected(self):
        template = deepcopy(self.source)
        template["sourceKind"] = "template"
        with self.assertRaisesRegex(ValueError, "never substitutes"):
            report.validate_study_source(template)
        for text in ["", None, "DEMONSTRATION ONLY — not research consent", "TEST ENVIRONMENT ONLY - BANKID SIGNING TEST", "[Insert approved consent]", "UTKAST\nStudy text", "Contact research@example.org"]:
            with self.subTest(text=text):
                source = deepcopy(self.source)
                source["consent"]["text"] = text
                with self.assertRaises(ValueError):
                    report.validate_study_source(source)
        self.source["questionnaires"] = [{"text": "TODO: approved question"}]
        with self.assertRaisesRegex(ValueError, "questionnaires"):
            report.validate_study_source(self.source)

    def test_missing_metadata_defaults_and_example_contacts_are_rejected(self):
        for field, value in [("appName", "Research Steps"), ("supportEmail", "research@example.org"),
                             ("privacyContact", "privacy@example.com"), ("introduction", "TBD"), ("appName", "")]:
            with self.subTest(field=field, value=value):
                manifest = deepcopy(self.manifest)
                manifest["studyContent"][field] = value
                with self.assertRaises(ValueError):
                    report.validate_study_manifest(manifest)
        del self.manifest["studyContent"]
        with self.assertRaises(ValueError):
            report.validate_study_manifest(self.manifest)

    def test_rendered_copy_is_checked_and_missing_capture_fails(self):
        self.manifest["cases"][0]["participantText"].append("[draft]")
        with self.assertRaisesRegex(ValueError, "intro"):
            report.validate_study_manifest(self.manifest)
        del self.manifest["cases"][0]["participantText"]
        with self.assertRaisesRegex(ValueError, "Missing participant copy"):
            report.validate_study_manifest(self.manifest)


if __name__ == "__main__":
    unittest.main()
