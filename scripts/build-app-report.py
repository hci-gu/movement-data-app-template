#!/usr/bin/env python3
"""Build an offline app showcase from real Flutter widgets and guardian HTML."""
from __future__ import annotations

import argparse
import base64
from datetime import datetime, timezone
import hashlib
import html
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import platform
import re
import shutil
import struct
import subprocess
import sys
import threading
import time

ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / "build" / "app-report"
PW_PACKAGE = "@playwright/cli@0.1.22"


def content_strings(value: object, path: str = "content"):
    """Inspect exported content, excluding legitimate input-format hints."""
    if isinstance(value, str):
        yield path, value
    elif isinstance(value, dict):
        for key, item in value.items():
            if key != "placeholder":
                yield from content_strings(item, f"{path}.{key}")
    elif isinstance(value, list):
        for i, item in enumerate(value):
            yield from content_strings(item, f"{path}[{i}]")


def validate_copy(value: object, path: str, *, check_contacts: bool = True) -> None:
    # Deliberately narrow: editorial quality, approved wording and actual service
    # availability still require the workflow's source and deployment review.
    markers = re.compile(
        r"\b(?:TODO|TBD|FIXME|lorem ipsum|demonstration only|demonstration consent|test environment only)\b"
        r"|\[(?:insert|replace|draft|utkast)\b[^\]]*\]"
        r"|^\s*(?:draft|utkast)(?:\s*[:—-].*)?\s*$",
        re.IGNORECASE | re.MULTILINE,
    )
    contacts = re.compile(r"@[\w.-]*\bexample\.(?:org|com|net|invalid)\b", re.IGNORECASE)
    for field, text in content_strings(value, path):
        if markers.search(text) or (check_contacts and contacts.search(text)):
            raise ValueError(f"Unfinished participant content in {field}. Replace draft markers/example contacts before study delivery.")


def validate_study_source(source: dict) -> None:
    if source.get("sourceKind") != "study":
        raise ValueError("Study definitions/consent are missing. Seed the study questionnaire and pass --consent; --study never substitutes template content.")
    consent = source.get("consent", {})
    if not all(isinstance(consent.get(field), str) and consent[field].strip() for field in ("title", "text")):
        raise ValueError("Study consent needs a final title and full text.")
    validate_copy(source, "study source")


def validate_study_manifest(manifest: dict) -> None:
    content = manifest.get("studyContent", {})
    for field in ("appName", "introduction", "supportEmail", "privacyContact"):
        if not isinstance(content.get(field), str) or not content[field].strip():
            raise ValueError(f"Study content is missing {field}; refresh the report captures after completing the app copy.")
    if content["appName"] == "Research Steps" or content["introduction"] == (
        "Contribute to research by answering the study questions and sharing your step history."
    ):
        raise ValueError("Replace the template study name and introduction before study delivery.")
    validate_copy(content, "StudyContent")
    for case in manifest["cases"]:
        if not isinstance(case.get("participantText"), list):
            raise ValueError(f"Missing participant copy capture for {case['id']}.")
        # Synthetic participant input may contain example contacts. Study contacts
        # are validated above; only draft markers are checked in rendered states.
        validate_copy(case["participantText"], case["id"], check_contacts=False)


def executable(name: str) -> str:
    found = shutil.which(name)
    if not found:
        raise RuntimeError(f"Install {name} and make it available on PATH.")
    return found


