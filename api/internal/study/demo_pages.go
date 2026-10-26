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
func WriteGuardianPreviews(dir string) error {
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
	for _, mode := range []string{"signing", "expired", "failed", "checking", "approved"} {
		page := guardianPageView{Mode: mode, StyleNonce: "demo", MobileLink: "#demo", DesktopLink: "#demo", QRURL: "qr.png?demo=1", StateURL: mode + ".json", RestartURL: "#demo", DoneURL: "approved.html"}
		if mode == "expired" || mode == "failed" {
			page.Mode = "signing"
		}
		var body bytes.Buffer
		if err := guardianPageTemplate.Execute(&body, page); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, mode+".html"), body.Bytes(), 0644); err != nil {
			return err
		}
		status := "pending"
		if mode == "failed" {
			status = "failed"
		}
		data, err := json.Marshal(map[string]any{"status": status, "qrAvailable": mode == "signing", "pickedUp": false})
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, mode+".json"), data, 0644); err != nil {
			return err
		}
	}
	return nil
}
