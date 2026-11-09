package study

import (
	"app/internal/kontoapi"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAdultBirthday(t *testing.T) {
	if adult("200811032384", time.Date(2026, 11, 2, 23, 59, 59, 0, time.UTC)) {
		t.Fatal("adult too early")
	}
	if !adult("200811032384", time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("birthday not adult")
	}
}
func TestGuardianAuthenticationRequiresExplicitBoundApproval(t *testing.T) {
	s, f, h := testService(t)
	_, token, user := authenticate(t, s, f, h, time.Now().AddDate(-14, 0, 0).Format("20060102")+"2384")
	requireCode(t, h, "POST", "/api/consent/accept", token, acceptInput(t, s), 200)
	if _, err := s.createGuardianRequest(user, "198001012384", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.createGuardianRequest(user, "198101012384", 2); err != nil {
		t.Fatal(err)
	}
	requests, err := s.guardianRequests(s.App, user.Id)
	if err != nil {
		t.Fatal(err)
	}
	consent, _ := s.activeAcceptance(s.App, user)
	for i, request := range requests {
		a, credential, err := s.startGuardian(context.Background(), request, "192.0.2.1")
		if err != nil {
			t.Fatal(err)
		}
		f.result = kontoapi.Result{Status: "complete", PersonalNumber: request.GetString("personalNumber")}
		if err = s.advance(context.Background(), a); err != nil {
			t.Fatal(err)
		}
		if a.Status != "accepted" {
			t.Fatal(a.Status, a.Hint)
		}
		if s.guardianEligible(s.App, user) {
			t.Fatal("guardian login unlocked participation")
		}
		base := "/guardian/" + request.Id
		cookie := &http.Cookie{Name: guardianCookie(request.Id), Value: credential}
		get := httptest.NewRequest("GET", base+"/return", nil)
		get.AddCookie(cookie)
		view := httptest.NewRecorder()
		h.ServeHTTP(view, get)
		if view.Code != 200 || !strings.Contains(view.Body.String(), "Bekräfta samtycke") {
			t.Fatalf("consent page: %d %s", view.Code, view.Body)
		}
		post := func(csrf string) *httptest.ResponseRecorder {
			in := acceptInput(t, s)
			form := url.Values{"csrf": {csrf}, "consentTextId": {in.Version}, "documentHash": {in.DocumentHash}, "participantConsentId": {consent.Id}, "accept": {"yes"}}
			r := httptest.NewRequest("POST", base+"/approve", strings.NewReader(form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.AddCookie(cookie)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			return w
		}
		if w := post("wrong"); w.Code != 403 {
			t.Fatal("missing CSRF protection", w.Code)
		}
		for range 2 {
			if w := post(a.Nonce); w.Code != 200 || !strings.Contains(w.Body.String(), "Samtycket har registrerats") {
				t.Fatalf("approval: %d %s", w.Code, w.Body)
			}
		}
		if i == 0 && s.guardianEligible(s.App, user) {
			t.Fatal("one of two approvals unlocked participation")
		}
	}
	requireCode(t, h, "GET", "/protected-test", token, nil, 204)
	approvals, _ := s.App.FindAllRecords("guardian_approvals")
	if len(approvals) != 2 {
		t.Fatal("duplicate approval")
	}
	requireCode(t, h, "POST", "/api/consent/withdraw", token, nil, 204)
	_, newToken, newUser := authenticate(t, s, f, h, user.GetString("personalNumber"))
	requireCode(t, h, "POST", "/api/consent/accept", newToken, acceptInput(t, s), 200)
	if s.guardianEligible(s.App, newUser) {
		t.Fatal("old approvals revived after withdrawal")
	}
	newConsent, err := s.activeAcceptance(s.App, newUser)
	if err != nil {
		t.Fatal(err)
	}
	for _, approval := range approvals {
		approval.Set("consent", newConsent.Id)
		if err := s.App.Save(approval); err != nil {
			t.Fatal(err)
		}
	}
	if s.guardianEligible(s.App, newUser) {
		t.Fatal("tampered approval binding revived old consent")
	}
}
func TestWrongGuardianAndStaleDocumentCannotApprove(t *testing.T) {
	s, f, h := testService(t)
	_, token, user := authenticate(t, s, f, h, time.Now().AddDate(-14, 0, 0).Format("20060102")+"2384")
	requireCode(t, h, "POST", "/api/consent/accept", token, acceptInput(t, s), 200)
	if _, err := s.createGuardianRequest(user, "198001012384", 1); err != nil {
		t.Fatal(err)
	}
	requests, _ := s.guardianRequests(s.App, user.Id)
	a, _, err := s.startGuardian(context.Background(), requests[0], "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	f.result = kontoapi.Result{Status: "complete", PersonalNumber: "198101012384"}
	if err = s.advance(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if a.Status != "rejected" {
		t.Fatal("wrong guardian authenticated")
	}
	a, _, err = s.startGuardian(context.Background(), requests[0], "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	f.result = kontoapi.Result{Status: "complete", PersonalNumber: "198001012384"}
	if err = s.advance(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	old := acceptInput(t, s)
	consent, _ := s.activeAcceptance(s.App, user)
	if _, err = PublishConsent(s.App, "v2", "Updated", "New consent."); err != nil {
		t.Fatal(err)
	}
	if err = s.approveGuardian(a, old, consent.Id); err == nil {
		t.Fatal("old document approved")
	}
}
