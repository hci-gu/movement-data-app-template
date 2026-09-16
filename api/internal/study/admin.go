package study

import (
	"math"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

// RegisterAdmin supports the native PocketBase collection forms. Only these
// create operations precede the generic protection of managed study records.
func (s *Service) RegisterAdmin() {
	s.App.OnRecordCreateRequest("consent_versions", "study_invitations").Bind(&hook.Handler[*core.RecordRequestEvent]{
		Id: "study_admin_create", Priority: -10,
		Func: func(e *core.RecordRequestEvent) error {
			if !e.HasSuperuserAuth() {
				return e.ForbiddenError("Only superusers can administer enrollment.", nil)
			}
			e.Response.Header().Set("Cache-Control", "no-store")
			var record *core.Record
			var err error
			switch e.Collection.Name {
			case "consent_versions":
				record, err = PublishConsent(e.App, s.Config, e.Record.GetString("version"), e.Record.GetString("title"), e.Record.GetString("text"))
			case "study_invitations":
				hours := e.Record.GetFloat("validityHours")
				if hours == 0 {
					hours = 168
				}
				if math.IsNaN(hours) || math.IsInf(hours, 0) || hours != math.Trunc(hours) || hours < 1 || hours > 2160 {
					return problem(400, "invalidValidity", "Validity must be a whole number of hours from 1 to 2160; leave blank for 168 hours.")
				}
				record, _, err = IssueInvitation(e.App, s.Config, e.Record.GetString("participantId"), e.Record.GetString("expectedPersonalNumber"), int(hours), s.now())
			}
			if err != nil {
				return err
			}
			e.Record = record
			if err := apis.EnrichRecord(e.RequestEvent, record); err != nil {
				return err
			}
			return e.JSON(200, record)
		},
	})
	s.App.OnRecordEnrich("study_invitations").BindFunc(func(e *core.RecordEnrichEvent) error {
		// These schema fields are only form inputs or response values, never
		// plaintext database storage. Work on the response record only.
		e.Record.Set("expectedPersonalNumber", "")
		e.Record.Set("invitationCode", "")
		if e.RequestInfo != nil && e.RequestInfo.HasSuperuserAuth() &&
			e.Record.GetString("study") == s.Config.StudyID && !e.Record.GetBool("consumed") &&
			e.Record.GetInt("expiresAt") > int(s.now().Unix()) && e.Record.GetString("tokenCipher") != "" {
			var code string
			if err := s.Config.open("invitation-code:"+e.Record.Id, e.Record.GetString("tokenCipher"), &code); err != nil {
				return problem(500, "invitationCodeUnavailable", "Unable to read the invitation code. Check the server encryption key configuration.")
			}
			e.Record.Set("invitationCode", code)
		}
		return e.Next()
	})
	s.App.OnRecordViewRequest("study_invitations").BindFunc(func(e *core.RecordRequestEvent) error {
		e.Response.Header().Set("Cache-Control", "no-store")
		return e.Next()
	})
	s.App.OnRecordsListRequest("study_invitations").BindFunc(func(e *core.RecordsListRequestEvent) error {
		e.Response.Header().Set("Cache-Control", "no-store")
		return e.Next()
	})
}
