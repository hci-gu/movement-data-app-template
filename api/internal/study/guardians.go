package study

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"time"

	"app/internal/bankid"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

var signingRequestIDPattern = regexp.MustCompile(`^[0-9]{3}-[0-9]{3}$`)

func generateSigningRequestID(app core.App) (string, error) {
	for range 100 {
		n, err := rand.Int(rand.Reader, big.NewInt(1000000))
		if err != nil {
			return "", err
		}
		id := fmt.Sprintf("%03d-%03d", n.Int64()/1000, n.Int64()%1000)
		if _, err := app.FindRecordById("signingRequests", id); errors.Is(err, sql.ErrNoRows) {
			return id, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", problem(503, "busy", "En signeringslänk kunde inte skapas. Försök igen.")
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
	return app.FindRecordsByFilter("signingRequests", "user={:user}", "slot", 3, 0, dbx.Params{"user": userID})
}

func (s *Service) guardianSigned(app core.App, request *core.Record, version string) bool {
	id := request.GetString("signature")
	if id == "" {
		return false
	}
	sig, err := app.FindRecordById("signatures", id)
	if err != nil || sig.GetString("user") != request.GetString("user") || sig.GetString("purpose") != "guardian" || sig.GetString("outcome") != "accepted" || sig.GetString("version") != version || sig.GetString("evidenceCipher") == "" {
		return false
	}
	var evidence Evidence
	if s.Config.Open("signature:"+sig.Id, sig.GetString("evidenceCipher"), &evidence) != nil {
		return false
	}
	doc, err := app.FindRecordById("consent_texts", version)
	if err != nil || evidence.Document == nil || evidence.Document.ID != doc.Id || evidence.Document.DocumentHash != doc.GetString("documentHash") || evidence.Document.Text != doc.GetString("text") {
		return false
	}
	user, err := app.FindRecordById("users", request.GetString("user"))
	if err != nil || evidence.Request.UserVisibleData != base64.StdEncoding.EncodeToString([]byte(guardianVisibleText(user, doc))) {
		return false
	}
	var result bankid.Result
	return json.Unmarshal(evidence.Completion, &result) == nil && result.CompletionData != nil && result.CompletionData.User.PersonalNumber == request.GetString("personalNumber") && adult(result.CompletionData.User.PersonalNumber, time.Unix(int64(sig.GetInt("receivedAt")), 0)) && result.CompletionData.Risk == "low" && validBase64(result.CompletionData.Signature) && validBase64(result.CompletionData.OCSPResponse)
}

func guardianVisibleText(user, doc *core.Record) string {
	return fmt.Sprintf("Jag samtycker som vårdnadshavare till att barnet med personnummer %s deltar i %s (samtyckesversion %s).\n\n%s", user.GetString("personalNumber"), doc.GetString("title"), doc.GetString("version"), doc.GetString("text"))
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
		if !s.guardianSigned(app, request, doc.Id) {
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
		items = append(items, map[string]any{"id": request.Id, "personalNumber": request.GetString("personalNumber"), "signed": s.guardianSigned(s.App, request, doc.Id), "link": s.Config.PublicURL + "/guardian/" + request.Id})
	}
	return map[string]any{"eligible": s.activeConsent(s.App, user) && s.guardianEligible(s.App, user), "adult": adult(user.GetString("personalNumber"), s.now()), "guardianCount": user.GetInt("guardianCount"), "requests": items}, nil
}

func (s *Service) createGuardianRequest(user *core.Record, personalNumber string, count int) (map[string]any, error) {
	if !personalNumberPattern.MatchString(personalNumber) || !adult(personalNumber, s.now()) || personalNumber == user.GetString("personalNumber") || count < 1 || count > 2 {
		return nil, problem(400, "invalidGuardian", "Ange en vuxen vårdnadshavares personnummer med 12 siffror och välj antal vårdnadshavare.")
	}
	if atLeastAge(user.GetString("personalNumber"), s.now(), 15) {
		return nil, problem(409, "alreadyEligible", "Vårdnadshavares samtycke behövs inte.")
	}
	var created *core.Record
	err := s.App.RunInTransaction(func(tx core.App) error {
		fresh, err := tx.FindRecordById("users", user.Id)
		if err != nil {
			return err
		}
		if !s.activeConsent(tx, fresh) {
			return problem(403, "consentRequired", "Signera det aktuella samtycket först.")
		}
		requests, err := s.guardianRequests(tx, fresh.Id)
		if err != nil {
			return err
		}
		configured := fresh.GetInt("guardianCount")
		if configured != 0 && configured != count {
			return problem(409, "guardianCountChanged", "Antalet vårdnadshavare är redan sparat för deltagaren.")
		}
		if len(requests) >= count {
			return problem(409, "guardianLimit", "Alla länkar till vårdnadshavare är redan skapade.")
		}
		for _, request := range requests {
			if request.GetString("personalNumber") == personalNumber {
				return problem(409, "duplicateGuardian", "Den här vårdnadshavaren har redan en länk.")
			}
		}
		if configured == 0 {
			fresh.Set("guardianCount", count)
			if err := tx.Save(fresh); err != nil {
				return err
			}
		}
		created, err = newRecord(tx, "signingRequests")
		if err != nil {
			return err
		}
		id, err := generateSigningRequestID(tx)
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
