package study

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
)

func (s *Service) userDTO(user *core.Record) map[string]any {
	return map[string]any{"id": user.Id, "collectionId": user.Collection().Id, "collectionName": "users", "personalNumber": user.GetString("personalNumber"), "consentRequired": !s.activeConsent(s.App, user), "guardianRequired": !s.guardianEligible(s.App, user)}
}
func (s *Service) validateSession(e *core.RequestEvent) error {
	token := bearer(e)
	user, err := s.App.FindAuthRecordByToken(token, core.TokenTypeAuth)
	if err != nil || user.Collection().Name != "users" {
		return problem(401, "sessionExpired", "Sign in again with BankID.")
	}
	// The token was verified above; inspect the application scope claims.
	claims, err := security.ParseUnverifiedJWT(token)
	if err != nil || claims[core.TokenClaimRefreshable] != false {
		return problem(401, "sessionExpired", "Sign in again with BankID.")
	}
	e.Auth = user
	return nil
}
func (s *Service) RequireSession(e *core.RequestEvent) error {
	if err := s.validateSession(e); err != nil {
		return err
	}
	return e.Next()
}
func (s *Service) RequireConsent(e *core.RequestEvent) error {
	if err := s.authorizeParticipant(e); err != nil {
		return err
	}
	return e.Next()
}

// authorizeParticipant is shared by upload and questionnaire access.
func (s *Service) authorizeParticipant(e *core.RequestEvent) error {
	if e.Auth == nil || !s.activeConsent(s.App, e.Auth) {
		return problem(403, "consentRequired", "Sign the current study consent before continuing.")
	}
	if !s.guardianEligible(s.App, e.Auth) {
		return problem(403, "guardianRequired", "Guardian signatures are required before continuing.")
	}
	return nil
}
func ProtectRecords(app core.App) {
	managed := []string{"consent_texts", "signatures", "signingRequests", "users"}
	app.OnRecordCreateRequest(managed...).BindFunc(func(e *core.RecordRequestEvent) error {
		return problem(403, "managedRecord", "Use the study administration or BankID flow.")
	})
	app.OnRecordUpdateRequest(managed...).BindFunc(func(e *core.RecordRequestEvent) error {
		return problem(403, "managedRecord", "Use the study administration or BankID flow.")
	})
	app.OnRecordDeleteRequest(managed...).BindFunc(func(e *core.RecordRequestEvent) error {
		return problem(403, "managedRecord", "Study evidence and participants cannot be deleted here.")
	})
	app.OnRecordAuthRequest("users").BindFunc(func(e *core.RecordAuthRequestEvent) error {
		return problem(403, "bankidRequired", "Use BankID to sign in.")
	})
}

// Questionnaire definitions are shared; answers remain owned by their participant.
// Rules handle record selection, while these hooks enforce BankID scope and current consent.
func (s *Service) registerQuestionnaireAccess() {
	names := []string{"questionnaires", "questions", "questionOptions", "answers"}
	guard := func(e *core.RequestEvent) error {
		if err := s.validateSession(e); err != nil {
			return err
		}
		return s.authorizeParticipant(e)
	}
	s.App.OnRecordsListRequest(names...).BindFunc(func(e *core.RecordsListRequestEvent) error {
		if !e.HasSuperuserAuth() {
			if err := guard(e.RequestEvent); err != nil {
				return err
			}
		}
		return e.Next()
	})
	s.App.OnRecordViewRequest(names...).BindFunc(func(e *core.RecordRequestEvent) error {
		if !e.HasSuperuserAuth() {
			if err := guard(e.RequestEvent); err != nil {
				return err
			}
		}
		return e.Next()
	})
	write := func(e *core.RecordRequestEvent) error {
		if !e.HasSuperuserAuth() {
			if err := guard(e.RequestEvent); err != nil {
				return err
			}
			if e.Record.GetString("user") != e.Auth.Id {
				return problem(403, "wrongParticipant", "Answers must belong to the authenticated participant.")
			}
		}
		return e.Next()
	}
	s.App.OnRecordCreateRequest("answers").BindFunc(write)
	s.App.OnRecordUpdateRequest("answers").BindFunc(write)
	s.App.OnRecordDeleteRequest("answers").BindFunc(write)
}
