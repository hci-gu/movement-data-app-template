package main

import (
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var storagePath = "/pb/pb_data/raw"

const uploadStateTimeout = 30 * time.Minute
const uploadsDirectoryName = "_uploads"
const uploadStateFileName = ".upload_state.json"

var uploadLocks sync.Map
var userIDRegex = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

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

func sanitizeUserID(value string) (string, error) {
	if value == "" || !userIDRegex.MatchString(value) {
		return "", fmt.Errorf("invalid userId: %q", value)
	}
	return value, nil
}

func getUploadLock(userID string) *sync.Mutex {
	lock, _ := uploadLocks.LoadOrStore(userID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func participantFolderPath(userID string) string {
	return filepath.Join(storagePath, userID)
}

func uploadsRootPath(userID string) string {
	return filepath.Join(participantFolderPath(userID), uploadsDirectoryName)
}

func uploadSessionPath(userID, sessionID string) string {
	return filepath.Join(uploadsRootPath(userID), sessionID)
}

func uploadStatePath(userID string) string {
	return filepath.Join(participantFolderPath(userID), uploadStateFileName)
}

func chunkFilePath(userID, sessionID string, chunkIndex int) string {
	return filepath.Join(
		uploadSessionPath(userID, sessionID),
		fmt.Sprintf("chunk-%05d.json.gz", chunkIndex),
	)
}

func chunkHashPath(userID, sessionID string, chunkIndex int) string {
	return filepath.Join(
		uploadSessionPath(userID, sessionID),
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

func loadUploadState(userID string) (*UploadState, error) {
	raw, err := os.ReadFile(uploadStatePath(userID))
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

func saveUploadState(userID string, state *UploadState) error {
	stateDir := participantFolderPath(userID)
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}

	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(uploadStatePath(userID), raw, 0o600)
}

func isUploadStateExpired(state *UploadState, now time.Time) bool {
	if state == nil {
		return true
	}
	return now.Sub(state.UpdatedAt) > uploadStateTimeout
}

func createNewUploadState(userID string, now time.Time) (*UploadState, error) {
	sessionID, err := newSessionID(now)
	if err != nil {
		return nil, err
	}

	sessionDir := uploadSessionPath(userID, sessionID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return nil, err
	}

	state := &UploadState{
		SessionID:     sessionID,
		ExpectedChunk: 0,
		StartedAt:     now,
		UpdatedAt:     now,
	}
	if err := saveUploadState(userID, state); err != nil {
		return nil, err
	}

	return state, nil
}

func isDuplicateChunk(userID string, state *UploadState, chunkIndex int, incomingHash string) (bool, error) {
	existingHashBytes, err := os.ReadFile(chunkHashPath(userID, state.SessionID, chunkIndex))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	existingHash := strings.TrimSpace(string(existingHashBytes))
	return existingHash == incomingHash, nil
}

func saveChunkHash(userID string, state *UploadState, chunkIndex int, hash string) error {
	return os.WriteFile(
		chunkHashPath(userID, state.SessionID, chunkIndex),
		[]byte(hash),
		0o600,
	)
}

func listSessionDirectories(userID string) ([]string, error) {
	entries, err := os.ReadDir(uploadsRootPath(userID))
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

func readDataFromSession(userID, sessionID string) ([]DataItem, error) {
	sessionDir := uploadSessionPath(userID, sessionID)
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
