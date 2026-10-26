#!/usr/bin/env python3
"""Normalize artwork to a 1024px master and install this template's iOS icons."""
from __future__ import annotations

import argparse
from io import BytesIO
import json
from pathlib import Path
import re
import sys
import tempfile

try:
    from PIL import Image, ImageCms, ImageOps
except ImportError:
    raise SystemExit("Install icon dependencies: python3 -m pip install -r scripts/requirements-icons.txt")

ROOT = Path(__file__).resolve().parents[1]
MASTER = Path("assets/icon/app-icon-1024.png")
CATALOG = Path("ios/Runner/Assets.xcassets/AppIcon.appiconset")
PROJECT = Path("ios/Runner.xcodeproj/project.pbxproj")
# Points and scale remain separate in Contents.json. Includes existing legacy
# slots and iPad slots even though Runner currently targets iPhone only.
SLOTS = (
    ("iphone", "20x20", "2x", 40),
    ("iphone", "20x20", "3x", 60),
    ("iphone", "29x29", "1x", 29),
    ("iphone", "29x29", "2x", 58),
    ("iphone", "29x29", "3x", 87),
    ("iphone", "40x40", "2x", 80),
    ("iphone", "40x40", "3x", 120),
    ("iphone", "60x60", "2x", 120),
    ("iphone", "60x60", "3x", 180),
    ("ipad", "20x20", "1x", 20),
    ("ipad", "20x20", "2x", 40),
    ("ipad", "29x29", "1x", 29),
    ("ipad", "29x29", "2x", 58),
    ("ipad", "40x40", "1x", 40),
    ("ipad", "40x40", "2x", 80),
    ("ipad", "76x76", "1x", 76),
    ("ipad", "76x76", "2x", 152),
    ("ipad", "83.5x83.5", "2x", 167),
    ("ios-marketing", "1024x1024", "1x", 1024),
)
SRGB = ImageCms.ImageCmsProfile(ImageCms.createProfile("sRGB"))


def filename(pixels: int) -> str:
    return "appstore.png" if pixels == 1024 else f"Icon-{pixels}.png"


def manifest() -> dict:
    return {
        "images": [
            {"idiom": idiom, "size": size, "scale": scale, "filename": filename(pixels)}
            for idiom, size, scale, pixels in SLOTS
        ],
        "info": {"author": "xcode", "version": 1},
    }


def check_layout(data: dict) -> None:
    """Refuse to overwrite a different platform or custom appearance catalog."""
    expected = {slot[:3] for slot in SLOTS}
    actual = []
    if set(data) != {"images", "info"} or not isinstance(data["images"], list):
        raise ValueError("Unsupported AppIcon catalog; this helper expects the template's All Sizes layout.")
    for item in data["images"]:
        if not isinstance(item, dict) or set(item) - {"idiom", "size", "scale", "filename"}:
            raise ValueError("Custom icon appearances/properties need separate handling; catalog was not changed.")
        actual.append(tuple(item.get(key) for key in ("idiom", "size", "scale")))
        name = item.get("filename")
        if name is not None and (
            not isinstance(name, str) or Path(name).name != name
            or "\\" in name or not name.lower().endswith(".png")
        ):
            raise ValueError(f"Invalid icon filename: {name!r}")
    if len(actual) != len(expected) or set(actual) != expected:
        raise ValueError("AppIcon slots differ from this template. Review the catalog instead of overwriting it.")


def project_layout(root: Path) -> dict:
    project = (root / PROJECT).read_text(encoding="utf-8")
    names = re.findall(r'ASSETCATALOG_COMPILER_APPICON_NAME\s*=\s*"?([^;"\s]+)"?\s*;', project)
    if not names or any(name != "AppIcon" for name in names):
        raise ValueError("Runner build settings must select AppIcon; review the Xcode project first.")
    data = json.loads((root / CATALOG / "Contents.json").read_text(encoding="utf-8"))
    check_layout(data)
    return data


