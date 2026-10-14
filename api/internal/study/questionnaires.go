package study

import (
	"app/internal/questionnaires"
	"database/sql"
	"errors"
	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func (s *Service) registerAnswerWrites(guard func(*core.RequestEvent) error) {
	s.App.OnRecordCreateRequest("answers").BindFunc(func(e *core.RecordRequestEvent) error {
		// Researcher administration remains separate from participant submission.
		if e.HasSuperuserAuth() {
			return e.Next()
		}
		if err := guard(e.RequestEvent); err != nil {
			return err
		}
		if id := e.Record.GetString("user"); id != "" && id != e.Auth.Id {
			return problem(403, "wrongParticipant", "Answers must belong to the authenticated participant.")
		}
		e.Record.Set("user", e.Auth.Id)
		formID := e.Record.GetString("questionnaire")
		var existing *core.Record
		err := e.App.RunInTransaction(func(tx core.App) error {
			d, err := questionnaires.Load(tx, formID)
			if err != nil {
				return problem(400, "invalidQuestionnaire", "Formuläret är inte tillgängligt eller har en ogiltig konfiguration.")
			}
			answers := map[string]any{}
			if err := e.Record.UnmarshalJSONField("answers", &answers); err != nil || answers == nil {
				return problem(400, "invalidAnswers", "Svaren måste vara ett JSON-objekt.")
			}
			messages := d.Validate(answers, Gender(e.Auth.GetString("personalNumber")), s.now())
			if len(messages) > 0 {
				fields := validation.Errors{}
				for id, message := range messages {
					fields[id] = validation.NewError("invalidAnswer", message)
				}
				return apis.NewBadRequestError("Kontrollera dina svar.", validation.Errors{"answers": fields})
			}
			for id, v := range answers {
				if v == nil || v == "" {
					delete(answers, id)
				}
			}
			e.Record.Set("answers", answers)
			key := e.Auth.Id + ":" + formID + ":once"
			existing, err = tx.FindFirstRecordByData("answers", "submissionKey", key)
			if err == nil {
				return nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			e.Record.Set("submissionKey", key)
			if e.Record.GetDateTime("started").IsZero() {
				e.Record.Set("started", s.now())
			}
			// Save through PocketBase inside the same transaction as duplicate lookup.
			// Do not call e.Next(), which would save a second time outside this transaction.
			return tx.Save(e.Record)
		})
		if err != nil {
			return err
		}
		if existing != nil {
			return e.JSON(200, existing.PublicExport())
		}
		return e.JSON(200, e.Record.PublicExport())
	})
	reject := func(e *core.RecordRequestEvent) error {
		if e.HasSuperuserAuth() {
			return e.Next()
		}
		if err := guard(e.RequestEvent); err != nil {
			return err
		}
		return problem(403, "submittedReadOnly", "Inskickade svar kan inte ändras eller raderas.")
	}
	s.App.OnRecordUpdateRequest("answers").BindFunc(reject)
	s.App.OnRecordDeleteRequest("answers").BindFunc(reject)
}
