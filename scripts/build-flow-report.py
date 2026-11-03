#!/usr/bin/env python3
"""Create a Swedish overview of the scoliosis study's successful participant flow.

Reuses the reviewed captures; does not rebuild the app or replace app_report.html.
"""
from __future__ import annotations

import base64
from datetime import datetime
import html
import json
from pathlib import Path
from zoneinfo import ZoneInfo

ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / "build" / "app-report"

# A deliberate editorial selection, rather than a filter that could accidentally
# include new failure states. Titles and descriptions are all Swedish.
SECTIONS = [
    (
        "1", "Läs om studien",
        "Deltagaren börjar med att läsa om studiens syfte och hur deltagandet går till. Studieinformation och kontaktuppgifter finns tillgängliga från startsidan.",
        [("introduction", "Introduktion")],
    ),
    (
        "2", "Lämna samtycke med BankID",
        "Deltagaren läser studieinformationen och samtycket, bekräftar sitt val och signerar med BankID. Efter signeringen öppnas checklistan.",
        [("consent", "Läs och bekräfta samtycket"), ("bankid-pending", "Signera med BankID")],
    ),
    (
        "3", "Svara på frågorna",
        "Checklistan leder vidare till frågor om behandling och fysisk aktivitet. Behandlingstyp och behandlingsdatum krävs för att bestämma insamlingsperioden. Övriga frågor är frivilliga. Här visas ett urval med operation som exempel; frågorna anpassas efter deltagarens behandling och uppgifter.",
        [
            ("home", "Din checklista"),
            ("question-scoliosis000001-scoliquestion02", "Välj behandling"),
            ("picker-day", "Ange behandlingsdatum"),
            ("question-scoliosis000001-scoliquestion05", "Svara om fysisk aktivitet"),
        ],
    ),
    (
        "4", "Hämta och granska rörelsedata",
        "När frågorna är inskickade kan deltagaren hämta tillgängliga mätningar från Apple Hälsa. Insamlingen omfattar sex rörelsemått, från ett år före till fem år efter behandlingsstart, som längst fram till i dag. Deltagaren granskar uppgifterna innan de skickas.",
        [("steps-idle", "Hämta från Apple Hälsa"), ("step-details", "Granska dina mätningar"), ("steps-ready", "Välj att skicka rörelsedata")],
    ),
    (
        "5", "Avsluta deltagandet i appen",
        "När både svaren och rörelsedatan har tagits emot är checklistan klar. Deltagaren kan avsluta och sedan återvända för att läsa sina svar och sitt samtycke.",
        [("steps-complete", "Allt i checklistan är klart"), ("done", "Bekräftelse på att allt är inskickat")],
    ),
]
GUARDIANS = [("guardians-new", "Skapa länkar till vårdnadshavare"), ("guardians-approved", "Samtliga vårdnadshavare har signerat")]


