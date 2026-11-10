package study

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	qrcode "github.com/skip2/go-qrcode"
)

// WriteGuardianPreviews is used only by the offline export command. Pages share
// the production HTML template; all links and status responses stay local.
func WriteGuardianPreviews(dir string, document Document) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	qr, err := qrcode.Encode("DEMONSTRATION ONLY - not a BankID order", qrcode.Medium, 280)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "qr.png"), qr, 0644); err != nil {
		return err
	}
	for _, mode := range []string{"authenticating", "expired", "failed", "checking", "consent", "approved", "error", "simulation-consent", "simulation-authenticating"} {
		page := guardianPageView{Mode: mode, StyleNonce: "demo", MobileLink: "#demo", DesktopLink: "#demo", QRURL: "qr.png?demo=1", StateURL: mode + ".json", RestartURL: "#demo", DoneURL: "consent.html", Document: &document, ParticipantSuffix: "2384", ConsentID: "demo", CSRF: "demo", ApproveURL: "#demo", Message: "Samtycket registrerades inte. Öppna inbjudan och läs det aktuella samtycket igen."}
		if mode == "expired" || mode == "failed" {
			page.Mode = "authenticating"
		}
		if mode == "simulation-consent" {
			page.Mode = "consent"
		}
		if mode == "simulation-authenticating" {
			page.Mode = "authenticating"
		}
		var body bytes.Buffer
		if err := guardianPageTemplate.Execute(&body, page); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, mode+".html"), body.Bytes(), 0644); err != nil {
			return err
		}
		status := "pending"
		if mode == "failed" || mode == "expired" {
			status = mode
		}
		data, err := json.Marshal(map[string]any{"status": status, "qrAvailable": page.Mode == "authenticating" && status == "pending"})
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, mode+".json"), data, 0644); err != nil {
			return err
		}
	}
	return nil
}
