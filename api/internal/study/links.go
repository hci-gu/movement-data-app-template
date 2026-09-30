package study

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

func (s *Service) registerLinks(r *router.Router[*core.RequestEvent]) {
	r.GET("/.well-known/apple-app-site-association", func(e *core.RequestEvent) error {
		if s.Config.IOSAppID == "" {
			return e.NoContent(404)
		}
		e.Response.Header().Set("Cache-Control", "public, max-age=3600")
		return e.JSON(200, map[string]any{"applinks": map[string]any{"apps": []string{}, "details": []any{
			map[string]any{"appID": s.Config.IOSAppID, "paths": []string{"/bankid/return"}},
		}}})
	})
	r.GET("/bankid/return", func(e *core.RequestEvent) error {
		e.Response.Header().Set("Cache-Control", "no-store")
		e.Response.Header().Set("Referrer-Policy", "no-referrer")
		e.Response.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		return e.HTML(200, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Return to the study</title><h1>Return to the study app</h1><p>Open the study app to check your BankID request. Your result will appear there once it has been confirmed.</p></html>`)
	})
}
