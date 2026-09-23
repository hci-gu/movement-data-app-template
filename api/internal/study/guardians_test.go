package study

import (
	"app/internal/bankid"
	_ "app/migrations"
	"bytes"
	"context"
	"encoding/base64"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
)

func TestAdultBirthday(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	if adult("200809240001", now) || !adult("200809230001", now) || adult("200813230001", now) || adult("200809220001", now.AddDate(-1, 0, 0)) {
		t.Fatal("age boundary was not enforced")
	}
}

func TestGuardianSignaturesGateParticipant(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Environment: "test", AppID: "example.app", PublicURL: "https://study.example", ReturnURL: "researchsteps://bankid/return", ActiveKey: "one", EncryptionKeys: map[string][]byte{"one": bytes.Repeat([]byte{1}, 32)}}
	doc, err := PublishConsent(app, cfg, "v1", "Study consent", "I consent.")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeProvider{result: guardianResult("201001012384")}
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	s := &Service{App: app, Config: cfg, provider: f, attempts: map[string]*Attempt{}, rates: map[string]rateEntry{}, now: func() time.Time { return now }}
	participant, err := s.Start(context.Background(), StartInput{ClientSecret: randomSecret(), Mode: "qr", Version: doc.Id, DocumentHash: doc.GetString("documentHash")}, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.advance(context.Background(), participant); err != nil {
		t.Fatal(err)
	}
	user, err := app.FindRecordById("users", participant.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if s.guardianEligible(app, user) {
		t.Fatal("minor was eligible without guardians")
	}
	if _, err := s.createGuardianRequest(user, "201201012384", 2); err == nil {
		t.Fatal("underage guardian accepted")
	}
	status, err := s.createGuardianRequest(user, "198001012384", 2)
	if err != nil {
		t.Fatal(err)
	}
	if status["eligible"] == true {
		t.Fatal("one unsigned link allowed upload")
	}
	if _, err := s.createGuardianRequest(user, "198001012384", 2); err == nil {
		t.Fatal("duplicate guardian accepted")
	}
	requests, err := s.guardianRequests(app, user.Id)
	if err != nil {
		t.Fatal(err)
	}
	firstLink := status["requests"].([]map[string]any)[0]["link"].(string)
	if !signingRequestIDPattern.MatchString(requests[0].Id) || firstLink != cfg.PublicURL+"/guardian/"+requests[0].Id {
		t.Fatal("guardian link must use the numeric signing request ID")
	}
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	s.RegisterRoutes(router)
	handler, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	openLink := httptest.NewRequest("GET", firstLink, nil)
	openLink.RemoteAddr = "192.0.2.2:1234"
	handoff := httptest.NewRecorder()
	handler.ServeHTTP(handoff, openLink)
	if handoff.Code != 200 || !strings.Contains(handoff.Body.String(), `href="https://app.bankid.com/?autostarttoken=auto"`) || !strings.Contains(handoff.Body.String(), "Open BankID") || strings.Contains(handoff.Body.String(), "#ZgotmplZ") || handoff.Header().Get("Location") != "" {
		t.Fatalf("guardian handoff: %d %s", handoff.Code, handoff.Body.String())
	}
	if !strings.Contains(handoff.Body.String(), `href="bankid:///?autostarttoken=auto"`) || !strings.Contains(handoff.Body.String(), "/guardian/"+requests[0].Id+"/qr?nonce=") {
		t.Fatal("guardian handoff needs a desktop launch and a phone QR option")
	}
	first, err := s.startGuardian(context.Background(), requests[0], "192.0.2.2", "test-browser")
	if err != nil {
		t.Fatal(err)
	}
	qrRequest := httptest.NewRequest("GET", cfg.PublicURL+"/guardian/"+requests[0].Id+"/qr?nonce="+url.QueryEscape(first.Nonce), nil)
	qrRequest.RemoteAddr = "192.0.2.2:1234"
	qrResponse := httptest.NewRecorder()
	handler.ServeHTTP(qrResponse, qrRequest)
	if qrResponse.Code != 200 || qrResponse.Header().Get("Content-Type") != "image/png" || !bytes.HasPrefix(qrResponse.Body.Bytes(), []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("guardian QR response: %d %s", qrResponse.Code, qrResponse.Header().Get("Content-Type"))
	}
	now = now.Add(time.Second)
	nextQR := httptest.NewRecorder()
	handler.ServeHTTP(nextQR, qrRequest)
	if nextQR.Code != 200 || bytes.Equal(nextQR.Body.Bytes(), qrResponse.Body.Bytes()) {
		t.Fatal("guardian QR did not change after one second")
	}
	stateRequest := httptest.NewRequest("GET", cfg.PublicURL+"/guardian/"+requests[0].Id+"/state?nonce="+url.QueryEscape(first.Nonce), nil)
	stateRequest.RemoteAddr = "192.0.2.2:1234"
	stateResponse := httptest.NewRecorder()
	handler.ServeHTTP(stateResponse, stateRequest)
	if stateResponse.Code != 200 || !strings.Contains(stateResponse.Body.String(), `"qrAvailable":true`) {
		t.Fatalf("guardian pending state: %d %s", stateResponse.Code, stateResponse.Body.String())
	}
	qrRequest.RemoteAddr = "192.0.2.3:1234"
	otherDeviceQR := httptest.NewRecorder()
	handler.ServeHTTP(otherDeviceQR, qrRequest)
	if otherDeviceQR.Code != 404 {
		t.Fatalf("guardian QR leaked to another device: %d", otherDeviceQR.Code)
	}
	if f.request.Web == nil || f.request.Web.ReferringDomain != "study.example" || f.request.App != nil || !strings.Contains(f.request.ReturnURL, "nonce=") || !s.guardianReturnMatches(requests[0].Id, first.Nonce, "192.0.2.2") || s.guardianReturnMatches(requests[0].Id, first.Nonce, "192.0.2.3") {
		t.Fatal("guardian web handoff was not bound to the starting device")
	}
	returnURL, err := url.Parse(f.request.ReturnURL)
	if err != nil {
		t.Fatal(err)
	}
	returnRequest := httptest.NewRequest("GET", returnURL.String(), nil)
	returnRequest.RemoteAddr = "192.0.2.3:1234"
	wrongDevice := httptest.NewRecorder()
	handler.ServeHTTP(wrongDevice, returnRequest)
	if wrongDevice.Code != 400 {
		t.Fatalf("return from wrong device: %d", wrongDevice.Code)
	}
	returnRequest.RemoteAddr = "192.0.2.2:1234"
	matchingDevice := httptest.NewRecorder()
	handler.ServeHTTP(matchingDevice, returnRequest)
	if matchingDevice.Code != 200 || !strings.Contains(matchingDevice.Body.String(), "Checking your signature") {
		t.Fatalf("return from signing device: %d %s", matchingDevice.Code, matchingDevice.Body.String())
	}
	f.result = guardianResult("198001012384")
	if err := s.advance(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if first.Status != "accepted" || s.guardianEligible(app, user) {
		t.Fatal("first of two signatures unlocked upload")
	}
	approved := httptest.NewRecorder()
	handler.ServeHTTP(approved, openLink)
	if approved.Code != 200 || !strings.Contains(approved.Body.String(), "Signature received") || strings.Contains(approved.Body.String(), "Open BankID on this phone") {
		t.Fatalf("guardian approval page: %d %s", approved.Code, approved.Body.String())
	}
	stateAfterSigning := httptest.NewRecorder()
	handler.ServeHTTP(stateAfterSigning, stateRequest)
	if stateAfterSigning.Code != 200 || !strings.Contains(stateAfterSigning.Body.String(), `"status":"accepted"`) {
		t.Fatalf("guardian signed state: %d %s", stateAfterSigning.Code, stateAfterSigning.Body.String())
	}
	status, err = s.createGuardianRequest(user, "197501012384", 2)
	if err != nil {
		t.Fatal(err)
	}
	if status["requests"].([]map[string]any)[0]["signed"] != true {
		t.Fatal("first signature not visible")
	}
	requests, err = s.guardianRequests(app, user.Id)
	if err != nil {
		t.Fatal(err)
	}
	secondLink := status["requests"].([]map[string]any)[1]["link"].(string)
	if secondLink != cfg.PublicURL+"/guardian/"+requests[1].Id {
		t.Fatal("second guardian link does not use its request ID")
	}
	s.Config.PublicURL = ""
	missingURLRequest := httptest.NewRequest("GET", secondLink, nil)
	missingURLRequest.RemoteAddr = "192.0.2.3:1234"
	missingURLResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingURLResponse, missingURLRequest)
	if missingURLResponse.Code != 503 || !strings.Contains(missingURLResponse.Body.String(), "BANKID_PUBLIC_URL") {
		t.Fatalf("missing public URL should be explained: %d %s", missingURLResponse.Code, missingURLResponse.Body.String())
	}
	s.Config.PublicURL = cfg.PublicURL
	f.startErr = &bankid.APIError{Status: 400, Code: "invalidParameters"}
	providerRequest := httptest.NewRequest("GET", secondLink, nil)
	providerRequest.RemoteAddr = "192.0.2.3:1234"
	providerResponse := httptest.NewRecorder()
	handler.ServeHTTP(providerResponse, providerRequest)
	if providerResponse.Code != 502 || !strings.Contains(providerResponse.Body.String(), "invalidParameters") {
		t.Fatalf("BankID rejection should be distinct: %d %s", providerResponse.Code, providerResponse.Body.String())
	}
	f.startErr = nil
	second, err := s.startGuardian(context.Background(), requests[1], "192.0.2.3", "test-browser")
	if err != nil {
		t.Fatal(err)
	}
	f.result = guardianResult("198001012384")
	if err := s.advance(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if second.Status == "accepted" || s.guardianEligible(app, user) {
		t.Fatal("wrong guardian identity was accepted")
	}
	second, err = s.startGuardian(context.Background(), requests[1], "192.0.2.3", "test-browser")
	if err != nil {
		t.Fatal(err)
	}
	f.result = guardianResult("197501012384")
	if err := s.advance(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	user, err = app.FindRecordById("users", user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "accepted" || !s.guardianEligible(app, user) {
		t.Fatalf("two valid guardians did not unlock upload: status=%s hint=%s count=%d", second.Status, second.Hint, user.GetInt("guardianCount"))
	}
	newDoc, err := PublishConsent(app, cfg, "v2", "Updated consent", "I consent to the update.")
	if err != nil {
		t.Fatal(err)
	}
	f.result = guardianResult("201001012384")
	renewal, err := s.Start(context.Background(), StartInput{ClientSecret: randomSecret(), Mode: "qr", Version: newDoc.Id, DocumentHash: newDoc.GetString("documentHash")}, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.advance(context.Background(), renewal); err != nil {
		t.Fatal(err)
	}
	if s.guardianEligible(app, user) {
		t.Fatal("old guardian signatures allowed the new consent version")
	}
	requests, err = s.guardianRequests(app, user.Id)
	if err != nil {
		t.Fatal(err)
	}
	first, err = s.startGuardian(context.Background(), requests[0], "192.0.2.2", "test-browser")
	if err != nil {
		t.Fatal(err)
	}
	f.result = guardianResult("198001012384")
	if err := s.advance(context.Background(), first); err != nil || first.Status != "accepted" {
		t.Fatalf("renew guardian consent: %v %s", err, first.Status)
	}
	f.result = guardianResult("201201012384")
	other, err := s.Start(context.Background(), StartInput{ClientSecret: randomSecret(), Mode: "qr", Version: newDoc.Id, DocumentHash: newDoc.GetString("documentHash")}, "192.0.2.4")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.advance(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	minor, err := app.FindRecordById("users", other.UserID)
	if err != nil {
		t.Fatal(err)
	}
	oneStatus, err := s.createGuardianRequest(minor, "198501012384", 1)
	if err != nil {
		t.Fatal(err)
	}
	oneRequest, err := s.guardianRequests(app, minor.Id)
	if err != nil || len(oneRequest) != 1 {
		t.Fatalf("single guardian request: %v", err)
	}
	oneLink := oneStatus["requests"].([]map[string]any)[0]["link"].(string)
	if oneLink != cfg.PublicURL+"/guardian/"+oneRequest[0].Id {
		t.Fatal("single guardian link does not use its request ID")
	}
	one, err := s.startGuardian(context.Background(), oneRequest[0], "192.0.2.5", "test-browser")
	if err != nil {
		t.Fatal(err)
	}
	f.result = guardianResult("198501012384")
	if err := s.advance(context.Background(), one); err != nil {
		t.Fatal(err)
	}
	minor, err = app.FindRecordById("users", minor.Id)
	if err != nil {
		t.Fatal(err)
	}
	if one.Status != "accepted" || !s.guardianEligible(app, minor) {
		t.Fatal("one selected guardian did not unlock upload")
	}
}

func guardianResult(personalNumber string) bankid.Result {
	return bankid.Result{Status: "complete", CompletionData: &bankid.Completion{User: bankid.User{PersonalNumber: personalNumber}, Signature: base64.StdEncoding.EncodeToString([]byte("signature")), OCSPResponse: base64.StdEncoding.EncodeToString([]byte("ocsp")), Risk: "low"}}
}
