package main

import (
	"app/internal/study"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func uploadApp(t *testing.T) *pocketbase.PocketBase {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	storagePath = t.TempDir()
	return app
}

func TestUploadEndpointsRequireBankIDSessionBeforeBodyOrStorage(t *testing.T) {
	app := uploadApp(t)
	svc, err := study.Open(app, study.Config{Environment: "disabled"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	registerDataRoutes(app, r, svc.RequireSession, svc.RequireConsent)
	h, err := r.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/data"} {
		req := httptest.NewRequest("POST", path, bytes.NewBufferString("invalid gzip"))
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
	entries, err := os.ReadDir(storagePath)
	if err != nil || len(entries) != 0 {
		t.Fatal("unauthorized request touched storage", entries, err)
	}
}

// Signing/session validation is exercised by internal/study HTTP integration
// tests. These route tests supply its authenticated participant to verify the
// existing gzip/chunk protocol and prevent identity overrides at the boundary.
func TestAuthenticatedUploadIdentityGzipAndChunkOrdering(t *testing.T) {
	app := uploadApp(t)
	users, _ := app.FindCollectionByNameOrId("users")
	user := core.NewRecord(users)
	user.Set("personalNumber", "200001012384")
	user.SetPassword("fixture-password-123456")
	if err := app.Save(user); err != nil {
		t.Fatal(err)
	}
	r, _ := apis.NewRouter(app)
	registerDataRoutes(app, r, func(e *core.RequestEvent) error { e.Auth = user; return e.Next() }, func(e *core.RequestEvent) error { return e.Next() })
	h, err := r.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	data := []DataItem{{Value: Value{"10"}, DateFrom: "2026-09-01T12:00:00Z", DateTo: "2026-09-01T12:01:00Z", DataType: "STEPS"}}
	send := func(participant string, chunk int) int {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"userId": participant, "chunkIndex": chunk, "data": data})
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, _ = gz.Write(raw)
		_ = gz.Close()
		req := httptest.NewRequest("POST", "/data", &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code >= 500 {
			t.Fatal(w.Code, w.Body.String())
		}
		return w.Code
	}
	if code := send("OTHER-001", 0); code != http.StatusForbidden {
		t.Fatal("cross-participant upload", code)
	}
	if code := send(user.Id, 1); code != http.StatusConflict {
		t.Fatal("nonzero first chunk", code)
	}
	if code := send(user.Id, 0); code != 200 {
		t.Fatal("gzip chunk", code)
	}
	if code := send(user.Id, 0); code != 200 {
		t.Fatal("duplicate chunk", code)
	}
	if code := send(user.Id, 2); code != 409 {
		t.Fatal("out-of-order chunk", code)
	}
	if code := send(user.Id, 1); code != 200 {
		t.Fatal("second chunk", code)
	}
	state, err := loadUploadState(user.Id)
	if err != nil {
		t.Fatal(err)
	}
	points, err := readDataFromSession(user.Id, state.SessionID)
	if err != nil || len(points) != 2 {
		t.Fatal("stored chunks", len(points), err)
	}
	info, err := os.Stat(chunkFilePath(user.Id, state.SessionID, 0))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("private upload permissions", err)
	}
}
