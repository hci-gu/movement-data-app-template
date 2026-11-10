package study

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
	"net/url"

	"github.com/pocketbase/pocketbase/core"
)

//go:embed guardian.html
var guardianPageFiles embed.FS

var guardianPageTemplate = template.Must(template.ParseFS(guardianPageFiles, "guardian.html"))

type guardianPageView struct {
	SimulationLabel                                         string
	Mode                                                    string
	StyleNonce                                              string
	MobileLink                                              string
	DesktopLink                                             template.URL
	QRURL                                                   string
	StateURL                                                string
	RestartURL                                              string
	DoneURL                                                 string
	Document                                                *Document
	ParticipantSuffix, ConsentID, CSRF, ApproveURL, Message string
}

func guardianLaunchPage(e *core.RequestEvent, a *Attempt) error {
	base := "/guardian/" + a.GuardianRequestID
	return renderGuardianPage(e, guardianPageView{
		Mode:        "authenticating",
		MobileLink:  "https://app.bankid.com/?autostarttoken=" + url.QueryEscape(a.Order.AutoStartToken),
		DesktopLink: template.URL("bankid:///?autostarttoken=" + url.QueryEscape(a.Order.AutoStartToken)),
		QRURL:       base + "/qr",
		StateURL:    base + "/state",
		RestartURL:  base + "/restart",
		DoneURL:     base + "/return",
		CSRF:        a.Nonce,
	})
}

func guardianApprovedPage(e *core.RequestEvent) error {
	return renderGuardianPage(e, guardianPageView{Mode: "approved"})
}

func guardianCheckingPage(e *core.RequestEvent) error {
	return renderGuardianPage(e, guardianPageView{Mode: "checking"})
}

func renderGuardianPage(e *core.RequestEvent, page guardianPageView) error {
	if e.Response.Header().Get("X-Study-Simulation") != "" {
		page.SimulationLabel = "TESTSIMULERING"
	}
	page.StyleNonce = randomSecret()
	var body bytes.Buffer
	if err := guardianPageTemplate.Execute(&body, page); err != nil {
		return err
	}
	e.Response.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'nonce-"+page.StyleNonce+"'; img-src 'self'; connect-src 'self'; form-action 'self'; script-src 'nonce-"+page.StyleNonce+"'; base-uri 'none'; frame-ancestors 'none'")
	return e.HTML(http.StatusOK, body.String())
}