def run(command: list[str], name: str, checks: list[dict], cwd: Path = ROOT) -> str:
    print(f"[{name}] {' '.join(command[:4])}", flush=True)
    start = time.monotonic()
    result = subprocess.run(command, cwd=cwd, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    (WORK / f"{name}.log").write_text(result.stdout, encoding="utf-8")
    checks.append({"name": name, "status": "passed" if result.returncode == 0 else "failed", "seconds": round(time.monotonic() - start, 1)})
    if result.returncode:
        raise RuntimeError(f"{name} failed; see {WORK / (name + '.log')}\n{result.stdout[-5000:]}")
    return result.stdout


def font_path(explicit: str | None) -> Path:
    if explicit:
        path = Path(explicit).expanduser().resolve()
    else:
        path = Path(executable("flutter")).resolve().parent / "cache/artifacts/material_fonts/Roboto-Regular.ttf"
    if not path.is_file():
        raise RuntimeError("Flutter's Roboto font was not found. Pass --font /path/to/a/licensed-font.ttf.")
    return path


def validate_manifest(manifest: dict, directory: Path) -> None:
    required = manifest["requiredCases"]
    cases = manifest["cases"]
    ids = [case["id"] for case in cases]
    if not required or len(set(required)) != len(required) or len(set(ids)) != len(ids) or set(ids) != set(required):
        raise ValueError("Preview coverage is incomplete or contains duplicate IDs.")
    for case in cases:
        if not case["images"]:
            raise ValueError(f"No screenshot for {case['id']}")
        for name in case["images"]:
            path = (directory / name).resolve()
            if not path.is_relative_to(directory.resolve()):
                raise ValueError("Screenshot path escapes the capture directory")
            data = path.read_bytes()
            if len(data) < 32 or data[:8] != b"\x89PNG\r\n\x1a\n":
                raise ValueError(f"Invalid PNG: {name}")
            width, height = struct.unpack(">II", data[16:24])
            if width < 100 or height < 100:
                raise ValueError(f"Screenshot is unexpectedly small: {name}")


def guardian_captures(checks: list[dict]) -> list[dict]:
    """Capture production templates served locally, with inert demo endpoints."""
    class Handler(SimpleHTTPRequestHandler):
        def __init__(self, *args, **kwargs):
            super().__init__(*args, directory=str(WORK / "source" / "guardian"), **kwargs)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    session = f"app-report-{server.server_port}"
    command = [executable("npx"), "--yes", "--package", PW_PACKAGE, "playwright-cli", "--session", session]
    out = WORK / "captures"
    cases = []
    try:
        run(command + ["open", f"http://127.0.0.1:{server.server_port}/signing.html"], "browser-open", checks, WORK)
        run(command + ["resize", "390", "844"], "browser-size", checks, WORK)
        for mode, title in [("signing", "Guardian BankID handoff"), ("expired", "Guardian QR expired"), ("failed", "Guardian signing failed"), ("checking", "Checking guardian signature"), ("approved", "Guardian signature received")]:
            run(command + ["goto", f"http://127.0.0.1:{server.server_port}/{mode}.html"], f"browser-{mode}-open", checks, WORK)
            # Wait for the page's own JavaScript to consume its local response.
            if mode in ("signing", "expired", "failed"):
                expected = {"signing": "Waiting for BankID…", "expired": "Restart to get a new QR code.", "failed": "Signing did not complete. Restart to try again."}[mode]
                code = "await page.waitForFunction(text => document.querySelector('#status').textContent === text, " + json.dumps(expected) + "); await page.locator('main').waitFor();"
                if mode == "signing":
                    code += " await page.waitForFunction(() => document.querySelector('#qr').complete && document.querySelector('#qr').naturalWidth > 0);"
                run(command + ["run-code", "async (page) => { " + code + " }"], f"browser-{mode}-ready", checks, WORK)
            name = f"guardian-browser-{mode}.png"
            run(command + ["screenshot", "--full-page", "--filename", str(out / name)], f"browser-{mode}-capture", checks, WORK)
            cases.append({"id": f"guardian-browser-{mode}", "title": title, "group": "Guardian browser pages", "path": f"/guardian/… ({mode})", "images": [name]})
    finally:
        subprocess.run(command + ["close"], cwd=WORK, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        server.shutdown()
        server.server_close()
    return cases


def render_report(manifest: dict, metadata: dict, captures: Path) -> str:
    esc = lambda value: html.escape(str(value), quote=True)
    groups: dict[str, list] = {}
    for case in manifest["cases"]:
        groups.setdefault(case["group"], []).append(case)
    sections, navigation = [], []
    for group_number, (group, cases) in enumerate(groups.items()):
        anchor = f"group-{group_number}"
        navigation.append(f'<a href="#{anchor}">{esc(group)} <span>{len(cases)}</span></a>')
        cards = []
        for case in cases:
            figures = []
            for i, filename in enumerate(case["images"]):
                image = base64.b64encode((captures / filename).read_bytes()).decode("ascii")
                label = f"{case['title']} · section {i + 1} of {len(case['images'])}"
                figure = f'<button class="shot" type="button" aria-label="Enlarge {esc(label)}"><img loading="lazy" decoding="async" src="data:image/png;base64,{image}" alt="{esc(label)}"></button>'
                figures.append(figure)
            more = f'<details class="continuation"><summary>Continue below · {len(figures)-1} more captures</summary>{"".join(figures[1:])}</details>' if len(figures) > 1 else ""
            cards.append(f'<article class="screen" data-search="{esc(group + " " + case["title"] + " " + case["id"])}"><div class="screen-heading"><h3>{esc(case["title"])}</h3></div>{figures[0]}{more}</article>')
        sections.append(f'<section class="group" id="{anchor}"><h2>{esc(group)} <span>{len(cases)} views</span></h2><div class="screens">{"".join(cards)}</div></section>')
    check_rows = []
    for check in metadata["checks"]:
        duration = f" · {check['seconds']}s" if "seconds" in check else ""
        check_rows.append(f'<li><strong>{esc(check["name"])}</strong><span class="status {esc(check["status"])}">{esc(check["status"])}{esc(duration)}</span></li>')
    checks = "".join(check_rows)
    source = "Template demonstration: synthetic example questionnaires and consent. No study questionnaire was seeded." if manifest["sourceKind"] == "template" else "Study questionnaire definitions exported from migrations in a disposable database. Participant answers, identities and movement measurements are synthetic."
    replacements = {
        "APP_NAME": esc(manifest["appName"]), "DATE": esc(metadata["generatedAt"]),
        "NAV": "".join(navigation), "SECTIONS": "".join(sections),
        "COUNT": str(len(manifest["cases"])), "IMAGES": str(sum(len(c["images"]) for c in manifest["cases"])),
        "QUESTIONS": str(manifest["questionCount"]), "SOURCE": esc(source),
        "REVISION": esc(metadata["revision"]), "CHECKS": checks,
        "FONT": esc(metadata["font"]), "DIGEST": esc(metadata["definitionDigest"][:16]),
    }
    result = (ROOT / "scripts" / "app-report-template.html").read_text(encoding="utf-8")
    for key, value in replacements.items():
        result = result.replace(f"@@{key}@@", value)
    return result


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--consent", help="Study consent UTF-8 file; required when not supplied by study migrations")
    parser.add_argument("--consent-title", default="Study consent")
    parser.add_argument("--font", help="Override Flutter SDK's Roboto font for captures")
    parser.add_argument("--prepare-only", action="store_true", help="Refresh demo content without tests, captures or report")
    parser.add_argument("--skip-builds", action="store_true", help="Generate a development report; mark native builds as not run")
    parser.add_argument("--study", action="store_true", help="Require final study content; reject template copy, example contacts and draft markers")
    args = parser.parse_args()
    WORK.mkdir(parents=True, exist_ok=True)
    checks: list[dict] = []
    export = [executable("go"), "run", "./cmd/demo-export", "--out", str(WORK / "source"), "--template-fixture", "internal/questionnaires/testdata/records.json"]
    if args.consent:
        export += ["--consent", str(Path(args.consent).expanduser().resolve()), "--title", args.consent_title]
    run(export, "export-study", checks, ROOT / "api")
    source = (WORK / "source" / "study.json").read_bytes()
    if args.study:
        validate_study_source(json.loads(source))
    generated = "// Generated by scripts/build-app-report.py. Demo entry point only.\nconst generatedStudyBase64 =\n    '" + base64.b64encode(source).decode("ascii") + "';\n"
    (ROOT / "lib" / "demo" / "study_data.g.dart").write_text(generated, encoding="utf-8")
    if args.prepare_only:
        print("Demo content refreshed. Run: flutter run -t lib/main_demo.dart")
        return
    font = font_path(args.font)
    capture_dir = WORK / "captures"
    if capture_dir.exists():
        shutil.rmtree(capture_dir)
    capture_dir.mkdir()
    flutter = executable("flutter")
    run([flutter, "pub", "get"], "dependencies", checks)
    run([flutter, "analyze"], "flutter-analysis", checks)
    run([sys.executable, "-m", "unittest", "discover", "-s", "scripts", "-p", "test_app_report.py"], "report-tests", checks)
    run([executable("go"), "test", "./..."], "backend-tests", checks, ROOT / "api")
    run([executable("go"), "build", "-o", "app", "."], "backend-build", checks, ROOT / "api")
    run([flutter, "test", "--reporter", "expanded", "--dart-define=REPORT_CAPTURE=true", f"--dart-define=REPORT_OUTPUT={capture_dir}", f"--dart-define=REPORT_FONT={font}"], "flutter-tests-and-captures", checks)
    manifest = json.loads((capture_dir / "manifest.json").read_text(encoding="utf-8"))
    validate_manifest(manifest, capture_dir)
    if args.study:
        validate_study_manifest(manifest)
        checks.append({"name": "study-content", "status": "passed"})
    browser = guardian_captures(checks)
    manifest["cases"].extend(browser)
    manifest["requiredCases"].extend(c["id"] for c in browser)
    validate_manifest(manifest, capture_dir)
    for name, command in [("android-release", [flutter, "build", "apk", "--release"]), ("ios-simulator", [flutter, "build", "ios", "--simulator"])]:
        if args.skip_builds or (name == "ios-simulator" and platform.system() != "Darwin"):
            checks.append({"name": name, "status": "not run"})
        else:
            run(command, name, checks)
    revision = subprocess.check_output([executable("git"), "rev-parse", "--short", "HEAD"], cwd=ROOT, text=True).strip()
    if subprocess.check_output([executable("git"), "status", "--porcelain"], cwd=ROOT, text=True).strip():
        revision += " + working changes"
    metadata = {"generatedAt": datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC"), "revision": revision,
                "font": font.name, "definitionDigest": hashlib.sha256(source).hexdigest(), "checks": checks}
    (WORK / "manifest.json").write_text(json.dumps({**manifest, **metadata}, indent=2), encoding="utf-8")
    report = render_report(manifest, metadata, capture_dir)
    destination = ROOT / "app_report.html"
    temporary = destination.with_suffix(".html.tmp")
    temporary.write_text(report, encoding="utf-8")
    temporary.replace(destination)
    print(f"Created {destination}: {len(manifest['cases'])} views, {sum(len(c['images']) for c in manifest['cases'])} screenshots.")


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, ValueError, OSError, subprocess.SubprocessError) as error:
        print(f"Report generation failed: {error}\napp_report.html was not updated. Any previous report is from an earlier run.", file=sys.stderr)
        sys.exit(1)
