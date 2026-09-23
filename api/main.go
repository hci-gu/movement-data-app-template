package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"app/internal/study"
	_ "app/migrations"

	"github.com/joho/godotenv"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
	}
	app := pocketbase.New()
	isGoRun := strings.HasPrefix(os.Args[0], os.TempDir())

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: isGoRun,
	})

	study.RegisterCommands(app)
	study.ProtectRecords(app)
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		storagePath = filepath.Join(app.DataDir(), "raw")
		cfg, err := study.LoadConfig()
		if err != nil {
			return err
		}
		svc, err := study.Open(app, cfg)
		if err != nil {
			return err
		}
		svc.RegisterAdmin()
		svc.RegisterRoutes(e.Router)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { defer close(done); svc.Run(ctx) }()
		app.OnTerminate().BindFunc(func(event *core.TerminateEvent) error { cancel(); <-done; return event.Next() })

		registerDataRoutes(app, e.Router, svc.RequireSession, svc.RequireConsent)

		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
