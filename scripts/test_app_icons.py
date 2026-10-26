import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

from PIL import Image, ImageCms

SPEC = importlib.util.spec_from_file_location("icons", Path(__file__).with_name("prepare-app-icons.py"))
icons = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(icons)


class AppIconTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.catalog = self.root / icons.CATALOG
        self.catalog.mkdir(parents=True)
        self.project = self.root / icons.PROJECT
        self.project.parent.mkdir(parents=True)
        self.project.write_text('ASSETCATALOG_COMPILER_APPICON_NAME = AppIcon;\n' * 3)
        # Start from the real template mapping, including the empty iPad slots.
        data = json.loads((icons.ROOT / icons.CATALOG / "Contents.json").read_text())
        (self.catalog / "Contents.json").write_text(json.dumps(data))
        for item in data["images"]:
            if item.get("filename"):
                (self.catalog / item["filename"]).write_bytes(b"old placeholder")
        self.source = self.root / "source.png"
        image = Image.new("RGBA", (1536, 1536), (18, 80, 100, 255))
        image.paste((245, 170, 30, 255), (500, 500, 1036, 1036))
        image.save(self.source)

    def test_exports_every_slot_from_the_normalized_master_and_preserves_source(self):
        original = self.source.read_bytes()
        icons.build(self.root, self.source)
        icons.check_assets(self.root / icons.MASTER, self.catalog)
        self.assertEqual(self.source.read_bytes(), original)
        master = icons.read_icon(self.root / icons.MASTER, 1024)
        self.assertEqual(master.mode, "RGB")
        self.assertEqual(len(list(self.catalog.glob("*.png"))), 13)
        data = json.loads((self.catalog / "Contents.json").read_text())
        self.assertEqual(len(data["images"]), 19)
        ipad_pro = next(item for item in data["images"] if item["size"] == "83.5x83.5")
        with Image.open(self.catalog / ipad_pro["filename"]) as image:
            self.assertEqual(image.size, (167, 167))
        with Image.open(self.catalog / "appstore.png") as image:
            self.assertEqual(image.size, (1024, 1024))
        icons.build(self.root, self.source)
        icons.check_assets(self.root / icons.MASTER, self.catalog)

    def test_transparency_requires_a_deliberate_background_including_palette_png(self):
        image = Image.new("P", (1024, 1024), 0)
        image.putpalette([255, 0, 0] * 256)
        image.save(self.source, transparency=0)
        before = (self.catalog / "Contents.json").read_bytes()
        with self.assertRaisesRegex(ValueError, "transparency"):
            icons.build(self.root, self.source)
        self.assertEqual((self.catalog / "Contents.json").read_bytes(), before)
        self.assertFalse((self.root / icons.MASTER).exists())
        icons.build(self.root, self.source, background="#123456")
        master = icons.read_icon(self.root / icons.MASTER, 1024)
        self.assertEqual(master.getpixel((0, 0)), (18, 52, 86))

    def test_no_silent_stretch_or_upscale(self):
        Image.new("RGB", (1536, 1024), "blue").save(self.source)
        with self.assertRaisesRegex(ValueError, "not square"):
            icons.normalize(self.source)
        self.assertEqual(icons.normalize(self.source, crop=True).size, (1024, 1024))
        Image.new("RGB", (512, 512), "blue").save(self.source)
        with self.assertRaisesRegex(ValueError, "does not upscale"):
            icons.normalize(self.source)

    def test_embedded_profile_is_converted_not_just_relabelled(self):
        source = self.root / "lab.tiff"
        lab = Image.new("LAB", (1024, 1024), (150, 160, 170))
        profile = ImageCms.ImageCmsProfile(ImageCms.createProfile("LAB"))
        lab.save(source, icc_profile=profile.tobytes())
        expected = ImageCms.profileToProfile(lab, profile, icons.SRGB, outputMode="RGB")
        actual = icons.normalize(source)
        self.assertEqual(actual.getpixel((20, 20)), expected.getpixel((20, 20)))

    def test_checks_catch_missing_stale_and_alpha_images(self):
        icons.build(self.root, self.source)
        file = self.catalog / "Icon-167.png"
        good = file.read_bytes()
        file.unlink()
        with self.assertRaises(FileNotFoundError):
            icons.check_assets(self.root / icons.MASTER, self.catalog)
        file.write_bytes(good)
        image = icons.read_icon(file, 167)
        image.putpixel((0, 0), (0, 0, 0))
        file.write_bytes(icons.png_bytes(image))
        with self.assertRaisesRegex(ValueError, "stale"):
            icons.check_assets(self.root / icons.MASTER, self.catalog)
        image.convert("RGBA").save(file, icc_profile=icons.SRGB.tobytes())
        with self.assertRaisesRegex(ValueError, "no alpha"):
            icons.check_assets(self.root / icons.MASTER, self.catalog)

    def test_custom_appearances_unassigned_files_and_wrong_build_settings_fail(self):
        contents = self.catalog / "Contents.json"
        original = contents.read_bytes()
        data = json.loads(original)
        data["images"][0]["appearances"] = [{"appearance": "luminosity", "value": "dark"}]
        contents.write_text(json.dumps(data))
        with self.assertRaisesRegex(ValueError, "Custom icon"):
            icons.build(self.root, self.source)
        self.assertIn("appearances", json.loads(contents.read_text())["images"][0])
        contents.write_bytes(original)
        extra = self.catalog / "custom.png"
        extra.write_bytes(b"keep this")
        with self.assertRaisesRegex(ValueError, "unassigned"):
            icons.build(self.root, self.source)
        self.assertEqual(extra.read_bytes(), b"keep this")
        extra.unlink()
        self.project.write_text("ASSETCATALOG_COMPILER_APPICON_NAME = AnotherIcon;")
        with self.assertRaisesRegex(ValueError, "build settings"):
            icons.build(self.root, self.source)


if __name__ == "__main__":
    unittest.main()