def normalize(source: Path, *, crop: bool = False, background: str | None = None) -> Image.Image:
    if background is not None and not re.fullmatch(r"#[0-9a-fA-F]{6}", background):
        raise ValueError("--background must be a quoted #RRGGBB color.")
    with Image.open(source) as original:
        if getattr(original, "n_frames", 1) != 1:
            raise ValueError("Use a single still image for the app icon.")
        image = ImageOps.exif_transpose(original)
        if min(image.size) < 1024:
            raise ValueError("Generate artwork at least 1024 pixels on each side; this script does not upscale.")
        if image.width != image.height and not crop:
            raise ValueError("Artwork is not square. Regenerate it or use --crop for an explicit center crop.")
        alpha = image.convert("RGBA").getchannel("A")
        profile = image.info.get("icc_profile")
        if profile:
            color = image if image.mode in ("RGB", "CMYK", "LAB", "L") else image.convert("RGB")
            try:
                color = ImageCms.profileToProfile(
                    color, ImageCms.ImageCmsProfile(BytesIO(profile)), SRGB, outputMode="RGB"
                )
            except (OSError, ValueError, ImageCms.PyCMSError) as error:
                raise ValueError("The artwork's color profile could not be converted to sRGB.") from error
        else:
            if image.mode not in ("1", "L", "LA", "P", "RGB", "RGBA"):
                raise ValueError("Unprofiled CMYK/LAB artwork is ambiguous; export an sRGB source first.")
            color = image.convert("RGB")  # Unprofiled RGB input is assumed sRGB.
        if alpha.getextrema()[0] < 255:
            if background is None:
                raise ValueError("Artwork contains transparency. Supply --background '#RRGGBB' or opaque artwork.")
            color.putalpha(alpha)
            color = Image.alpha_composite(Image.new("RGBA", image.size, background), color).convert("RGB")
        if image.width != image.height:
            side = min(color.size)
            left, top = (color.width - side) // 2, (color.height - side) // 2
            color = color.crop((left, top, left + side, top + side))
        return color.resize((1024, 1024), Image.Resampling.LANCZOS)


def png_bytes(image: Image.Image) -> bytes:
    buffer = BytesIO()
    # A fresh image drops EXIF, palette transparency and inherited metadata.
    clean = Image.new("RGB", image.size)
    clean.paste(image)
    clean.save(buffer, format="PNG", icc_profile=SRGB.tobytes())
    return buffer.getvalue()


def read_icon(path: Path, pixels: int) -> Image.Image:
    with Image.open(path) as image:
        image.load()
        if image.format != "PNG" or image.size != (pixels, pixels):
            raise ValueError(f"{path}: expected a {pixels}x{pixels} PNG.")
        if image.mode != "RGB" or "transparency" in image.info:
            raise ValueError(f"{path}: icons must be RGB with no alpha channel or transparency.")
        profile = image.info.get("icc_profile")
        if not profile:
            raise ValueError(f"{path}: missing sRGB color profile.")
        try:
            description = ImageCms.getProfileDescription(ImageCms.ImageCmsProfile(BytesIO(profile)))
        except (OSError, ValueError, ImageCms.PyCMSError) as error:
            raise ValueError(f"{path}: invalid color profile.") from error
        if "srgb" not in description.lower():
            raise ValueError(f"{path}: expected sRGB color profile.")
        return image.copy()


def check_assets(master_path: Path, catalog: Path) -> None:
    master = read_icon(master_path, 1024)
    data = json.loads((catalog / "Contents.json").read_text(encoding="utf-8"))
    check_layout(data)
    sizes = {slot[:3]: slot[3] for slot in SLOTS}
    referenced = set()
    for item in data["images"]:
        name = item.get("filename")
        if not name:
            raise ValueError(f"Unpopulated icon slot: {item}")
        pixels = sizes[tuple(item[key] for key in ("idiom", "size", "scale"))]
        actual = read_icon(catalog / name, pixels)
        expected = master.resize((pixels, pixels), Image.Resampling.LANCZOS)
        if actual.tobytes() != expected.tobytes():
            raise ValueError(f"{name} is stale or does not match the 1024px master. Regenerate the set.")
        referenced.add(name)
    extras = {p.name for p in catalog.iterdir() if p.suffix.lower() == ".png"} - referenced
    if extras:
        raise ValueError(f"Unassigned PNGs in AppIcon: {', '.join(sorted(extras))}. Review these files manually.")


