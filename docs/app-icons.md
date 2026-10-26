# Study app icons

Generate study artwork when an image generator is available, then use the
checked-in helper to produce and install the iOS assets. This is a required part
of a study adaptation before the final build and app report. Keep the source
artwork, normalized master and installed assets in the study repository.

## What Apple requires and what this template uses

Apple supports a single 1024 × 1024 source or individually supplied sizes in an
asset catalog. This template already uses **All Sizes**; the helper preserves
that layout and fills every iPhone, iPad and App Store slot. Pixel dimensions are
the catalog's point size multiplied by its scale, so the iPad Pro 83.5pt @2x
entry is 167 × 167 pixels. See Apple's
[asset-catalog guide](https://developer.apple.com/documentation/xcode/configuring-your-app-icon)
and [format reference](https://developer.apple.com/library/archive/documentation/Xcode/Reference/xcode_ref-Asset_Catalog_Format/AppIconType.html).

The helper produces an opaque RGB PNG master with an embedded sRGB profile and
square corners. Apple applies the icon mask. For this flattened default icon,
remove the alpha channel even if all pixels appear opaque; App Store validation
can reject a large icon that contains alpha. This rule is for the default flat
asset, not transparent foreground layers or optional dark variants. See Apple's
[design guidance](https://developer.apple.com/design/human-interface-guidelines/app-icons)
and [large-icon validation guidance from Apple DTS](https://developer.apple.com/forums/thread/795411).

Apple also supports layered artwork through Icon Composer and additional dark
and tinted appearances. These require deliberate design and are outside this
flat-image helper. It refuses to overwrite a catalog with custom appearances or
a different layout. If the study adopts Icon Composer, update this workflow and
its checks instead of running the flat-image installer over that setup.

The App Store icon is included in the uploaded app build. It is **not** a separate
PNG upload to the app's listing. Setting up the icon does not sign, upload or
submit the app. See [Add an app icon in App Store Connect](https://developer.apple.com/help/app-store-connect/manage-app-information/add-an-app-icon/).

## 1. Generate or obtain the artwork

Read the study purpose, participant group and branding in `context/`. If image
generation is available, create a custom icon using that context. Use supplied
approved branding when the study requires it. If generation is unavailable, use
supplied artwork and explicitly report any remaining template placeholder.

A useful prompt structure is:

> Create a square app icon for [study purpose and participants], following
> [supplied palette/branding]. Use one clear, recognizable motif with strong
> contrast and enough margin to survive the system's rounded mask. Fill the
> square with an opaque background. Deliver artwork only, at least 1024 × 1024:
> no phone mockup, border, pre-rounded corners, baked-in shadow or small text.

Inspect the result for relevance, legibility and unintended text. Copy the
selected original into `assets/icon/source.png` (or another descriptive source
filename). Preserve its original resolution. A generated image stored only in a
tool's cache is not a project asset. Record the source/prompt and design rationale
in the study's documentation. Use the image generator for artwork; use the
resizing script below for deterministic file preparation.

## 2. Install the resizing dependency

The helper needs **Python 3.11+** and the pinned Pillow dependency. From the repo
root on macOS/Linux:

```sh
python3 -m venv build/icon-tools
build/icon-tools/bin/python -m pip install -r scripts/requirements-icons.txt
```

On Windows, use `py -3 -m venv build/icon-tools` and invoke
`build\icon-tools\Scripts\python.exe` instead of `build/icon-tools/bin/python`.
The environment stays under the ignored `build/` directory. These are development
dependencies; the app and backend do not need Pillow.

Alternatively, if `uv` is installed, commands can use
`uv run --python 3.14 --with-requirements scripts/requirements-icons.txt python`
in place of `build/icon-tools/bin/python`.

## 3. Normalize to 1024 and install all Xcode sizes

```sh
build/icon-tools/bin/python scripts/prepare-app-icons.py assets/icon/source.png
build/icon-tools/bin/python scripts/prepare-app-icons.py --check
```

The first command performs the complete pipeline:

1. Read and orient the source image. Require at least 1024 pixels on each side;
   do not silently upscale or stretch artwork.
2. Convert its embedded color profile to sRGB. Unprofiled RGB artwork is assumed
   sRGB; unprofiled CMYK/LAB is rejected. Remove the alpha channel.
3. Resize with Lanczos filtering to exactly **1024 × 1024**, saved as
   `assets/icon/app-icon-1024.png`.
4. Derive every smaller icon from that normalized master. Generate the complete
   set in a staging directory and validate it before installation.
5. Install the PNGs in `ios/Runner/Assets.xcassets/AppIcon.appiconset/` and update
   `Contents.json`. Remove only obsolete PNGs referenced by the previous manifest.
6. Verify the installed set and that the project's icon build settings select
   `AppIcon`. The existing asset catalog is already included in the Runner target;
   individual PNGs do not need new Xcode file references.

The original source is preserved. Rerunning replaces the generated master and
icon set. Keep originals outside `AppIcon.appiconset`. Unknown/unassigned PNGs,
custom variants and unexpected catalog layouts stop the command for review.

Transparent input needs an explicit background color selected for the design:

```sh
build/icon-tools/bin/python scripts/prepare-app-icons.py assets/icon/source.png --background '#123456'
```

For nonsquare artwork, prefer regenerating a square image. `--crop` explicitly
permits a centered square crop before resizing; inspect it for lost content.
Neither option changes the original source.

The installed PNG names are `Icon-<pixels>.png`, plus `appstore.png`. Matching
pixel dimensions reuse one file across slots:

| Use | Point size and scale | Pixel size |
| --- | --- | --- |
| iPhone notifications | 20pt @2x / @3x | 40 / 60 |
| iPhone settings, including existing legacy slot | 29pt @1x / @2x / @3x | 29 / 58 / 87 |
| iPhone search | 40pt @2x / @3x | 80 / 120 |
| iPhone Home Screen | 60pt @2x / @3x | 120 / 180 |
| iPad notifications | 20pt @1x / @2x | 20 / 40 |
| iPad settings | 29pt @1x / @2x | 29 / 58 |
| iPad search | 40pt @1x / @2x | 40 / 80 |
| iPad Home Screen | 76pt @1x / @2x | 76 / 152 |
| iPad Pro Home Screen | 83.5pt @2x | 167 |
| App Store (`ios-marketing`) | 1024pt @1x | 1024 |

This fills **19 catalog entries using 13 PNG files**. Runner currently targets
iPhone; populating the existing iPad slots does not change supported devices.

## 4. Verify before delivery

`--check` performs no writes. It checks every required slot, filename, PNG size,
RGB mode, lack of alpha/transparency, sRGB profile, consistency with the master,
unassigned PNGs and the icon build-setting declarations. A failure must be fixed
before calling icon preparation complete. It intentionally fails on the unchanged
template placeholders, which have no prepared study master.

Inspect the 1024px master and small exports, especially 29px, 40px, 60px and 180px.
Then open `ios/Runner.xcworkspace` in Xcode and check Runner → General → App Icons
and Launch Screen selects `AppIcon`. All relevant asset wells should be filled.

Run `flutter build ios --simulator` and inspect the installed Home Screen icon.
Check appearance on the intended iOS versions; the OS can apply different visual
treatments. For App Store delivery, use the configured signing setup to create a
device release archive, validate it in Xcode Organizer and upload the build when
release is authorized. Report icon validation separately from signing, upload
and App Store review; this helper cannot guarantee submission acceptance.

After changing the helper, run:

```sh
build/icon-tools/bin/python -m py_compile scripts/prepare-app-icons.py scripts/test_app_icons.py
build/icon-tools/bin/python -m unittest discover -s scripts -p test_app_icons.py
```

## Other platform assets

This helper installs **iOS assets only**. For an adaptation that also ships Android
or web, export the same approved artwork to their own destinations and verify
their manifests. Do not leave another supported platform with the template icon.

| Destination | Required exports in the current template |
| --- | --- |
| `android/app/src/main/res/mipmap-*/ic_launcher.png` | mdpi 48px, hdpi 72px, xhdpi 96px, xxhdpi 144px, xxxhdpi 192px |
| `web/icons/` | `Icon-192.png`, `Icon-512.png`, `Icon-maskable-192.png`, `Icon-maskable-512.png`; preserve the maskable safe area |
| `web/favicon.png` | 32px |

Adaptive/round Android icons, watchOS, macOS and other targets require their own
asset handling if enabled. Review current platform guidance when adding support.

Apple references checked on 2026-10-26. Recheck them when changing Xcode/icon
formats or adding platforms.