def main() -> None:
    manifest = json.loads((WORK / "manifest.json").read_text(encoding="utf-8"))
    if manifest.get("sourceKind") != "study":
        raise ValueError("Generate the full report with --study before creating the flow report.")
    cases = {case["id"]: case for case in manifest["cases"]}
    escape = lambda text: html.escape(str(text), quote=True)

    def card(case_id: str, title: str) -> str:
        figures = []
        for index, filename in enumerate(cases[case_id]["images"], 1):
            path = (WORK / "captures" / filename).resolve()
            if not path.is_relative_to((WORK / "captures").resolve()):
                raise ValueError("Screenshot path escapes the capture directory")
            data = path.read_bytes()
            if not data.startswith(b"\x89PNG\r\n\x1a\n"):
                raise ValueError(f"Invalid screenshot: {filename}")
            label = escape(f"{title} · bild {index} av {len(cases[case_id]['images'])}")
            encoded = base64.b64encode(data).decode("ascii")
            figures.append(f'<button class="shot" type="button" aria-label="Förstora {label}"><img loading="lazy" decoding="async" src="data:image/png;base64,{encoded}" alt="{label}"></button>')
        if not figures:
            raise ValueError(f"No screenshot for {case_id}")
        continuation = (
            '<details class="continuation"><summary>Visa resten av skärmen</summary>'
            + ''.join(figures[1:]) + '</details>'
        ) if len(figures) > 1 else ''
        return f'<article class="screen" data-search="{escape(title)}"><div class="screen-heading"><h3>{escape(title)}</h3></div>{figures[0]}{continuation}</article>'

    sections, navigation = [], []
    for number, title, description, selection in SECTIONS:
        anchor = f"steg-{number}"
        navigation.append(f'<a href="#{anchor}">{number}. {escape(title)}</a>')
        sections.append(f'<section class="group" id="{anchor}"><h2>{number}. {escape(title)}</h2><p class="intro">{escape(description)}</p><div class="screens">{"".join(card(*item) for item in selection)}</div></section>')
        if number == "2":
            sections.append(
                '<details class="guardian-branch" id="vardnadshavare"><summary>För deltagare under 15 år: vårdnadshavares samtycke</summary>'
                '<p class="intro">Innan deltagaren kan fortsätta behöver samtliga vårdnadshavare samtycka. En länk skapas för varje vårdnadshavare, som signerar med BankID. Deltagaren kan tacka nej även om vårdnadshavarna samtycker.</p>'
                f'<div class="screens">{"".join(card(*item) for item in GUARDIANS)}</div></details>'
            )

    # Share the full report's visual language and image viewer. Its English
    # headings, build metadata and error-case inventory are not copied.
    template = (ROOT / "scripts" / "app-report-template.html").read_text(encoding="utf-8")
    css = template.split('<style>', 1)[1].split('</style>', 1)[0]
    viewer_js = template.split("document.querySelectorAll('.shot')", 1)[1].split('</script>', 1)[0]
    viewer_js = "const viewer=document.getElementById('viewer');\ndocument.querySelectorAll('.shot')" + viewer_js
    captured_at = datetime.strptime(manifest["generatedAt"], "%Y-%m-%d %H:%M UTC").replace(tzinfo=ZoneInfo("UTC")).astimezone(ZoneInfo("Europe/Stockholm"))
    app_name = escape(manifest['appName'])
    output = f'''<!doctype html>
<html lang="sv"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{app_name} · Deltagarens väg genom appen</title><link rel="icon" href="data:,">
<style>{css}
main{{max-width:1460px}}.screens{{grid-template-columns:repeat(2,minmax(0,1fr));gap:32px;max-width:900px}}.screen{{max-width:390px}}.group{{margin-top:64px}}.group>.intro{{margin-bottom:26px}}.group h2{{line-height:1.3}}.screen-heading{{min-height:44px}}.guardian-branch{{margin:44px 0;padding:24px;border:1px solid var(--line);border-radius:12px;background:#fffdf8}}.guardian-branch>summary{{font-weight:650;font-size:18px;cursor:pointer;line-height:1.5}}.guardian-branch .intro{{font-size:15px}}.flow{{display:flex;flex-wrap:wrap;gap:10px;list-style:none;padding:24px 0;margin:30px 0;border-block:1px solid var(--line)}}.flow li{{padding:8px 13px;background:#e4f4ee;border-radius:8px;font-size:13px}}.hero .intro{{max-width:720px}}.source-note{{font-size:13px;line-height:1.7;color:var(--muted)}}
@media(max-width:600px){{.screens{{grid-template-columns:1fr}}.screen{{margin-inline:auto;width:100%}}.group h2{{font-size:23px}}.guardian-branch{{padding:18px}}}}
</style></head><body>
<aside class="sidebar"><div class="brand">En översikt av appen</div><h2>{app_name}</h2><nav aria-label="Rapportens avsnitt"><a href="#oversikt">Översikt</a>{''.join(navigation)}</nav><p class="offline">Rapporten fungerar utan internet.<br>Alla bilder finns i filen.<br>Klicka på en bild för att förstora.</p></aside>
<main><header class="hero" id="oversikt"><div class="eyebrow">Deltagarens väg genom appen</div><h1>{app_name}</h1><p class="intro">Så går deltagandet till, från den första informationen till inskickade svar och rörelsedata.</p>
<ol class="flow" aria-label="Deltagandets fem steg"><li>1. Studieinformation</li><li>2. Samtycke</li><li>3. Frågor</li><li>4. Rörelsedata</li><li>5. Klart</li></ol>
<p class="source-note">Skärmbilderna visar appen med exempeluppgifter. Formuläret visas i urval. Bilder från {captured_at:%Y-%m-%d}, kl. {captured_at:%H.%M}. Klicka på en bild för att se den i större format.</p></header>
{''.join(sections)}
<p class="endnote">Översikten visar ett lyckat deltagande. BankID-appen och iPhones egna behörighetsdialoger ingår inte i bilderna.</p></main>
<dialog id="viewer" aria-labelledby="viewer-title"><div class="toolbar"><p id="viewer-title"></p><button type="button" id="close-viewer">Stäng</button></div><img id="viewer-image" alt=""></dialog>
<script>{viewer_js}</script></body></html>'''
    destination = ROOT / "app_flow_sv.html"
    destination.write_text(output, encoding="utf-8")
    selected = [item for section in SECTIONS for item in section[3]] + GUARDIANS
    print(f"Created {destination}: {len(selected)} screens, {sum(len(cases[item[0]]['images']) for item in selected)} screenshots.")


if __name__ == "__main__":
    main()
