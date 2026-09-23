package study

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

func (s *Service) RegisterAdmin() {
	s.App.OnRecordCreateRequest("consent_texts").Bind(&hook.Handler[*core.RecordRequestEvent]{
		Id: "consent_admin_create", Priority: -10,
		Func: func(e *core.RecordRequestEvent) error {
			if !e.HasSuperuserAuth() {
				return e.ForbiddenError("Only superusers can publish consent.", nil)
			}
			record, err := PublishConsent(e.App, e.Record.GetString("version"), e.Record.GetString("title"), e.Record.GetString("text"))
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
}