def atomic_write(path: Path, data: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as handle:
        temporary = Path(handle.name)
        handle.write(data)
    try:
        temporary.replace(path)
    finally:
        temporary.unlink(missing_ok=True)


def build(root: Path, source: Path, *, crop: bool = False, background: str | None = None) -> None:
    old = project_layout(root)
    catalog = root / CATALOG
    if source.resolve().is_relative_to(catalog.resolve()):
        raise ValueError("Keep the source outside AppIcon.appiconset, for example assets/icon/source.png.")
    old_names = {item["filename"] for item in old["images"] if item.get("filename")}
    extras = {p.name for p in catalog.iterdir() if p.suffix.lower() == ".png"} - old_names
    if extras:
        raise ValueError(f"Review unassigned AppIcon files before replacing the set: {sorted(extras)}")
    master = normalize(source, crop=crop, background=background)
    # Validate a complete staged set before changing any project assets.
    with tempfile.TemporaryDirectory(prefix="study-icons-") as temporary:
        staging = Path(temporary)
        (staging / "master.png").write_bytes(png_bytes(master))
        staged_catalog = staging / "AppIcon.appiconset"
        staged_catalog.mkdir()
        for pixels in sorted({slot[3] for slot in SLOTS}):
            image = master.resize((pixels, pixels), Image.Resampling.LANCZOS)
            (staged_catalog / filename(pixels)).write_bytes(png_bytes(image))
        contents = (json.dumps(manifest(), indent=2) + "\n").encode()
        (staged_catalog / "Contents.json").write_bytes(contents)
        check_assets(staging / "master.png", staged_catalog)
        for file in staged_catalog.glob("*.png"):
            atomic_write(catalog / file.name, file.read_bytes())
        atomic_write(root / MASTER, (staging / "master.png").read_bytes())
        atomic_write(catalog / "Contents.json", contents)
    new_names = {filename(slot[3]) for slot in SLOTS}
    # Remove only obsolete images referenced by the old manifest.
    for name in old_names - new_names:
        (catalog / name).unlink(missing_ok=True)
    check_assets(root / MASTER, catalog)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", nargs="?", type=Path, help="Generated or supplied square artwork, at least 1024px")
    parser.add_argument("--check", action="store_true", help="Validate installed icons without writing any files")
    parser.add_argument("--crop", action="store_true", help="Explicitly center-crop nonsquare artwork before resizing")
    parser.add_argument("--background", help="Flatten transparent artwork on an explicit #RRGGBB color")
    parser.add_argument("--project-root", type=Path, default=ROOT, help="Template root (defaults to this repository)")
    args = parser.parse_args()
    root = args.project_root.expanduser().resolve()
    if args.check:
        if args.source or args.crop or args.background:
            parser.error("--check cannot be combined with a source or conversion options")
        project_layout(root)
        check_assets(root / MASTER, root / CATALOG)
        print("Verified 1024px RGB/sRGB master, all 19 iOS slots, matching PNGs and AppIcon build settings.")
    else:
        if args.source is None:
            parser.error("Provide a source image, or use --check")
        build(root, args.source.expanduser(), crop=args.crop, background=args.background)
        print(f"Master: {root / MASTER}")
        print(f"Installed 13 PNGs in {root / CATALOG} for all 19 iPhone/iPad/App Store slots.")
        print("Next: inspect small icons, build iOS, then validate the signed archive before App Store upload.")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"Icon preparation failed: {error}", file=sys.stderr)
        sys.exit(1)
