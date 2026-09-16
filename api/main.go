package main

import (
	"app/internal/study"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	_ "app/migrations"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/router"
)

var storagePath = "/pb/pb_data/raw"

const uploadStateTimeout = 30 * time.Minute
const uploadsDirectoryName = "_uploads"
const uploadStateFileName = ".upload_state.json"

var uploadLocks sync.Map
var participantIDRegex = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type Value struct {
	NumericValue string `json:"numericValue"`
}

type DataItem struct {
	Value        Value  `json:"value"`
	DataType     string `json:"data_type"`
	Unit         string `json:"unit"`
	DateFrom     string `json:"date_from"`
	DateTo       string `json:"date_to"`
	PlatformType string `json:"platform_type"`
	DeviceID     string `json:"device_id"`
	SourceID     string `json:"source_id"`
	SourceName   string `json:"source_name"`
}

type UploadState struct {
	SessionID     string    `json:"sessionId"`
	ExpectedChunk int       `json:"expectedChunk"`
	StartedAt     time.Time `json:"startedAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func main() {
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
		if err := svc.Recover(); err != nil {
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

func registerDataRoutes(app core.App, r *router.Router[*core.RequestEvent], requireSession, requireConsent func(*core.RequestEvent) error) {
	r.GET("/test", func(c *core.RequestEvent) error {
		return c.String(http.StatusOK, "Research steps template API is running")
	})

	r.POST("/info", func(c *core.RequestEvent) error {
		reqBody := struct {
			ParticipantID string                 `json:"participantId"`
			Data          map[string]interface{} `json:"data"`
		}{}
		if err := c.BindBody(&reqBody); err != nil {
			return httpError(http.StatusBadRequest, "Invalid request body")
		}

		participantID, err := authorizedParticipant(c, reqBody.ParticipantID)
		if err != nil {
			return err
		}

		user, err := getUserForParticipantID(app, participantID)
		if err != nil {
			return httpError(http.StatusNotFound, "Participant not found")
		}

		collection, err := app.FindCollectionByNameOrId("info")
		if err != nil {
			return httpError(http.StatusInternalServerError, "Failed to find info collection")
		}

		record := core.NewRecord(collection)
		record.Set("user", user.Id)
		record.Set("data", reqBody.Data)
		if err := app.Save(record); err != nil {
			return httpError(http.StatusInternalServerError, "Failed to save participant info")
		}

		return c.NoContent(http.StatusCreated)
	}).BindFunc(requireSession, requireConsent)

	r.POST("/data", func(c *core.RequestEvent) error {
		reqBody := struct {
			ParticipantID string     `json:"participantId"`
			ChunkIndex    int        `json:"chunkIndex"`
			Data          []DataItem `json:"data"`
		}{}
		if err := c.BindBody(&reqBody); err != nil {
			return httpError(http.StatusBadRequest, "Invalid request body")
		}

		participantID, err := authorizedParticipant(c, reqBody.ParticipantID)
		if err != nil {
			return err
		}

		if reqBody.ChunkIndex < 0 {
			return httpError(http.StatusBadRequest, "Invalid chunkIndex")
		}
		if len(reqBody.Data) == 0 {
			return httpError(http.StatusBadRequest, "No data points provided")
		}

		dataHash, err := hashDataItems(reqBody.Data)
		if err != nil {
			log.Println("Error hashing upload:", err)
			return httpError(http.StatusInternalServerError, "Failed to process upload")
		}

		now := time.Now().UTC()
		lock := getUploadLock(participantID)
		lock.Lock()
		defer lock.Unlock()

		state, err := loadUploadState(participantID)
		if err != nil {
			return httpError(http.StatusInternalServerError, "Failed to read upload state")
		}

		switch {
		case reqBody.ChunkIndex == 0:
			if state == nil || isUploadStateExpired(state, now) {
				state, err = createNewUploadState(participantID, now)
				if err != nil {
					return httpError(http.StatusInternalServerError, "Failed to start upload")
				}
			} else if state.ExpectedChunk > 0 {
				duplicateFirstChunk, err := isDuplicateChunk(participantID, state, 0, dataHash)
				if err != nil {
					return httpError(http.StatusInternalServerError, "Failed to process upload")
				}
				if !duplicateFirstChunk {
					state, err = createNewUploadState(participantID, now)
					if err != nil {
						return httpError(http.StatusInternalServerError, "Failed to restart upload")
					}
				}
			}
		case state == nil || isUploadStateExpired(state, now):
			return httpError(http.StatusConflict, "No active upload session. Restart from chunk 0.")
		}

		if reqBody.ChunkIndex > state.ExpectedChunk {
			return httpError(
				http.StatusConflict,
				fmt.Sprintf("Out-of-order chunk. Expected chunk %d.", state.ExpectedChunk),
			)
		}

		if reqBody.ChunkIndex < state.ExpectedChunk {
			isDuplicate, err := isDuplicateChunk(participantID, state, reqBody.ChunkIndex, dataHash)
			if err != nil {
				return httpError(http.StatusInternalServerError, "Failed to process upload")
			}
			if !isDuplicate {
				return httpError(http.StatusConflict, "Chunk already received with different content.")
			}

			filePath := chunkFilePath(participantID, state.SessionID, reqBody.ChunkIndex)
			return c.JSON(http.StatusOK, map[string]any{
				"message":    "Chunk already received",
				"filePath":   filePath,
				"sessionId":  state.SessionID,
				"chunkIndex": reqBody.ChunkIndex,
			})
		}

		filePath := chunkFilePath(participantID, state.SessionID, reqBody.ChunkIndex)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
			return httpError(http.StatusInternalServerError, "Failed to prepare storage directory")
		}

		if err := writeCompressedFile(filePath, reqBody.Data); err != nil {
			return httpError(http.StatusInternalServerError, "Failed to persist upload chunk")
		}
		if err := saveChunkHash(participantID, state, reqBody.ChunkIndex, dataHash); err != nil {
			return httpError(http.StatusInternalServerError, "Failed to persist chunk metadata")
		}

		state.ExpectedChunk++
		state.UpdatedAt = now
		if err := saveUploadState(participantID, state); err != nil {
			return httpError(http.StatusInternalServerError, "Failed to persist upload state")
		}

		dataFrom, dataTo, err := getEarliestAndLatestDates(reqBody.Data)
		if err != nil {
			return httpError(http.StatusInternalServerError, "Failed to read upload date coverage")
		}

		collection, err := app.FindCollectionByNameOrId("dataUploads")
		if err != nil {
			return httpError(http.StatusInternalServerError, "Failed to find uploads collection")
		}

		user, err := getUserForParticipantID(app, participantID)
		if err != nil {
			return httpError(http.StatusNotFound, "Participant not found")
		}

		record := core.NewRecord(collection)
		record.Set("user", user.Id)
		record.Set("filePath", filePath)
		record.Set("timestamp", now)
		record.Set("dataFrom", dataFrom)
		record.Set("dataTo", dataTo)
		if err := app.Save(record); err != nil {
			return httpError(http.StatusInternalServerError, "Failed to save upload metadata")
		}

		return c.JSON(http.StatusOK, map[string]any{
			"message":    "Data saved successfully",
			"filePath":   filePath,
			"sessionId":  state.SessionID,
			"chunkIndex": reqBody.ChunkIndex,
		})
	}).Bind(apis.BodyLimit(200*1024*1024)).BindFunc(requireSession, requireConsent, decompressUpload)

	r.GET("/data/{participantId}", requireAPIKey(func(c *core.RequestEvent) error {
		participantID, err := sanitizeParticipantID(c.Request.PathValue("participantId"))
		if err != nil {
			return httpError(http.StatusBadRequest, "Invalid participantId")
		}

		lock := getUploadLock(participantID)
		lock.Lock()
		defer lock.Unlock()

		sessionDirs, err := listSessionDirectories(participantID)
		if err != nil {
			return httpError(http.StatusInternalServerError, "Failed to list upload sessions")
		}

		for index := len(sessionDirs) - 1; index >= 0; index-- {
			allData, err := readDataFromSession(participantID, sessionDirs[index])
			if err != nil {
				return httpError(http.StatusInternalServerError, "Failed to read upload session")
			}
			if len(allData) > 0 {
				return c.JSON(http.StatusOK, allData)
			}
		}

		allData, err := readLegacyData(participantFolderPath(participantID))
		if err != nil {
			return httpError(http.StatusInternalServerError, "Failed to read upload data")
		}
		return c.JSON(http.StatusOK, allData)
	}))

}

func requireAPIKey(next func(*core.RequestEvent) error) func(*core.RequestEvent) error {
	return func(c *core.RequestEvent) error {
		apiKey := os.Getenv("API_KEY")
		if apiKey == "" {
			log.Println("WARNING: API_KEY environment variable is not set")
			return httpError(http.StatusInternalServerError, "Server misconfigured")
		}

		providedKey := c.Request.Header.Get("X-API-Key")
		if providedKey == "" || subtle.ConstantTimeCompare([]byte(providedKey), []byte(apiKey)) != 1 {
			return httpError(http.StatusUnauthorized, "Invalid or missing API key")
		}

		return next(c)
	}
}

func sanitizeParticipantID(value string) (string, error) {
	if value == "" || !participantIDRegex.MatchString(value) {
		return "", fmt.Errorf("invalid participantId: %q", value)
	}
	return value, nil
}

func getUploadLock(participantID string) *sync.Mutex {
	lock, _ := uploadLocks.LoadOrStore(participantID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func participantFolderPath(participantID string) string {
	return filepath.Join(storagePath, participantID)
}

func uploadsRootPath(participantID string) string {
	return filepath.Join(participantFolderPath(participantID), uploadsDirectoryName)
}

func uploadSessionPath(participantID, sessionID string) string {
	return filepath.Join(uploadsRootPath(participantID), sessionID)
}

func uploadStatePath(participantID string) string {
	return filepath.Join(participantFolderPath(participantID), uploadStateFileName)
}

func chunkFilePath(participantID, sessionID string, chunkIndex int) string {
	return filepath.Join(
		uploadSessionPath(participantID, sessionID),
		fmt.Sprintf("chunk-%05d.json.gz", chunkIndex),
	)
}

func chunkHashPath(participantID, sessionID string, chunkIndex int) string {
	return filepath.Join(
		uploadSessionPath(participantID, sessionID),
		fmt.Sprintf("chunk-%05d.sha256", chunkIndex),
	)
}

func newSessionID(now time.Time) (string, error) {
	token := make([]byte, 6)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"%s-%s",
		now.UTC().Format("20060102T150405.000000000Z"),
		hex.EncodeToString(token),
	), nil
}

func hashDataItems(data []DataItem) (string, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func loadUploadState(participantID string) (*UploadState, error) {
	raw, err := os.ReadFile(uploadStatePath(participantID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var state UploadState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	if state.SessionID == "" {
		return nil, nil
	}

	return &state, nil
}

func saveUploadState(participantID string, state *UploadState) error {
	stateDir := participantFolderPath(participantID)
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}

	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(uploadStatePath(participantID), raw, 0o600)
}

func isUploadStateExpired(state *UploadState, now time.Time) bool {
	if state == nil {
		return true
	}
	return now.Sub(state.UpdatedAt) > uploadStateTimeout
}

func createNewUploadState(participantID string, now time.Time) (*UploadState, error) {
	sessionID, err := newSessionID(now)
	if err != nil {
		return nil, err
	}

	sessionDir := uploadSessionPath(participantID, sessionID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return nil, err
	}

	state := &UploadState{
		SessionID:     sessionID,
		ExpectedChunk: 0,
		StartedAt:     now,
		UpdatedAt:     now,
	}
	if err := saveUploadState(participantID, state); err != nil {
		return nil, err
	}

	return state, nil
}

func isDuplicateChunk(participantID string, state *UploadState, chunkIndex int, incomingHash string) (bool, error) {
	existingHashBytes, err := os.ReadFile(chunkHashPath(participantID, state.SessionID, chunkIndex))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	existingHash := strings.TrimSpace(string(existingHashBytes))
	return existingHash == incomingHash, nil
}

func saveChunkHash(participantID string, state *UploadState, chunkIndex int, hash string) error {
	return os.WriteFile(
		chunkHashPath(participantID, state.SessionID, chunkIndex),
		[]byte(hash),
		0o600,
	)
}

func listSessionDirectories(participantID string) ([]string, error) {
	entries, err := os.ReadDir(uploadsRootPath(participantID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	directories := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, entry.Name())
		}
	}
	sort.Strings(directories)

	return directories, nil
}

func readDataFromSession(participantID, sessionID string) ([]DataItem, error) {
	sessionDir := uploadSessionPath(participantID, sessionID)
	files, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, err
	}

	chunkFiles := make([]string, 0, len(files))
	for _, file := range files {
		name := file.Name()
		if !file.IsDir() && strings.HasPrefix(name, "chunk-") && strings.HasSuffix(name, ".json.gz") {
			chunkFiles = append(chunkFiles, name)
		}
	}
	sort.Strings(chunkFiles)

	var allData []DataItem
	for _, chunkFile := range chunkFiles {
		filePath := filepath.Join(sessionDir, chunkFile)
		dataItems, err := readCompressedFile(filePath)
		if err != nil {
			return nil, err
		}
		allData = append(allData, dataItems...)
	}

	return allData, nil
}

func readLegacyData(folderPath string) ([]DataItem, error) {
	files, err := os.ReadDir(folderPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	gzipFiles := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".gz" {
			gzipFiles = append(gzipFiles, file.Name())
		}
	}
	sort.Strings(gzipFiles)

	var allData []DataItem
	for _, gzipFile := range gzipFiles {
		filePath := filepath.Join(folderPath, gzipFile)
		dataItems, err := readCompressedFile(filePath)
		if err != nil {
			return nil, err
		}
		allData = append(allData, dataItems...)
	}

	return allData, nil
}

func getEarliestAndLatestDates(data []DataItem) (string, string, error) {
	if len(data) == 0 {
		return "", "", fmt.Errorf("no data points provided")
	}

	earliest := data[0].DateFrom
	latest := data[0].DateTo

	for _, item := range data {
		if strings.Compare(item.DateFrom, earliest) < 0 {
			earliest = item.DateFrom
		}
		if strings.Compare(item.DateTo, latest) > 0 {
			latest = item.DateTo
		}
	}

	return earliest, latest, nil
}

func writeCompressedFile(filePath string, data []DataItem) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	if _, err = gzipWriter.Write(jsonData); err != nil {
		_ = gzipWriter.Close()
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	return file.Sync()
}

func readCompressedFile(filePath string) ([]DataItem, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	decompressedData, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, err
	}

	var dataItems []DataItem
	if err := json.Unmarshal(decompressedData, &dataItems); err != nil {
		return nil, err
	}

	return dataItems, nil
}

func getUserForParticipantID(app core.App, participantID string) (*core.Record, error) {
	return app.FindFirstRecordByData("users", "username", participantID)
}

func httpError(status int, message string) error { return apis.NewApiError(status, message, nil) }

// The body identifier is optional for older clients, but can never override identity.
func authorizedParticipant(e *core.RequestEvent, supplied string) (string, error) {
	if e.Auth == nil {
		return "", httpError(401, "Participant authentication required")
	}
	id, err := sanitizeParticipantID(e.Auth.GetString("username"))
	if err != nil || (supplied != "" && supplied != id) {
		return "", httpError(403, "Participant does not match authenticated session")
	}
	return id, nil
}

func decompressUpload(e *core.RequestEvent) error {
	const maxSize = 200 * 1024 * 1024
	e.Request.Body = http.MaxBytesReader(e.Response, e.Request.Body, maxSize)
	encoding := e.Request.Header.Get("Content-Encoding")
	if encoding != "" && encoding != "gzip" {
		return httpError(415, "Unsupported content encoding")
	}
	if encoding == "gzip" {
		reader, err := gzip.NewReader(e.Request.Body)
		if err != nil {
			return httpError(400, "Invalid gzip body")
		}
		defer reader.Close()
		e.Request.Body = http.MaxBytesReader(e.Response, reader, maxSize)
		e.Request.Header.Del("Content-Encoding")
		e.Request.ContentLength = -1
	}
	return e.Next()
}
