package study

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

var guardianRequestIDPattern = regexp.MustCompile(`^[0-9]{3}-[0-9]{3}$`)

func generateGuardianRequestID(app core.App) (string, error) {
	for range 100 {
		n, err := rand.Int(rand.Reader, big.NewInt(1000000))
		if err != nil {
			return "", err
		}
		id := fmt.Sprintf("%03d-%03d", n.Int64()/1000, n.Int64()%1000)
		if _, err := app.FindRecordById("guardian_requests", id); errors.Is(err, sql.ErrNoRows) {
			return id, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", problem(503, "busy", "No guardian request ID is available. Please try again.")
}

func adult(personalNumber string, now time.Time) bool {
	return atLeastAge(personalNumber, now, 18)
}

func atLeastAge(personalNumber string, now time.Time, years int) bool {
	if !personalNumberPattern.MatchString(personalNumber) {
		return false
	}
	year, _ := strconv.Atoi(personalNumber[:4])
	month, _ := strconv.Atoi(personalNumber[4:6])
	day, _ := strconv.Atoi(personalNumber[6:8])
	birth := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if birth.Year() != year || int(birth.Month()) != month || birth.Day() != day || birth.After(now.UTC()) {
		return false
	}
	return !now.UTC().Before(birth.AddDate(years, 0, 0))
}

func (s *Service) guardianRequests(app core.App, userID string) ([]*core.Record, error) {
	return app.FindRecordsByFilter("guardian_requests", "user={:user}", "slot", 3, 0, dbx.Params{"user": userID})
}

func (s *Service) guardianApproved(app core.App, request *core.Record, version string) bool {
	user, err := app.FindRecordById("users", request.GetString("user"))
	if err != nil {
		return false
	}
	consent, err := s.activeAcceptance(app, user)
	if err != nil || consent.GetString("version") != version {
		return false
	}
	approval, err := app.FindFirstRecordByFilter("guardian_approvals", "request={:request} && consent={:consent}", dbx.Params{"request": request.Id, "consent": consent.Id})
	if err != nil {
		return false
	}
	var evidence ConsentEvidence
	if s.Config.Open("guardian:"+approval.Id, approval.GetString("evidenceCipher"), &evidence) != nil {
		return false
	}
	doc, err := app.FindRecordById("consent_texts", version)
	return err == nil && evidence.Method == "guardianWebApproval" && evidence.Document == documentDTO(doc) && evidence.ParticipantID == user.Id && evidence.ParticipantConsentID == consent.Id && evidence.GuardianRequestID == request.Id && evidence.PersonalNumber == request.GetString("personalNumber") && adult(evidence.PersonalNumber, time.Unix(evidence.AcceptedAt, 0)) && evidence.AuthenticationID == approval.GetString("authentication") && evidence.AcceptedAt == int64(approval.GetInt("acceptedAt"))
}

func guardianVisibleText(user, doc *core.Record) string {
	return fmt.Sprintf("I approve participation for the person with personal number %s in %s (consent version %s).\n\n%s", user.GetString("personalNumber"), doc.GetString("title"), doc.GetString("version"), doc.GetString("text"))
}

func (s *Service) guardianEligible(app core.App, user *core.Record) bool {
	if atLeastAge(user.GetString("personalNumber"), s.now(), 15) {
		return true
	}
	count := user.GetInt("guardianCount")
	if count < 1 || count > 2 {
		return false
	}
	doc, err := currentDocument(app)
	if err != nil {
		return false
	}
	requests, err := s.guardianRequests(app, user.Id)
	if err != nil || len(requests) != count {
		return false
	}
	for _, request := range requests {
		if !s.guardianApproved(app, request, doc.Id) {
			return false
		}
	}
	return true
}

func (s *Service) guardianStatus(user *core.Record) (map[string]any, error) {
	user, err := s.App.FindRecordById("users", user.Id)
	if err != nil {
		return nil, err
	}
	requests, err := s.guardianRequests(s.App, user.Id)
	if err != nil {
		return nil, err
	}
	doc, err := currentDocument(s.App)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(requests))
	for _, request := range requests {
		items = append(items, map[string]any{"id": request.Id, "personalNumber": request.GetString("personalNumber"), "approved": s.guardianApproved(s.App, request, doc.Id), "link": s.Config.PublicURL + "/guardian/" + request.Id})
	}
	return map[string]any{"eligible": s.activeConsent(s.App, user) && s.guardianEligible(s.App, user), "adult": atLeastAge(user.GetString("personalNumber"), s.now(), 15), "guardianCount": user.GetInt("guardianCount"), "requests": items}, nil
}

func (s *Service) createGuardianRequest(user *core.Record, personalNumber string, count int) (map[string]any, error) {
	if !personalNumberPattern.MatchString(personalNumber) || !adult(personalNumber, s.now()) || personalNumber == user.GetString("personalNumber") || count < 1 || count > 2 {
		return nil, problem(400, "invalidGuardian", "Enter an adult guardian's 12-digit personal number and choose one or two guardians.")
	}
	if atLeastAge(user.GetString("personalNumber"), s.now(), 15) {
		return nil, problem(409, "alreadyEligible", "Guardian permission is not needed.")
	}
	var created *core.Record
	err := s.App.RunInTransaction(func(tx core.App) error {
		fresh, err := tx.FindRecordById("users", user.Id)
		if err != nil {
			return err
		}
		if !s.activeConsent(tx, fresh) {
			return problem(403, "consentRequired", "Accept the current study consent first.")
		}
		requests, err := s.guardianRequests(tx, fresh.Id)
		if err != nil {
			return err
		}
		configured := fresh.GetInt("guardianCount")
		if configured != 0 && configured != count {
			return problem(409, "guardianCountChanged", "The guardian count is already set for this participant.")
		}
		if len(requests) >= count {
			return problem(409, "guardianLimit", "All guardian links have already been created.")
		}
		for _, request := range requests {
			if request.GetString("personalNumber") == personalNumber {
				return problem(409, "duplicateGuardian", "This guardian already has a link.")
			}
		}
		if configured == 0 {
			fresh.Set("guardianCount", count)
			if err := tx.Save(fresh); err != nil {
				return err
			}
		}
		created, err = newRecord(tx, "guardian_requests")
		if err != nil {
			return err
		}
		id, err := generateGuardianRequestID(tx)
		if err != nil {
			return err
		}
		created.Set("id", id)
		created.Set("user", fresh.Id)
		created.Set("slot", len(requests)+1)
		created.Set("personalNumber", personalNumber)
		return tx.Save(created)
	})
	if err != nil {
		return nil, err
	}
	return s.guardianStatus(user)
}
